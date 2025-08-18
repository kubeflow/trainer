package volcano

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/apply"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	jobsetv1alpha2ac "sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"
	volcanov1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"
	"volcano.sh/apis/pkg/client/applyconfiguration/scheduling/v1beta1"
	volcanov1beta1ac "volcano.sh/apis/pkg/client/applyconfiguration/scheduling/v1beta1"
)

type FieldIndexerFunc func(ctx context.Context, obj client.Object, field string, fn client.IndexerFunc) error

func (f FieldIndexerFunc) IndexField(ctx context.Context, obj client.Object, field string, fn client.IndexerFunc) error {
	return f(ctx, obj, field, fn)
}

func TestNewVolcano(t *testing.T) {
	scheme := apiruntime.NewScheme()
	if err := trainer.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add trainer scheme: %v", err)
	}

	cases := map[string]struct {
		indexerFunc     FieldIndexerFunc
		expectErr       string
		expectCalledSet map[string]bool
	}{
		"successfully registers all indexers": {
			indexerFunc: func(ctx context.Context, obj client.Object, field string, fn client.IndexerFunc) error {
				return nil
			},
			expectErr: "",
			expectCalledSet: map[string]bool{
				TrainingRuntimeContainerRuntimeClassKey:        true,
				ClusterTrainingRuntimeContainerRuntimeClassKey: true,
			},
		},
		"training runtime indexer fails": {
			indexerFunc: func(ctx context.Context, obj client.Object, field string, fn client.IndexerFunc) error {
				if field == TrainingRuntimeContainerRuntimeClassKey {
					return fmt.Errorf("test error")
				}
				return nil
			},
			expectErr: "setting index on runtimeClass for TrainingRuntime: test error",
		},
		"cluster training runtime indexer fails": {
			indexerFunc: func(ctx context.Context, obj client.Object, field string, fn client.IndexerFunc) error {
				if field == ClusterTrainingRuntimeContainerRuntimeClassKey {
					return fmt.Errorf("test error")
				}
				return nil
			},
			expectErr: "setting index on runtimeClass for ClusterTrainingRuntime: test error",
		},
	}

	for name, tc := range cases {
		called := map[string]bool{}
		wrappedIndexer := FieldIndexerFunc(func(ctx context.Context, obj client.Object, field string, fn client.IndexerFunc) error {
			called[field] = true
			return tc.indexerFunc(ctx, obj, field, fn)
		})

		_, err := New(context.Background(), fake.NewClientBuilder().WithScheme(scheme).Build(), wrappedIndexer)

		gotErrStr := ""
		if err != nil {
			gotErrStr = err.Error()
		}

		if diff := cmp.Diff(tc.expectErr, gotErrStr); diff != "" {
			t.Errorf("%s: unexpected error (-want +got):\n%s", name, diff)
		}

		if tc.expectCalledSet != nil {
			if diff := cmp.Diff(tc.expectCalledSet, called); diff != "" {
				t.Errorf("%s: indexer calls mismatch (-want +got):\n%s", name, diff)
			}
		}
	}
}

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

	got := info.Scheduler.PodLabels[volcanov1beta1.VolcanoGroupNameAnnotationKey]
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

func (e *errorClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
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
								NetworkTopology: &trainer.NetworkTopologySpec{
									Mode:               trainer.HardNetworkTopologyMode,
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

	v := &Volcano{
		scheme:     scheme,
		restMapper: nil, // not required for this test
	}

	builders := v.ReconcilerBuilders()
	if diff := cmp.Diff(1, len(builders)); diff != "" {
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
