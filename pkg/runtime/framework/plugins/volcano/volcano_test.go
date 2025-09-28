package volcano

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	batchv1ac "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	jobsetv1alpha2ac "sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"
	volcanov1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"
	"volcano.sh/apis/pkg/client/applyconfiguration/scheduling/v1beta1"
	volcanov1beta1ac "volcano.sh/apis/pkg/client/applyconfiguration/scheduling/v1beta1"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/apply"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	index "github.com/kubeflow/trainer/v2/pkg/runtime/indexer"
	runtimeindexer "github.com/kubeflow/trainer/v2/pkg/runtime/indexer"
)

func TestName(t *testing.T) {
	v := &Volcano{}
	expectedName := "Volcano"
	if v.Name() != expectedName {
		t.Errorf("expected name %s, got %s", expectedName, v.Name())
	}
}

func TestEnforcePodGroupPolicy(t *testing.T) {
	v := &Volcano{}

	jobName := "test-trainjob"
	info := &runtime.Info{
		Scheduler: &runtime.Scheduler{
			PodLabels: nil,
		},
		RuntimePolicy: runtime.RuntimePolicy{
			PodGroupPolicy: &trainer.PodGroupPolicy{
				PodGroupPolicySource: trainer.PodGroupPolicySource{
					Volcano: &trainer.VolcanoPodGroupPolicySource{},
				},
			},
		},
	}
	trainJob := &trainer.TrainJob{}
	trainJob.Name = jobName

	err := v.EnforcePodGroupPolicy(info, trainJob)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := info.Scheduler.PodAnnotations[volcanov1beta1.KubeGroupNameAnnotationKey]
	want := jobName
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected PodLabel (-want +got):\n%s", diff)
	}

	// Test with nil info
	err = v.EnforcePodGroupPolicy(nil, trainJob)
	if err != nil {
		t.Errorf("expected nil error when info is nil, got: %v", err)
	}
}

type errorClient struct {
	client.WithWatch
	errNotFound bool
}

func (e *errorClient) Get(_ context.Context, key client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
	if !e.errNotFound {
		return apierrors.NewNotFound(corev1.Resource("podgroups"), key.Name)
	}
	return fmt.Errorf("mock get error")
}

