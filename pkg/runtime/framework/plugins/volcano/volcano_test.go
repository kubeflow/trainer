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
	"cmp"
	"context"
	"errors"
	"strconv"
	"testing"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	batchv1ac "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	jobsetv1alpha2ac "sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"
	volcanov1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/apply"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestVolcano(t *testing.T) {
	objCmpOpts := []gocmp.Option{
		cmpopts.SortSlices(func(a, b apiruntime.Object) int {
			return cmp.Compare(a.GetObjectKind().GroupVersionKind().String(), b.GetObjectKind().GroupVersionKind().String())
		}),
		cmpopts.SortSlices(func(a, b corev1.EnvVar) int { return cmp.Compare(a.Name, b.Name) }),
	}

	errorGetPodGroup := errors.New("error when getting existing PodGroup")

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

	createBaseInfo := &runtime.Info{
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

	cases := map[string]struct {
		trainJob                   *trainer.TrainJob
		info                       *runtime.Info
		objs                       []client.Object
		expectInfo                 *runtime.Info
		expectObjs                 []apiruntime.Object
		expectEnforcePodGroupError error
		expectBuildError           error
	}{
		"Test nil info": {},
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
			objs: []client.Object{},
			expectInfo: &runtime.Info{
				Scheduler: &runtime.Scheduler{
					PodAnnotations: map[string]string{
						volcanov1beta1.KubeGroupNameAnnotationKey: "test-trainjob",
					},
				},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainer.PodGroupPolicy{
						PodGroupPolicySource: trainer.PodGroupPolicySource{
							Volcano: &trainer.VolcanoPodGroupPolicySource{},
						},
					},
				},
			},
			expectObjs: []apiruntime.Object{
				&volcanov1beta1.PodGroup{
					TypeMeta: metav1.TypeMeta{
						APIVersion: volcanov1beta1.SchemeGroupVersion.String(),
						Kind:       "PodGroup",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-trainjob",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         trainer.GroupVersion.String(),
								Kind:               trainer.TrainJobKind,
								Name:               "test-trainjob",
								UID:                types.UID(""),
								Controller:         ptr.To(true),
								BlockOwnerDeletion: ptr.To(true),
							},
						},
					},
					Spec: volcanov1beta1.PodGroupSpec{
						MinResources: &corev1.ResourceList{},
					},
				},
			},
			expectEnforcePodGroupError: nil,
			expectBuildError:           nil,
		},
		"PodGroup exists and trainjob not suspended": {
			trainJob: &trainer.TrainJob{
				ObjectMeta: metav1.ObjectMeta{Name: "job-exist-running", Namespace: "test-ns", UID: "1"},
				Spec:       trainer.TrainJobSpec{Suspend: ptr.To(false)},
			},
			info: &runtime.Info{
				TemplateSpec: createBaseInfo.TemplateSpec,
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainer.PodGroupPolicy{
						PodGroupPolicySource: trainer.PodGroupPolicySource{
							Volcano: &trainer.VolcanoPodGroupPolicySource{},
						},
					},
				},
				Scheduler: &runtime.Scheduler{},
			},
			objs: []client.Object{
				&volcanov1beta1.PodGroup{
					ObjectMeta: metav1.ObjectMeta{Name: "job-exist-running", Namespace: "test-ns"},
				},
			},
			expectInfo: &runtime.Info{
				TemplateSpec: createBaseInfo.TemplateSpec,
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainer.PodGroupPolicy{
						PodGroupPolicySource: trainer.PodGroupPolicySource{
							Volcano: &trainer.VolcanoPodGroupPolicySource{},
						},
					},
				},
				Scheduler: &runtime.Scheduler{
					PodAnnotations: map[string]string{
						volcanov1beta1.KubeGroupNameAnnotationKey: "job-exist-running",
					},
				},
			},
			expectObjs:                 nil,
			expectEnforcePodGroupError: nil,
			expectBuildError:           nil,
		},
		"PodGroup exists but trainjob suspended": {
			trainJob: &trainer.TrainJob{
				ObjectMeta: metav1.ObjectMeta{Name: "job-update", Namespace: "test-ns", UID: "2"},
				Spec:       trainer.TrainJobSpec{Suspend: ptr.To(true)},
			},
			info: &runtime.Info{
				TemplateSpec: createBaseInfo.TemplateSpec,
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
			objs: []client.Object{
				&volcanov1beta1.PodGroup{ObjectMeta: metav1.ObjectMeta{Name: "job-update", Namespace: "test-ns"}},
			},
			expectInfo: &runtime.Info{
				TemplateSpec: createBaseInfo.TemplateSpec,
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
				Scheduler: &runtime.Scheduler{
					PodAnnotations: map[string]string{
						volcanov1beta1.KubeGroupNameAnnotationKey: "job-update",
					},
				},
			},
			expectObjs: []apiruntime.Object{
				&volcanov1beta1.PodGroup{
					TypeMeta: metav1.TypeMeta{
						APIVersion: volcanov1beta1.SchemeGroupVersion.String(),
						Kind:       "PodGroup",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "job-update",
						Namespace: "test-ns",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         trainer.GroupVersion.String(),
								Kind:               trainer.TrainJobKind,
								Name:               "job-update",
								UID:                types.UID(strconv.Itoa(2)),
								Controller:         ptr.To(true),
								BlockOwnerDeletion: ptr.To(true),
							},
						},
					},
					Spec: volcanov1beta1.PodGroupSpec{
						MinMember: 5,
						MinResources: &corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2300m"),
							corev1.ResourceMemory: resource.MustParse("3Gi"),
						},
						Queue:             "q1",
						PriorityClassName: "high-priority",
						NetworkTopology: &volcanov1beta1.NetworkTopologySpec{
							Mode:               volcanov1beta1.HardNetworkTopologyMode,
							HighestTierAllowed: ptr.To(1),
						},
					},
				},
			},
			expectEnforcePodGroupError: nil,
			expectBuildError:           nil,
		},
		"Error when getting existing PodGroup": {
			trainJob: &trainer.TrainJob{
				ObjectMeta: metav1.ObjectMeta{Name: "job-error", Namespace: "test-ns", UID: "3"},
				Spec:       trainer.TrainJobSpec{Suspend: ptr.To(false)},
			},
			info: &runtime.Info{
				TemplateSpec: createBaseInfo.TemplateSpec,
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainer.PodGroupPolicy{
						PodGroupPolicySource: trainer.PodGroupPolicySource{
							Volcano: &trainer.VolcanoPodGroupPolicySource{},
						},
					},
				},
				Scheduler: &runtime.Scheduler{},
			},
			objs: nil,
			expectInfo: &runtime.Info{
				TemplateSpec: createBaseInfo.TemplateSpec,
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainer.PodGroupPolicy{
						PodGroupPolicySource: trainer.PodGroupPolicySource{
							Volcano: &trainer.VolcanoPodGroupPolicySource{},
						},
					},
				},
				Scheduler: &runtime.Scheduler{
					PodAnnotations: map[string]string{
						volcanov1beta1.KubeGroupNameAnnotationKey: "job-error",
					},
				},
			},
			expectObjs:                 nil,
			expectEnforcePodGroupError: nil,
			expectBuildError:           errorGetPodGroup,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)

			clientBuilder := utiltesting.NewClientBuilder().WithObjects(c.objs...)

			if name == "Error when getting existing PodGroup" {
				clientBuilder.WithInterceptorFuncs(interceptor.Funcs{
					Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						return errorGetPodGroup
					},
				})
			}

			cli := clientBuilder.Build()
			plugin, err := New(ctx, cli, utiltesting.AsIndex(clientBuilder))
			if err != nil {
				t.Fatalf("Failed to create plugin: %v", err)
			}

			// Test EnforcePodGroupPolicy
			err = plugin.(framework.EnforcePodGroupPolicyPlugin).EnforcePodGroupPolicy(c.info, c.trainJob)

			if diff := gocmp.Diff(c.expectEnforcePodGroupError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error from EnforcePodGroupPolicy (-want,+got):\n%s", diff)
			}
			if diff := gocmp.Diff(c.expectInfo, c.info,
				cmpopts.SortSlices(func(a, b string) bool { return a < b }),
				cmpopts.SortMaps(func(a, b int) bool { return a < b }),
			); len(diff) != 0 {
				t.Errorf("Unexpected info from EnforcePodGroupPolicy (-want,+got):\n%s", diff)
			}

			// Test Build
			var objs []any
			objs, err = plugin.(framework.ComponentBuilderPlugin).Build(ctx, c.info, c.trainJob)
			if diff := gocmp.Diff(c.expectBuildError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error from Build (-want, +got): %s", diff)
			}

			// Convert objects and compare
			var typedObjs []apiruntime.Object
			typedObjs, err = utiltesting.ToObject(cli.Scheme(), objs...)
			if err != nil {
				t.Errorf("Failed to convert object: %v", err)
			}
			if diff := gocmp.Diff(c.expectObjs, typedObjs, objCmpOpts...); len(diff) != 0 {
				t.Errorf("Unexpected objects from Build (-want, +got): %s", diff)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	cases := map[string]struct {
		info         *runtime.Info
		oldObj       *trainer.TrainJob
		newObj       *trainer.TrainJob
		objs         []client.Object
		wantError    field.ErrorList
		wantWarnings admission.Warnings
	}{
		"no action when info is nil": {},
		"no action when Volcano policy not enabled": {
			info: runtime.NewInfo(),
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Obj(),
		},
		"queue annotation is empty": {
			info: runtime.NewInfo(
				runtime.WithPodGroupPolicy(&trainer.PodGroupPolicy{
					PodGroupPolicySource: trainer.PodGroupPolicySource{
						Volcano: &trainer.VolcanoPodGroupPolicySource{},
					},
				}),
				runtime.WithAnnotations(map[string]string{
					volcanov1beta1.QueueNameAnnotationKey: "",
				}),
			),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Obj(),
			wantError: field.ErrorList{
				field.Invalid(
					field.NewPath("spec").Child("annotations").Key(volcanov1beta1.QueueNameAnnotationKey),
					"",
					"Volcano queue name must not be empty",
				),
			},
		},
		"priorityClassName does not exist": {
			info: runtime.NewInfo(
				runtime.WithPodGroupPolicy(&trainer.PodGroupPolicy{
					PodGroupPolicySource: trainer.PodGroupPolicySource{
						Volcano: &trainer.VolcanoPodGroupPolicySource{},
					},
				}),
				runtime.WithTemplateSpecObjApply(
					jobsetv1alpha2ac.JobSetSpec().
						WithReplicatedJobs(jobsetv1alpha2ac.ReplicatedJob().
							WithTemplate(batchv1ac.JobTemplateSpec().
								WithSpec(batchv1ac.JobSpec().
									WithTemplate(corev1ac.PodTemplateSpec().
										WithSpec(corev1ac.PodSpec().
											WithPriorityClassName("non-existent"),
										),
									),
								),
							),
						),
				),
			),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").Obj(),
			wantError: field.ErrorList{
				field.Invalid(
					field.NewPath("spec").Child("templateSpec").Child("priorityClassName"),
					"non-existent",
					`PriorityClass "non-existent" doesn't exist: priorityclasses.scheduling.k8s.io "non-existent" not found`,
				),
			},
		},
		"priorityClassName exists": {
			info: runtime.NewInfo(
				runtime.WithPodGroupPolicy(&trainer.PodGroupPolicy{
					PodGroupPolicySource: trainer.PodGroupPolicySource{
						Volcano: &trainer.VolcanoPodGroupPolicySource{},
					},
				}),
				runtime.WithTemplateSpecObjApply(runtime.TemplateSpec{
					ObjApply: jobsetv1alpha2ac.JobSetSpec().
						WithReplicatedJobs(jobsetv1alpha2ac.ReplicatedJob().
							WithTemplate(batchv1ac.JobTemplateSpec().
								WithSpec(batchv1ac.JobSpec().
									WithTemplate(corev1ac.PodTemplateSpec().
										WithSpec(corev1ac.PodSpec().
											WithPriorityClassName("existing-priority"),
										),
									),
								),
							),
						),
				}),
			),
			objs: []client.Object{
				&schedulingv1.PriorityClass{
					ObjectMeta: metav1.ObjectMeta{Name: "existing-priority"},
					Value:      100,
				},
			},
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").Obj(),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			ctx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)

			clientBuilder := utiltesting.NewClientBuilder().WithObjects(tc.objs...)
			cli := clientBuilder.Build()

			v, err := New(ctx, cli, nil)
			if err != nil {
				t.Fatalf("failed to init Volcano plugin: %v", err)
			}

			warnings, errs := v.(framework.CustomValidationPlugin).Validate(ctx, tc.info, tc.oldObj, tc.newObj)

			if diff := gocmp.Diff(tc.wantError, errs); diff != "" {
				t.Errorf("Unexpected Validate errors (-want,+got):\n%s", diff)
			}

			if diff := gocmp.Diff(tc.wantWarnings, warnings); diff != "" {
				t.Errorf("Unexpected Validate warnings (-want,+got):\n%s", diff)
			}
		})
	}
}
