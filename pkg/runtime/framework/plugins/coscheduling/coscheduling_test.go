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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"

	trainerv1alpha1 "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
	"github.com/kubeflow/trainer/v2/pkg/runtime/indexer"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
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
				Scheduler: &runtime.Scheduler{},
				Labels:    map[string]string{"key": "value"},
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
		"succeeded to build PodGroup with multiple PodSets": {
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
							Count: ptr.To[int32](2),
							SinglePodRequests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
						{
							Name:  "dataset-initializer",
							Count: ptr.To[int32](1),
							SinglePodRequests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("250m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
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
							Count: ptr.To[int32](2),
							SinglePodRequests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
						{
							Name:  "dataset-initializer",
							Count: ptr.To[int32](1),
							SinglePodRequests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("250m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
						},
					},
				},
			},
			objs: []client.Object{}, // Simulate no existing PodGroup
			wantObjs: []apiruntime.Object{
				utiltesting.MakeSchedulerPluginsPodGroup(metav1.NamespaceDefault, "trainJob").
					MinMember(3).
					MinResources(corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1.25"),
						corev1.ResourceMemory: resource.MustParse("2.5Gi"),
					}).
					SchedulingTimeout(30).
					ControllerReference(trainerv1alpha1.GroupVersion.WithKind(trainerv1alpha1.TrainJobKind), "trainJob", "trainJob").
					Obj(),
			},
		},
		"succeeded to build PodGroup with MinResources": {
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
							Count: ptr.To[int32](2),
							SinglePodRequests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
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
							Count: ptr.To[int32](2),
							SinglePodRequests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
					},
				},
			},
			objs: []client.Object{}, // Simulate no existing PodGroup
			wantObjs: []apiruntime.Object{
				utiltesting.MakeSchedulerPluginsPodGroup(metav1.NamespaceDefault, "trainJob").
					MinMember(2).
					MinResources(corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					}).
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
		"no action when PodGroup already exists and TrainJob is not suspended": {
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
		"no action when TrainJob is not suspended": {
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
							Count: ptr.To[int32](2),
							SinglePodRequests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
					},
				},
			},
			trainJob: &trainerv1alpha1.TrainJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existingTrainJob",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: trainerv1alpha1.TrainJobSpec{
					Suspend: ptr.To(false),
				},
			},
			wantInfo: &runtime.Info{
				Scheduler: &runtime.Scheduler{
					PodLabels: map[string]string{
						"scheduling.x-k8s.io/pod-group": "existingTrainJob",
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
							Count: ptr.To[int32](2),
							SinglePodRequests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
					},
				},
			},
			objs: []client.Object{
				&schedulerpluginsv1alpha1.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "existingTrainJob",
						Namespace: metav1.NamespaceDefault,
					},
					Spec: schedulerpluginsv1alpha1.PodGroupSpec{
						MinMember: 2,
						MinResources: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
						ScheduleTimeoutSeconds: ptr.To[int32](30),
					},
				},
			},
			wantObjs: nil, // No new objects should be created
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
					if podGroup, ok := obj.(*schedulerpluginsv1alpha1.PodGroup); ok {
						// Check if the key matches the expected PodGroup
						if key.Name == "existingTrainJob" && key.Namespace == metav1.NamespaceDefault {
							// Simulate finding the PodGroup by copying the expected object into the provided obj
							*podGroup = schedulerpluginsv1alpha1.PodGroup{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "existingTrainJob",
									Namespace: metav1.NamespaceDefault,
								},
								Spec: schedulerpluginsv1alpha1.PodGroupSpec{
									MinMember: 2,
									MinResources: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("1"),
										corev1.ResourceMemory: resource.MustParse("2Gi"),
									},
									ScheduleTimeoutSeconds: ptr.To[int32](30),
								},
							}
							return nil
						}
					}

					if _, ok := obj.(*schedulerpluginsv1alpha1.PodGroup); ok && errors.Is(tc.wantBuildError, errorGetPodGroup) {
						return errorGetPodGroup
					}
					return client.Get(ctx, key, obj, opts...)
				},
			})
			cli := clientBuilder.Build()
			plugin, err := New(ctx, cli, utiltesting.AsIndex(clientBuilder), nil)
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

			var objs []apiruntime.ApplyConfiguration
			objs, err = plugin.(framework.ComponentBuilderPlugin).Build(ctx, tc.info, tc.trainJob)
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

