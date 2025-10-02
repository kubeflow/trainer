/*
Copyright 2025 The Kubeflow Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package volcano

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	batchv1ac "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	jobsetv1alpha2ac "sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"
	volcanov1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"
	"volcano.sh/apis/pkg/client/applyconfiguration/scheduling/v1beta1"
	volcanov1beta1ac "volcano.sh/apis/pkg/client/applyconfiguration/scheduling/v1beta1"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/apply"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
)

func TestVolcano(t *testing.T) {
	scheme := apiruntime.NewScheme()
	if err := trainer.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add trainer scheme: %v", err)
	}
	if err := volcanov1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add volcano scheme: %v", err)
	}

	ctx := context.Background()
	// Test Name()
	t.Run("Name", func(t *testing.T) {
		v := &Volcano{}
		expectedName := "Volcano"
		if v.Name() != expectedName {
			t.Errorf("expected name %s, got %s", expectedName, v.Name())
		}
	})

	// Test ReconcilerBuilders()
	t.Run("ReconcilerBuilders", func(t *testing.T) {
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
	})

	// Test Build()
	mockGetError := errors.New("mock get error")

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
		expectInfo *runtime.Info
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
		"inject group-name annotation": {
			trainJob: &trainer.TrainJob{ObjectMeta: metav1.ObjectMeta{Name: "test-trainjob"}},
			info: &runtime.Info{
				Scheduler: &runtime.Scheduler{},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainer.PodGroupPolicy{
						PodGroupPolicySource: trainer.PodGroupPolicySource{
							Volcano: &trainer.VolcanoPodGroupPolicySource{},
						},
					},
				},
			},
			expectInfo: &runtime.Info{
				Scheduler: &runtime.Scheduler{
					PodAnnotations: map[string]string{
						volcanov1beta1.KubeGroupNameAnnotationKey: "test-trainjob",
					},
				},
			},
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
				Scheduler: &runtime.Scheduler{},
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
				Scheduler: &runtime.Scheduler{},
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
				Scheduler: &runtime.Scheduler{},
			},
			existingPG: nil,
			expectPG:   nil,
			expectErr:  mockGetError,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
			if c.existingPG != nil {
				clientBuilder.WithObjects(c.existingPG)
			}
			if name == "Error when getting existing PodGroup" {
				clientBuilder.WithInterceptorFuncs(interceptor.Funcs{
					Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						return mockGetError
					},
				})
			}
			cli := clientBuilder.Build()

			v := &Volcano{
				client: cli,
				scheme: scheme,
			}

			_ = v.EnforcePodGroupPolicy(c.info, c.trainJob)
			if c.expectInfo != nil {
				if diff := cmp.Diff(c.expectInfo.Scheduler.PodAnnotations, c.info.Scheduler.PodAnnotations); diff != "" {
					t.Errorf("PodAnnotations mismatch (-want +got):\n%s", diff)
				}
			}

			objs, err := v.Build(ctx, c.info, c.trainJob)
			if diff := cmp.Diff(c.expectErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch:\n%s", diff)
			}
			if c.expectPG != nil {
				if len(objs) != 1 {
					t.Fatalf("expected 1 object, got %d", len(objs))
				}
				actualPG := objs[0].(*v1beta1.PodGroupApplyConfiguration)
				if diff := cmp.Diff(c.expectPG, actualPG); diff != "" {
					t.Errorf("PodGroup mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}

	// Test Validate()
	validationTests := map[string]struct {
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

	for name, tt := range validationTests {
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
