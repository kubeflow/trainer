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

package coscheduling

import (
	"cmp"
	"context"
	"errors"
	"testing"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"

	trainerv1alpha1 "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/runtime"
	"github.com/kubeflow/trainer/pkg/runtime/framework"
	utiltesting "github.com/kubeflow/trainer/pkg/util/testing"
)

func TestCoScheduling(t *testing.T) {
	objCmpOpts := []gocmp.Option{
		cmpopts.SortSlices(func(a, b apiruntime.Object) int {
			return cmp.Compare(a.GetObjectKind().GroupVersionKind().String(), b.GetObjectKind().GroupVersionKind().String())
		}),
		cmpopts.SortSlices(func(a, b corev1.EnvVar) int { return cmp.Compare(a.Name, b.Name) }),
		gocmp.Comparer(utiltesting.MPISecretDataComparer),
	}
	errorGetPodGroup := errors.New("failed to get PodGroup from API during Build")

	cases := map[string]struct {
		info                    *runtime.Info
		trainJob                *trainerv1alpha1.TrainJob
		objs                    []client.Object
		wantInfo                *runtime.Info
		wantObjs                []apiruntime.Object
		wantPodGroupPolicyError error
		wantBuildError          error
	}{
		"no action when info is nil": {},
		"no action when podGroupPolicy is nil": {
			info: &runtime.Info{
				Labels: map[string]string{"key": "value"},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: nil,
				},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "trainJob").
				UID("trainJob").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(2).
						Obj()).
				Obj(),
			wantInfo: &runtime.Info{
				Labels: map[string]string{"key": "value"},
			},
		},
		"no action when coscheduling is nil": {
			info: &runtime.Info{
				Scheduler: &runtime.Scheduler{},
				Labels:    map[string]string{"key": "value"},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainerv1alpha1.PodGroupPolicy{
						PodGroupPolicySource: trainerv1alpha1.PodGroupPolicySource{
							Coscheduling: nil,
						},
					},
				},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "trainJob").
				UID("trainJob").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(2).
						Obj()).
				Obj(),
			wantInfo: &runtime.Info{
				Scheduler: &runtime.Scheduler{
					PodLabels: map[string]string{
						"scheduling.x-k8s.io/pod-group": "trainJob",
					},
				},
				Labels: map[string]string{"key": "value"},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainerv1alpha1.PodGroupPolicy{},
				},
			},
		},
		"no action when trainJob is nil": {
			info: &runtime.Info{
				Scheduler: &runtime.Scheduler{},
				Labels:    map[string]string{"key": "value"},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: nil,
				},
			},
			trainJob: nil,
			wantInfo: &runtime.Info{
				Labels:    map[string]string{"key": "value"},
				Scheduler: &runtime.Scheduler{},
			},
		},
		"succeeded to build PodGroup": {
			info: &runtime.Info{
				Scheduler: &runtime.Scheduler{},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainerv1alpha1.PodGroupPolicy{
						PodGroupPolicySource: trainerv1alpha1.PodGroupPolicySource{
							Coscheduling: &trainerv1alpha1.CoschedulingPodGroupPolicySource{
								ScheduleTimeoutSeconds: ptr.To[int32](30),
							},
						},
					},
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:  "node",
							Count: ptr.To[int32](1),
						},
					},
				},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "trainJob").
				UID("trainJob").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(2).
						Obj()).
				Obj(),
			wantInfo: &runtime.Info{
				Scheduler: &runtime.Scheduler{
					PodLabels: map[string]string{
						"scheduling.x-k8s.io/pod-group": "trainJob",
					},
				},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainerv1alpha1.PodGroupPolicy{
						PodGroupPolicySource: trainerv1alpha1.PodGroupPolicySource{
							Coscheduling: &trainerv1alpha1.CoschedulingPodGroupPolicySource{
								ScheduleTimeoutSeconds: ptr.To[int32](30),
							},
						},
					},
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:  "node",
							Count: ptr.To[int32](1),
						},
					},
				},
			},
			objs: []client.Object{}, // Simulate no existing PodGroup
			wantObjs: []apiruntime.Object{
				utiltesting.MakeSchedulerPluginsPodGroup(metav1.NamespaceDefault, "trainJob").
					MinMember(1).
					MinResources(corev1.ResourceList{}).
					SchedulingTimeout(30).
					ControllerReference(trainerv1alpha1.GroupVersion.WithKind(trainerv1alpha1.TrainJobKind), "trainJob", "trainJob").
					Obj(),
			},
		},
		"failed to get PodGroup due to API error": {
			info: &runtime.Info{
				Scheduler: &runtime.Scheduler{},
				Labels:    map[string]string{"key": "value"},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainerv1alpha1.PodGroupPolicy{
						PodGroupPolicySource: trainerv1alpha1.PodGroupPolicySource{
							Coscheduling: &trainerv1alpha1.CoschedulingPodGroupPolicySource{
								ScheduleTimeoutSeconds: ptr.To[int32](30),
							},
						},
					},
				},
			},
			trainJob: &trainerv1alpha1.TrainJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "trainJob",
					Namespace: metav1.NamespaceDefault,
				},
			},
			wantInfo: &runtime.Info{
				Scheduler: &runtime.Scheduler{
					PodLabels: map[string]string{
						"scheduling.x-k8s.io/pod-group": "trainJob",
					},
				},
				Labels: map[string]string{"key": "value"},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainerv1alpha1.PodGroupPolicy{
						PodGroupPolicySource: trainerv1alpha1.PodGroupPolicySource{
							Coscheduling: &trainerv1alpha1.CoschedulingPodGroupPolicySource{
								ScheduleTimeoutSeconds: ptr.To[int32](30),
							},
						},
					},
				},
			},
			wantBuildError: errorGetPodGroup,
		},
		"no action when PodGroup already exists": {
			info: &runtime.Info{
				Scheduler: &runtime.Scheduler{},
				Labels:    map[string]string{"key": "value"},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainerv1alpha1.PodGroupPolicy{
						PodGroupPolicySource: trainerv1alpha1.PodGroupPolicySource{
							Coscheduling: &trainerv1alpha1.CoschedulingPodGroupPolicySource{
								ScheduleTimeoutSeconds: ptr.To[int32](30),
							},
						},
					},
				},
			},
			trainJob: &trainerv1alpha1.TrainJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "trainJob",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: trainerv1alpha1.TrainJobSpec{
					Suspend: ptr.To(false),
				},
			},
			objs: []client.Object{
				utiltesting.MakeSchedulerPluginsPodGroup(metav1.NamespaceDefault, "trainJob").
					MinMember(1).
					MinResources(corev1.ResourceList{}).
					SchedulingTimeout(30).
					ControllerReference(trainerv1alpha1.GroupVersion.WithKind(trainerv1alpha1.TrainJobKind), "trainJob", "trainJob").
					Obj(),
			},
			wantInfo: &runtime.Info{
				Scheduler: &runtime.Scheduler{
					PodLabels: map[string]string{
						"scheduling.x-k8s.io/pod-group": "trainJob",
					},
				},
				Labels: map[string]string{"key": "value"},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainerv1alpha1.PodGroupPolicy{
						PodGroupPolicySource: trainerv1alpha1.PodGroupPolicySource{
							Coscheduling: &trainerv1alpha1.CoschedulingPodGroupPolicySource{
								ScheduleTimeoutSeconds: ptr.To[int32](30),
							},
						},
					},
				},
			},
			wantObjs:                nil,
			wantPodGroupPolicyError: nil,
			wantBuildError:          nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)
			clientBuilder := utiltesting.NewClientBuilder().WithObjects(tc.objs...)
			clientBuilder.WithInterceptorFuncs(interceptor.Funcs{
				Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if _, ok := obj.(*schedulerpluginsv1alpha1.PodGroup); ok && errors.Is(tc.wantBuildError, errorGetPodGroup) {
						return errorGetPodGroup
					}
					return client.Get(ctx, key, obj, opts...)
				},
			})
			cli := clientBuilder.Build()
			plugin, err := New(ctx, cli, utiltesting.AsIndex(clientBuilder))
			if err != nil {
				t.Fatalf("Failed to create plugin: %v", err)
			}
			err = plugin.(framework.EnforcePodGroupPolicyPlugin).EnforcePodGroupPolicy(tc.info, tc.trainJob)
			if diff := gocmp.Diff(tc.wantPodGroupPolicyError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error from EnforcePodGroupPolicy (-want,+got):\n%s", diff)
			}
			if diff := gocmp.Diff(tc.wantInfo, tc.info,
				cmpopts.SortSlices(func(a, b string) bool { return a < b }),
				cmpopts.SortMaps(func(a, b int) bool { return a < b }),
			); len(diff) != 0 {
				t.Errorf("Unexpected info from EnforcePodGroupPolicy (-want,+got):\n%s", diff)
			}

			var objs []any
			objs, err = plugin.(framework.ComponentBuilderPlugin).Build(context.Background(), tc.info, tc.trainJob)
			if diff := gocmp.Diff(tc.wantBuildError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error from Build (-want, +got): %s", diff)
			}
			var typedObjs []apiruntime.Object
			typedObjs, err = utiltesting.ToObject(cli.Scheme(), objs...)
			if err != nil {
				t.Errorf("Failed to convert object: %v", err)
			}
			if diff := gocmp.Diff(tc.wantObjs, typedObjs, objCmpOpts...); len(diff) != 0 {
				t.Errorf("Unexpected objects from Build (-want, +got): %s", diff)
			}
		})
	}
}