func TestBuildPodGroup(t *testing.T) {
	scheme := apiruntime.NewScheme()
	if err := trainer.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add trainer scheme: %v", err)
	}
	if err := volcanov1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add volcano scheme: %v", err)
	}

	ctx := context.Background()

	jobSetSpecApply, err := apply.FromTypedObjWithFields[jobsetv1alpha2ac.JobSetSpecApplyConfiguration](&jobsetv1alpha2.JobSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: jobsetv1alpha2.GroupVersion.String(),
			Kind:       "JobSet",
		},
		Spec: jobsetv1alpha2.JobSetSpec{
			ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
				{
					Name: "launcher",
					Template: batchv1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									PriorityClassName: "high-priority",
								},
							},
						},
					},
				},
			},
		},
	}, "spec")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	baseInfo := func() *runtime.Info {
		return &runtime.Info{
			TemplateSpec: runtime.TemplateSpec{
				ObjApply: jobSetSpecApply,
				PodSets: []runtime.PodSet{
					{
						Name:  "launcher",
						Count: ptr.To[int32](1),
						SinglePodRequests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("300m"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
					{
						Name:  "worker",
						Count: ptr.To[int32](4),
						SinglePodRequests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("0.5Gi"),
						},
					},
				},
			},
		}
	}

	cases := map[string]struct {
		trainJob   *trainer.TrainJob
		info       *runtime.Info
		existingPG client.Object
		expectPG   *v1beta1.PodGroupApplyConfiguration
		expectErr  error
	}{
		"Test nil info": {
			trainJob:   &trainer.TrainJob{},
			info:       nil,
			existingPG: nil,
			expectPG:   nil,
			expectErr:  nil,
		},
		"PodGroup exists and trainjob not suspended": {
			trainJob: &trainer.TrainJob{
				ObjectMeta: metav1.ObjectMeta{Name: "job-exist-running", Namespace: "test-ns", UID: "1"},
				Spec:       trainer.TrainJobSpec{Suspend: ptr.To(false)},
			},
			info: &runtime.Info{
				TemplateSpec: baseInfo().TemplateSpec,
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainer.PodGroupPolicy{
						PodGroupPolicySource: trainer.PodGroupPolicySource{
							Volcano: &trainer.VolcanoPodGroupPolicySource{},
						},
					},
				},
			},
			existingPG: &volcanov1beta1.PodGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "job-exist-running", Namespace: "test-ns"},
			},
			expectPG:  nil,
			expectErr: nil,
		},
		"PodGroup exists but trainjob suspended": {
			trainJob: &trainer.TrainJob{
				ObjectMeta: metav1.ObjectMeta{Name: "job-update", Namespace: "test-ns", UID: "2"},
				Spec:       trainer.TrainJobSpec{Suspend: ptr.To(true)},
			},
			info: &runtime.Info{
				TemplateSpec: baseInfo().TemplateSpec,
				Annotations: map[string]string{
					"scheduling.volcano.sh/queue-name": "q1",
				},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainer.PodGroupPolicy{
						PodGroupPolicySource: trainer.PodGroupPolicySource{
							Volcano: &trainer.VolcanoPodGroupPolicySource{
								NetworkTopology: &volcanov1beta1.NetworkTopologySpec{
									Mode:               volcanov1beta1.HardNetworkTopologyMode,
									HighestTierAllowed: ptr.To(1),
								},
							},
						},
					},
				},
			},
			existingPG: &volcanov1beta1.PodGroup{ObjectMeta: metav1.ObjectMeta{Name: "job-update", Namespace: "test-ns"}},
			expectPG: volcanov1beta1ac.PodGroup("job-update", "test-ns").
				WithOwnerReferences(metav1ac.OwnerReference().
					WithAPIVersion(trainer.GroupVersion.String()).
					WithKind(trainer.TrainJobKind).
					WithName("job-update").
					WithUID(types.UID(strconv.Itoa(2))).
					WithController(true).
					WithBlockOwnerDeletion(true)).
				WithSpec(volcanov1beta1ac.PodGroupSpec().
					WithMinMember(5).
					WithMinResources(corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2300m"),
						corev1.ResourceMemory: resource.MustParse("3Gi"),
					}).
					WithQueue("q1").
					WithPriorityClassName("high-priority").
					WithNetworkTopology(&volcanov1beta1ac.NetworkTopologySpecApplyConfiguration{
						Mode:               ptr.To(volcanov1beta1.HardNetworkTopologyMode),
						HighestTierAllowed: ptr.To(1),
					})),
			expectErr: nil,
		},
		"Error when getting existing PodGroup": {
			trainJob: &trainer.TrainJob{
				ObjectMeta: metav1.ObjectMeta{Name: "job-error", Namespace: "test-ns", UID: "3"},
				Spec:       trainer.TrainJobSpec{Suspend: ptr.To(false)},
			},
			info: &runtime.Info{
				TemplateSpec: baseInfo().TemplateSpec,
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainer.PodGroupPolicy{
						PodGroupPolicySource: trainer.PodGroupPolicySource{
							Volcano: &trainer.VolcanoPodGroupPolicySource{},
						},
					},
				},
			},
			existingPG: nil,
			expectPG:   nil,
			expectErr:  fmt.Errorf("mock get error"),
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
			// fake for get existing PodGroup
			if c.existingPG != nil {
				clientBuilder.WithObjects(c.existingPG)
			}
			client_ := clientBuilder.Build()

			// fake for error handling
			if name == "Error when getting existing PodGroup" {
				client_ = &errorClient{WithWatch: client_, errNotFound: true}
			}
			if name == "PodGroup exists but trainjob suspended" {
				client_ = &errorClient{WithWatch: client_, errNotFound: false}
			}

			v := &Volcano{
				client:     client_,
				restMapper: nil,
				scheme:     scheme,
			}
			objs, err := v.Build(ctx, c.info, c.trainJob)

			if c.expectErr != nil {
				if err == nil {
					t.Fatalf("expected error: %v, got nil", c.expectErr)
				}
				if diff := cmp.Diff(c.expectErr.Error(), err.Error()); diff != "" {
					t.Errorf("unexpected error (-want +got):\n%s", diff)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if c.expectPG == nil {
				if objs != nil {
					t.Errorf("expected nil result, got: %+v", objs)
				}
				return
			}

			if len(objs) != 1 {
				t.Fatalf("expected 1 object, got %d", len(objs))
			}
			actualPG, ok := objs[0].(*v1beta1.PodGroupApplyConfiguration)
			if !ok {
				t.Fatalf("expected PodGroupApplyConfiguration, got %T", objs[0])
			}

			if diff := cmp.Diff(c.expectPG, actualPG); diff != "" {
				t.Errorf("mismatch in PodGroup (-want +got):\n%s", diff)
			}
		})
	}
}
func TestReconcilerBuilders(t *testing.T) {
	scheme := apiruntime.NewScheme()
	if err := trainer.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add trainer scheme: %v", err)
	}
	if err := volcanov1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add volcano scheme: %v", err)
	}

	gvk := volcanov1beta1.SchemeGroupVersion.WithKind("PodGroup")
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{volcanov1beta1.SchemeGroupVersion})
	mapper.Add(gvk, meta.RESTScopeNamespace)
	v := &Volcano{
		scheme:     scheme,
		client:     fake.NewClientBuilder().WithScheme(scheme).Build(),
		restMapper: mapper,
	}

	builders := v.ReconcilerBuilders()
	if diff := cmp.Diff(3, len(builders)); diff != "" {
		t.Errorf("unexpected builder count (-want +got):\n%s", diff)
	}

	b := &builder.Builder{}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Ensure builder function does not return nil
	b2 := builders[0](b, cl, nil)
	if b2 == nil {
		t.Errorf("builder should not be nil after applying reconciler builder function")
	}
}