func drainQueue(q workqueue.TypedRateLimitingInterface[reconcile.Request]) []reconcile.Request {
	var got []reconcile.Request
	for q.Len() > 0 {
		item, shutdown := q.Get()
		if shutdown {
			break
		}
		got = append(got, item)
		q.Done(item)
	}
	return got
}

func TestPodGroupLimitRangeHandler(t *testing.T) {
	reqCmpOpts := []gocmp.Option{
		cmpopts.SortSlices(func(a, b reconcile.Request) int {
			if c := cmp.Compare(a.Namespace, b.Namespace); c != 0 {
				return c
			}
			return cmp.Compare(a.Name, b.Name)
		}),
	}

	suspendedDefault := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "suspended-default").
		Suspend(true).
		Obj()
	activeDefault := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "active-default").
		Suspend(false).
		Obj()
	suspendedOtherNS := utiltesting.MakeTrainJobWrapper("other-ns", "suspended-other").
		Suspend(true).
		Obj()

	lr := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-lr",
			Namespace: metav1.NamespaceDefault,
		},
	}

	cases := map[string]struct {
		objs     []client.Object
		listErr  error
		trigger  func(ctx context.Context, h *PodGroupLimitRangeHandler, q workqueue.TypedRateLimitingInterface[reconcile.Request])
		wantReqs []reconcile.Request
	}{
		"create queues only suspended TrainJobs in same namespace": {
			objs: []client.Object{suspendedDefault, activeDefault, suspendedOtherNS},
			trigger: func(ctx context.Context, h *PodGroupLimitRangeHandler, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				h.Create(ctx, event.TypedCreateEvent[*corev1.LimitRange]{Object: lr}, q)
			},
			wantReqs: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Namespace: metav1.NamespaceDefault, Name: "suspended-default"}},
			},
		},
		"update queues suspended TrainJobs in same namespace": {
			objs: []client.Object{suspendedDefault, activeDefault},
			trigger: func(ctx context.Context, h *PodGroupLimitRangeHandler, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				h.Update(ctx, event.TypedUpdateEvent[*corev1.LimitRange]{ObjectOld: lr, ObjectNew: lr}, q)
			},
			wantReqs: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Namespace: metav1.NamespaceDefault, Name: "suspended-default"}},
			},
		},
		"delete queues suspended TrainJobs in same namespace": {
			objs: []client.Object{suspendedDefault, activeDefault},
			trigger: func(ctx context.Context, h *PodGroupLimitRangeHandler, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				h.Delete(ctx, event.TypedDeleteEvent[*corev1.LimitRange]{Object: lr}, q)
			},
			wantReqs: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Namespace: metav1.NamespaceDefault, Name: "suspended-default"}},
			},
		},
		"generic is a no-op": {
			objs: []client.Object{suspendedDefault},
			trigger: func(ctx context.Context, h *PodGroupLimitRangeHandler, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				h.Generic(ctx, event.TypedGenericEvent[*corev1.LimitRange]{Object: lr}, q)
			},
			wantReqs: nil,
		},
		"list error queues nothing": {
			objs:    []client.Object{suspendedDefault},
			listErr: errors.New("list failed"),
			trigger: func(ctx context.Context, h *PodGroupLimitRangeHandler, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				h.Create(ctx, event.TypedCreateEvent[*corev1.LimitRange]{Object: lr}, q)
			},
			wantReqs: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			clientBuilder := utiltesting.NewClientBuilder().WithObjects(tc.objs...)
			if tc.listErr != nil {
				clientBuilder.WithInterceptorFuncs(interceptor.Funcs{
					List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
						return tc.listErr
					},
				})
			}
			h := &PodGroupLimitRangeHandler{client: clientBuilder.Build()}
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			defer q.ShutDown()

			tc.trigger(ctx, h, q)
			got := drainQueue(q)
			if diff := gocmp.Diff(tc.wantReqs, got, reqCmpOpts...); diff != "" {
				t.Errorf("Unexpected queued requests (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestPodGroupRuntimeClassHandler(t *testing.T) {
	reqCmpOpts := []gocmp.Option{
		cmpopts.SortSlices(func(a, b reconcile.Request) int {
			if c := cmp.Compare(a.Namespace, b.Namespace); c != 0 {
				return c
			}
			return cmp.Compare(a.Name, b.Name)
		}),
	}

	rcName := "kata"
	otherRCName := "runc"
	rc := &nodev1.RuntimeClass{
		ObjectMeta: metav1.ObjectMeta{Name: rcName},
	}

	makeSpecWithRC := func(name string) trainerv1alpha1.TrainingRuntimeSpec {
		return trainerv1alpha1.TrainingRuntimeSpec{
			Template: trainerv1alpha1.JobSetTemplateSpec{
				Spec: jobsetv1alpha2.JobSetSpec{
					ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
						{
							Name: "node",
							Template: batchv1.JobTemplateSpec{
								Spec: batchv1.JobSpec{
									Template: corev1.PodTemplateSpec{
										Spec: corev1.PodSpec{
											RuntimeClassName: ptr.To(name),
										},
									},
								},
							},
						},
					},
				},
			},
		}
	}

	trWithRC := utiltesting.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "tr-kata").
		RuntimeSpec(makeSpecWithRC(rcName)).
		Obj()
	ctrWithRC := utiltesting.MakeClusterTrainingRuntimeWrapper("ctr-kata").
		RuntimeSpec(makeSpecWithRC(rcName)).
		Obj()
	trOtherRC := utiltesting.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "tr-other").
		RuntimeSpec(makeSpecWithRC(otherRCName)).
		Obj()

	trGVK := trainerv1alpha1.GroupVersion.WithKind(trainerv1alpha1.TrainingRuntimeKind)
	ctrGVK := trainerv1alpha1.GroupVersion.WithKind(trainerv1alpha1.ClusterTrainingRuntimeKind)

	suspendedTRJob := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "job-tr-suspended").
		RuntimeRef(trGVK, "tr-kata").
		Suspend(true).
		Obj()
	activeTRJob := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "job-tr-active").
		RuntimeRef(trGVK, "tr-kata").
		Suspend(false).
		Obj()
	suspendedCTRJob := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "job-ctr-suspended").
		RuntimeRef(ctrGVK, "ctr-kata").
		Suspend(true).
		Obj()
	suspendedOtherRCJob := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "job-other-rc").
		RuntimeRef(trGVK, "tr-other").
		Suspend(true).
		Obj()

	baseObjs := []client.Object{
		trWithRC,
		ctrWithRC,
		trOtherRC,
		suspendedTRJob,
		activeTRJob,
		suspendedCTRJob,
		suspendedOtherRCJob,
	}
	wantSuspended := []reconcile.Request{
		{NamespacedName: types.NamespacedName{Namespace: metav1.NamespaceDefault, Name: "job-ctr-suspended"}},
		{NamespacedName: types.NamespacedName{Namespace: metav1.NamespaceDefault, Name: "job-tr-suspended"}},
	}

	cases := map[string]struct {
		objs     []client.Object
		listErr  error
		trigger  func(ctx context.Context, h *PodGroupRuntimeClassHandler, q workqueue.TypedRateLimitingInterface[reconcile.Request])
		wantReqs []reconcile.Request
	}{
		"create queues suspended TrainJobs referencing matching TrainingRuntime and ClusterTrainingRuntime": {
			objs: baseObjs,
			trigger: func(ctx context.Context, h *PodGroupRuntimeClassHandler, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				h.Create(ctx, event.TypedCreateEvent[*nodev1.RuntimeClass]{Object: rc}, q)
			},
			wantReqs: wantSuspended,
		},
		"update queues suspended TrainJobs for updated RuntimeClass": {
			objs: baseObjs,
			trigger: func(ctx context.Context, h *PodGroupRuntimeClassHandler, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				h.Update(ctx, event.TypedUpdateEvent[*nodev1.RuntimeClass]{ObjectOld: rc, ObjectNew: rc}, q)
			},
			wantReqs: wantSuspended,
		},
		"delete queues suspended TrainJobs for deleted RuntimeClass": {
			objs: baseObjs,
			trigger: func(ctx context.Context, h *PodGroupRuntimeClassHandler, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				h.Delete(ctx, event.TypedDeleteEvent[*nodev1.RuntimeClass]{Object: rc}, q)
			},
			wantReqs: wantSuspended,
		},
		"generic is a no-op": {
			objs: baseObjs,
			trigger: func(ctx context.Context, h *PodGroupRuntimeClassHandler, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				h.Generic(ctx, event.TypedGenericEvent[*nodev1.RuntimeClass]{Object: rc}, q)
			},
			wantReqs: nil,
		},
		"list error queues nothing": {
			objs:    baseObjs,
			listErr: errors.New("list failed"),
			trigger: func(ctx context.Context, h *PodGroupRuntimeClassHandler, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				h.Create(ctx, event.TypedCreateEvent[*nodev1.RuntimeClass]{Object: rc}, q)
			},
			wantReqs: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			clientBuilder := utiltesting.NewClientBuilder().
				WithObjects(tc.objs...).
				WithIndex(&trainerv1alpha1.TrainingRuntime{}, indexer.TrainingRuntimeContainerRuntimeClassKey, indexer.IndexTrainingRuntimeContainerRuntimeClass).
				WithIndex(&trainerv1alpha1.ClusterTrainingRuntime{}, indexer.ClusterTrainingRuntimeContainerRuntimeClassKey, indexer.IndexClusterTrainingRuntimeContainerRuntimeClass).
				WithIndex(&trainerv1alpha1.TrainJob{}, indexer.TrainJobRuntimeRefKey, indexer.IndexTrainJobTrainingRuntime).
				WithIndex(&trainerv1alpha1.TrainJob{}, indexer.TrainJobClusterRuntimeRefKey, indexer.IndexTrainJobClusterTrainingRuntime)
			if tc.listErr != nil {
				clientBuilder.WithInterceptorFuncs(interceptor.Funcs{
					List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
						return tc.listErr
					},
				})
			}
			h := &PodGroupRuntimeClassHandler{client: clientBuilder.Build()}
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			defer q.ShutDown()

			tc.trigger(ctx, h, q)
			got := drainQueue(q)
			if diff := gocmp.Diff(tc.wantReqs, got, reqCmpOpts...); diff != "" {
				t.Errorf("Unexpected queued requests (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestReconcilerBuilders(t *testing.T) {
	cases := map[string]struct {
		withPodGroupCRD bool
		wantCount       int
	}{
		"returns nil when PodGroup CRD is not installed": {
			withPodGroupCRD: false,
			wantCount:       0,
		},
		"returns three reconciler builders when PodGroup CRD is installed": {
			withPodGroupCRD: true,
			wantCount:       3,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			clientBuilder := utiltesting.NewClientBuilder()
			if tc.withPodGroupCRD {
				mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{schedulerpluginsv1alpha1.SchemeGroupVersion})
				mapper.Add(schedulerpluginsv1alpha1.SchemeGroupVersion.WithKind("PodGroup"), meta.RESTScopeNamespace)
				clientBuilder.WithRESTMapper(mapper)
			}
			cli := clientBuilder.Build()
			p, err := New(ctx, cli, utiltesting.AsIndex(clientBuilder), nil)
			if err != nil {
				t.Fatalf("Failed to create plugin: %v", err)
			}
			builders := p.(framework.WatchExtensionPlugin).ReconcilerBuilders()
			if len(builders) != tc.wantCount {
				t.Errorf("Unexpected number of reconciler builders: want %d, got %d", tc.wantCount, len(builders))
			}
		})
	}
}
