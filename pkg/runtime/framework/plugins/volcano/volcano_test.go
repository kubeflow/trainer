package volcano

import (
	"context"
	"fmt"
	"testing"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/apply"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
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
	require.NoError(t, trainer.AddToScheme(scheme))

	// Case 1: Success case, no error in indexer
	called := map[string]bool{}
	indexer := FieldIndexerFunc(func(ctx context.Context, obj client.Object, field string, fn client.IndexerFunc) error {
		called[field] = true
		return nil
	})

	plugin, err := New(context.Background(), fake.NewClientBuilder().WithScheme(scheme).Build(), indexer)
	require.NoError(t, err)
	require.IsType(t, &Volcano{}, plugin)

	require.True(t, called[TrainingRuntimeContainerRuntimeClassKey], "TrainingRuntime index should be registered")
	require.True(t, called[ClusterTrainingRuntimeContainerRuntimeClassKey], "ClusterTrainingRuntime index should be registered")

	// Case 2: Simulate error in IndexField for TrainingRuntime
	indexerWithError := FieldIndexerFunc(func(ctx context.Context, obj client.Object, field string, fn client.IndexerFunc) error {
		if field == TrainingRuntimeContainerRuntimeClassKey {
			return fmt.Errorf("test error")
		}
		return nil
	})

	plugin, err = New(context.Background(), fake.NewClientBuilder().WithScheme(scheme).Build(), indexerWithError)
	require.Error(t, err)
	require.Contains(t, err.Error(), "test error", "Error should contain the simulated error message")

	// Case 3: Simulate error in IndexField for ClusterTrainingRuntime
	indexerWithError = FieldIndexerFunc(func(ctx context.Context, obj client.Object, field string, fn client.IndexerFunc) error {
		if field == ClusterTrainingRuntimeContainerRuntimeClassKey {
			return fmt.Errorf("test error")
		}
		return nil
	})

	plugin, err = New(context.Background(), fake.NewClientBuilder().WithScheme(scheme).Build(), indexerWithError)
	require.Error(t, err)
	require.Contains(t, err.Error(), "test error", "Error should contain the simulated error message")
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
	require.Equal(t, jobName, got, "PodGroup label should match the train job name")

	// Test with nil info
	err = v.EnforcePodGroupPolicy(nil, trainJob)
	require.Nil(t, err, "should return nil when info is nil")
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
	// Test with nil info
	v := &Volcano{}
	res, _ := v.Build(context.Background(), nil, &trainer.TrainJob{})
	require.Nil(t, res, "should return nil when info is nil")

	scheme := apiruntime.NewScheme()
	require.NoError(t, trainer.AddToScheme(scheme))
	require.NoError(t, volcanov1beta1.AddToScheme(scheme))

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

	cases := []struct {
		testName   string
		trainJob   *trainer.TrainJob
		info       *runtime.Info
		existingPG client.Object
		expectPG   *v1beta1.PodGroupApplyConfiguration
		expectErr  error
	}{
		{
			testName: "PodGroup exists and trainjob not suspended",
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
		{
			testName: "PodGroup exists but trainjob suspended",
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
		{
			testName: "Error when getting existing PodGroup",
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

	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
			// fake for get existing PodGroup
			if c.existingPG != nil {
				clientBuilder.WithObjects(c.existingPG)
			}
			client_ := clientBuilder.Build()

			// fake for error handling
			if c.testName == "Error when getting existing PodGroup" {
				client_ = &errorClient{WithWatch: client_, errNotFound: true}
			}
			if c.testName == "PodGroup exists but trainjob suspended" {
				client_ = &errorClient{WithWatch: client_, errNotFound: false}
			}

			v := &Volcano{
				client:     client_,
				restMapper: nil,
				scheme:     scheme,
			}
			objs, err := v.Build(ctx, c.info, c.trainJob)

			if c.expectErr != nil {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			if c.expectPG == nil {
				require.Nil(t, objs)
				return
			}
			require.NotNil(t, objs)
			require.Len(t, objs, 1)

			actualPodGroup, ok := objs[0].(*v1beta1.PodGroupApplyConfiguration)
			if !ok {
				t.Fatalf("expected PodGroupApplyConfiguration, got %T", objs[0])
			}
			require.Equal(t, c.expectPG.Name, actualPodGroup.Name, "PodGroup name should match")
			require.Equal(t, c.expectPG.Namespace, actualPodGroup.Namespace, "PodGroup namespace should match")
			require.Equal(t, c.expectPG.Spec.Queue, actualPodGroup.Spec.Queue, "Queue should match")
			require.Equal(t, c.expectPG.Spec.PriorityClassName, actualPodGroup.Spec.PriorityClassName, "PriorityClassName should match")
			require.Equal(t, c.expectPG.Spec.MinMember, actualPodGroup.Spec.MinMember, "MinMember should match")
			for k, v := range *c.expectPG.Spec.MinResources {
				actualValue := (*actualPodGroup.Spec.MinResources)[k]
				require.Equal(t, v.String(), actualValue.String(), "MinResources for %s should match", k)
			}
		})
	}
}
func TestReconcilerBuilders(t *testing.T) {
	scheme := apiruntime.NewScheme()
	require.NoError(t, trainer.AddToScheme(scheme))
	require.NoError(t, volcanov1beta1.AddToScheme(scheme))

	v := &Volcano{
		scheme:     scheme,
		restMapper: nil, // not required for this test
	}

	builders := v.ReconcilerBuilders()
	require.Len(t, builders, 1, "should return one builder function")

	// We just check that the builder returns without panic
	b := &builder.Builder{}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	b = builders[0](b, cl, nil) // cache is not used in this test
	require.NotNil(t, b, "builder should not be nil after applying the reconciler builder function")
}