func runHandlerTest[T any](t *testing.T, cli client.WithWatch, handler any, obj T) {

	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	ctx := context.Background()

	assertQueue := func(want int) {
		if got := q.Len(); got != want {
			t.Fatalf("expected %d reconcile requests in queue, got %d", want, got)
		}
	}

	switch h := handler.(type) {
	case *PodGroupRuntimeClassHandler:
		if rc, ok := any(obj).(*nodev1.RuntimeClass); ok {
			h.client = cli
			h.Create(ctx, event.TypedCreateEvent[*nodev1.RuntimeClass]{Object: rc}, q)
			assertQueue(1)
			h.Update(ctx, event.TypedUpdateEvent[*nodev1.RuntimeClass]{ObjectNew: rc}, q)
			assertQueue(1)
			h.Delete(ctx, event.TypedDeleteEvent[*nodev1.RuntimeClass]{Object: rc}, q)
			assertQueue(1)
		}
	case *PodGroupLimitRangeHandler:
		if lr, ok := any(obj).(*corev1.LimitRange); ok {
			h.client = cli
			h.Create(ctx, event.TypedCreateEvent[*corev1.LimitRange]{Object: lr}, q)
			assertQueue(1)
			h.Update(ctx, event.TypedUpdateEvent[*corev1.LimitRange]{ObjectNew: lr}, q)
			assertQueue(1)
			h.Delete(ctx, event.TypedDeleteEvent[*corev1.LimitRange]{Object: lr}, q)
			assertQueue(1)
		}
	}

	if q.Len() == 0 {
		t.Fatalf("expected at least 1 reconcile request in queue, got 0")
	}
}

