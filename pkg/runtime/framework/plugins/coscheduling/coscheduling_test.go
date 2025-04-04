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
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"
	schedulerpluginsv1alpha1ac "sigs.k8s.io/scheduler-plugins/pkg/generated/applyconfiguration/scheduling/v1alpha1"

	trainerv1alpha1 "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/runtime"
	"github.com/kubeflow/trainer/pkg/runtime/framework"
	testingutil "github.com/kubeflow/trainer/pkg/util/testing"
)

func TestEnforcePodGroupPolicy(t *testing.T) {
	cases := map[string]struct {
		runtimeInfo    *runtime.Info
		trainJob       *trainerv1alpha1.TrainJob
		expectedLabels map[string]string
		wantError      error
	}{
		"successful enforcement": {
			runtimeInfo: &runtime.Info{
				Scheduler: &runtime.Scheduler{},
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &trainerv1alpha1.PodGroupPolicy{
						PodGroupPolicySource: trainerv1alpha1.PodGroupPolicySource{
							Coscheduling: &trainerv1alpha1.CoschedulingPodGroupPolicySource{
								ScheduleTimeoutSeconds: ptr.To[int32](99),
							},
						},
					},
				},
			},
			trainJob: &trainerv1alpha1.TrainJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: metav1.NamespaceDefault,
				},
			},
			expectedLabels: map[string]string{
				schedulerpluginsv1alpha1.PodGroupLabel: "test-job",
			},
			wantError: nil,
		},
		"successful with nil info": {
			runtimeInfo: nil,
			trainJob:    &trainerv1alpha1.TrainJob{},
			wantError:   nil,
		},
		"successful with nil pod group policy": {
			runtimeInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{},
			},
			trainJob:  &trainerv1alpha1.TrainJob{},
			wantError: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			clientBuilder := testingutil.NewClientBuilder()
			plugin, err := New(ctx, clientBuilder.Build(), testingutil.AsIndex(clientBuilder))
			if err != nil {
				t.Fatal(err)
			}
			p, ok := plugin.(framework.EnforcePodGroupPolicyPlugin)
			if !ok {
				t.Fatalf("Expected plugin to be of type EnforcePodGroupPolicyPlugin, got %T", plugin)
			}
			err = p.EnforcePodGroupPolicy(tc.runtimeInfo, tc.trainJob)
			if diff := cmp.Diff(tc.wantError, err); len(diff) != 0 {
				t.Errorf("Unexpected error (-want,+got):\n%s", diff)
			}
			if tc.runtimeInfo != nil && tc.runtimeInfo.Scheduler != nil {
				if diff := cmp.Diff(tc.expectedLabels, tc.runtimeInfo.Scheduler.PodLabels); len(diff) != 0 {
					t.Errorf("Unexpected pod labels (-want,+got):\n%s", diff)
				}
			}
		})
	}
}

func TestBuild(t *testing.T) {
	errorGetPodGroup := errors.New("failed to get PodGroup from API during Build")

	cases := map[string]struct {
		info           *runtime.Info
		trainJob       *trainerv1alpha1.TrainJob
		objs           []client.Object
		mockGetError   error
		expectedOutput []any
		wantBuildError error
	}{
		"succeeded to build PodGroup": {
			info: &runtime.Info{
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
			trainJob: &trainerv1alpha1.TrainJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: metav1.NamespaceDefault,
				},
			},
			objs:         []client.Object{}, // Simulate no existing PodGroup
			mockGetError: nil,
			expectedOutput: []any{
				func() *schedulerpluginsv1alpha1ac.PodGroupApplyConfiguration {
					trainJobName := "test-job"
					podGroup := schedulerpluginsv1alpha1ac.PodGroup(trainJobName, metav1.NamespaceDefault)
					podGroup.WithSpec(schedulerpluginsv1alpha1ac.PodGroupSpec().
						WithMinMember(1).
						WithMinResources(v1.ResourceList{}).
						WithScheduleTimeoutSeconds(30))
					podGroup.WithOwnerReferences(metav1ac.OwnerReference().
						WithAPIVersion(trainerv1alpha1.GroupVersion.String()).
						WithKind(trainerv1alpha1.TrainJobKind).
						WithName(trainJobName).
						WithUID("").
						WithController(true).
						WithBlockOwnerDeletion(true))
					return podGroup
				}(),
			},
			wantBuildError: nil,
		},
		"failed to get PodGroup due to API error": {
			info: &runtime.Info{
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
					Name:      "test-job",
					Namespace: metav1.NamespaceDefault,
				},
			},
			objs:           []client.Object{}, // No PodGroup to start with
			mockGetError:   errorGetPodGroup,
			expectedOutput: nil,
			wantBuildError: errorGetPodGroup, // Expect error from client.Get
		},
		"no action when PodGroup already exists": {
			info: &runtime.Info{
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
					Name:      "test-job",
					Namespace: metav1.NamespaceDefault,
				},
				Spec: trainerv1alpha1.TrainJobSpec{
					Suspend: ptr.To(false), // TrainJob is not suspended
				},
			},
			objs: []client.Object{
				// Simulate an existing PodGroup
				&schedulerpluginsv1alpha1.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job",
						Namespace: metav1.NamespaceDefault,
					},
				},
			},
			mockGetError:   nil,
			expectedOutput: nil, // No output since PodGroup should not be updated
			wantBuildError: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			clientBuilder := testingutil.NewClientBuilder().WithObjects(tc.objs...)
			clientBuilder.WithInterceptorFuncs(interceptor.Funcs{
				Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if _, ok := obj.(*schedulerpluginsv1alpha1.PodGroup); ok && tc.mockGetError != nil {
						return tc.mockGetError
					}
					return client.Get(ctx, key, obj, opts...)
				},
			})
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			plugin, err := New(ctx, clientBuilder.Build(), testingutil.AsIndex(clientBuilder))
			if err != nil {
				t.Fatal(err)
			}
			p, ok := plugin.(framework.ComponentBuilderPlugin)
			if !ok {
				t.Fatalf("Expected plugin to be of type EnforcePodGroupPolicyPlugin, got %T", plugin)
			}
			output, err := p.Build(context.Background(), tc.info, tc.trainJob)
			if diff := cmp.Diff(tc.wantBuildError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error from Build (-want, +got): %s", diff)
			}
			if diff := cmp.Diff(tc.expectedOutput, output); len(diff) != 0 {
				t.Errorf("Unexpected output from Build (-want, +got): %s", diff)
			}
		})
	}
}
