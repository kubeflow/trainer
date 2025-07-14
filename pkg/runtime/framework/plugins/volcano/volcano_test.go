package volcano

import (
	"context"
	"testing"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
}

func TestEnforcePodGroupPolicy(t *testing.T) {
	v := &Volcano{}

	jobName := "test-trainjob"
	info := &runtime.Info{
		Scheduler: &runtime.Scheduler{
			PodLabels: map[string]string{},
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

	got := info.Scheduler.PodLabels["volcano.sh/podgroup"]
	if got != jobName {
		t.Errorf("expected label volcano.sh/podgroup=%s, got %s", jobName, got)
	}
}

func TestBuildPodGroup(t *testing.T) {
	scheme := apiruntime.NewScheme()
	require.NoError(t, trainer.AddToScheme(scheme))
	require.NoError(t, volcanov1beta1.AddToScheme(scheme))

	ctx := context.Background()

	baseInfo := func() *runtime.Info {
		return &runtime.Info{
			TemplateSpec: runtime.TemplateSpec{
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
							Volcano: &trainer.VolcanoPodGroupPolicySource{
								Queue: ptr.To("q1"),
							},
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
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainer.PodGroupPolicy{
						PodGroupPolicySource: trainer.PodGroupPolicySource{
							Volcano: &trainer.VolcanoPodGroupPolicySource{
								Queue:             ptr.To("q1"),
								PriorityClassName: ptr.To("high-priority"),
								MinTaskMember: map[string]int32{
									"launcher": 1,
									"worker":   2,
								},
							},
						},
					},
				},
			},
			existingPG: &volcanov1beta1.PodGroup{ObjectMeta: metav1.ObjectMeta{Name: "job-update", Namespace: "test-ns"}},
			expectPG: volcanov1beta1ac.PodGroup("job-update", "test-ns").
				WithSpec(volcanov1beta1ac.PodGroupSpec().
					WithMinMember(3).
					WithMinResources(corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1300m"),
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					}).
					WithMinTaskMember(map[string]int32{
						"launcher": 1,
						"worker":   2}).
					WithQueue("q1").
					WithPriorityClassName("high-priority")),
			expectErr: nil,
		},
		{
			testName: "Test PodGroup creation with default value of minTaskMember",
			trainJob: &trainer.TrainJob{
				ObjectMeta: metav1.ObjectMeta{Name: "job-new", Namespace: "test-ns", UID: "3"},
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
			expectPG: volcanov1beta1ac.PodGroup("job-new", "test-ns").
				WithSpec(volcanov1beta1ac.PodGroupSpec().
					WithMinMember(5).
					WithMinResources(corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2300m"),
						corev1.ResourceMemory: resource.MustParse("3Gi"),
					}).
					WithMinTaskMember(map[string]int32{
						"launcher": 1,
						"worker":   4})),
			expectErr: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
			// fake for get existing PodGroup
			if c.existingPG != nil {
				clientBuilder.WithObjects(c.existingPG)
			}

			v := &Volcano{
				client:     clientBuilder.Build(),
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
			require.Equal(t, c.expectPG.Spec.MinTaskMember, actualPodGroup.Spec.MinTaskMember, "MinTaskMember should match")
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