func TestPodGroupRuntimeClassHandler_AllEvents(t *testing.T) {
	scheme := apiruntime.NewScheme()
	_ = trainer.AddToScheme(scheme)
	_ = nodev1.AddToScheme(scheme)

	trainJob := &trainer.TrainJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
		},
		Spec: trainer.TrainJobSpec{
			Suspend: ptr.To(true),
		},
	}
	tr := &trainer.TrainingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-runtime",
		},
		Spec: trainer.TrainingRuntimeSpec{
			PodGroupPolicy: &trainer.PodGroupPolicy{
				PodGroupPolicySource: trainer.PodGroupPolicySource{
					Volcano: &trainer.VolcanoPodGroupPolicySource{},
				},
			},
		},
	}
	rc := &nodev1.RuntimeClass{ObjectMeta: metav1.ObjectMeta{Name: "test-class"}}

	// case 1: TrainingRuntime
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(trainJob, tr).
		WithIndex(&trainer.TrainingRuntime{}, index.TrainingRuntimeContainerRuntimeClassKey,
			func(obj client.Object) []string {
				return []string{"test-class"}
			}).
		WithIndex(&trainer.ClusterTrainingRuntime{}, index.ClusterTrainingRuntimeContainerRuntimeClassKey,
			func(obj client.Object) []string {
				return []string{"test-class"}
			}).
		WithIndex(&trainer.TrainJob{}, runtimeindexer.TrainJobRuntimeRefKey,
			func(obj client.Object) []string {
				return []string{"test-runtime"}
			}).
		Build()

	runHandlerTest(t, cli, &PodGroupRuntimeClassHandler{}, rc)

	// case 2: ClusterTrainingRuntime
	cr := &trainer.ClusterTrainingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-runtime",
		},
		Spec: trainer.TrainingRuntimeSpec{
			PodGroupPolicy: &trainer.PodGroupPolicy{
				PodGroupPolicySource: trainer.PodGroupPolicySource{
					Volcano: &trainer.VolcanoPodGroupPolicySource{},
				},
			},
		},
	}

	cli2 := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(trainJob, cr).
		WithIndex(&trainer.TrainingRuntime{}, index.TrainingRuntimeContainerRuntimeClassKey,
			func(obj client.Object) []string {
				return []string{"test-class"}
			}).
		WithIndex(&trainer.ClusterTrainingRuntime{}, index.ClusterTrainingRuntimeContainerRuntimeClassKey,
			func(obj client.Object) []string {
				return []string{"test-class"}
			}).
		WithIndex(&trainer.TrainJob{}, runtimeindexer.TrainJobClusterRuntimeRefKey,
			func(obj client.Object) []string {
				return []string{"test-runtime"}
			}).
		Build()

	runHandlerTest(t, cli2, &PodGroupRuntimeClassHandler{}, rc)

}

func TestPodGroupLimitRangeHandler_AllEvents(t *testing.T) {
	scheme := apiruntime.NewScheme()
	_ = trainer.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	trainJob := &trainer.TrainJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
		},
		Spec: trainer.TrainJobSpec{
			Suspend: ptr.To(true),
		},
	}

	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(trainJob).Build()

	lr := &corev1.LimitRange{ObjectMeta: metav1.ObjectMeta{Name: "test-job", Namespace: "default"}}

	runHandlerTest(t, cli, &PodGroupLimitRangeHandler{}, lr)
}

func TestValidate(t *testing.T) {
	scheme := apiruntime.NewScheme()
	_ = schedulingv1.AddToScheme(scheme)

	tests := map[string]struct {
		annotations       map[string]string
		priorityClassName *string
		existingPriority  *schedulingv1.PriorityClass
		wantErr           bool
	}{
		"queue annotation missing": {
			annotations: map[string]string{},
			wantErr:     false,
		},
		"queue annotation empty": {
			annotations: map[string]string{
				volcanov1beta1.QueueNameAnnotationKey: "",
			},
			wantErr: true,
		},
		"queue annotation valid": {
			annotations: map[string]string{
				volcanov1beta1.QueueNameAnnotationKey: "default",
			},
			wantErr: false,
		},
		"priorityClassName is system-cluster-critical": {
			priorityClassName: ptr.To("system-cluster-critical"),
			wantErr:           false,
		},
		"priorityClassName does not exist": {
			priorityClassName: ptr.To("non-existent"),
			wantErr:           true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var objs []client.Object
			if tt.existingPriority != nil {
				objs = append(objs, tt.existingPriority)
			}

			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
			v := &Volcano{client: c}

			info := &runtime.Info{
				Annotations: tt.annotations,
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainer.PodGroupPolicy{
						PodGroupPolicySource: trainer.PodGroupPolicySource{
							Volcano: &trainer.VolcanoPodGroupPolicySource{},
						},
					},
				},
			}

			if tt.priorityClassName != nil {
				jobSetSpec := jobsetv1alpha2ac.JobSetSpec().
					WithReplicatedJobs(
						jobsetv1alpha2ac.ReplicatedJob().
							WithTemplate(
								batchv1ac.JobTemplateSpec().
									WithSpec(
										batchv1ac.JobSpec().
											WithTemplate(
												corev1ac.PodTemplateSpec().
													WithSpec(
														corev1ac.PodSpec().
															WithPriorityClassName(*tt.priorityClassName),
													),
											),
									),
							),
					)
				info.TemplateSpec = runtime.TemplateSpec{
					ObjApply: jobSetSpec,
				}
			}
			_, errs := v.Validate(context.Background(), info, nil, &trainer.TrainJob{})
			if (len(errs) > 0) != tt.wantErr {
				t.Errorf("expected error: %v, got: %v", tt.wantErr, errs)
			}
		})
	}
}
