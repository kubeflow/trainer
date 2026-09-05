/*
Copyright 2026 The Kubeflow Authors.

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

package networkpolicy

import (
	"context"
	"testing"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"

	trainerv1alpha1 "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestNetworkPolicy(t *testing.T) {
	cases := map[string]struct {
		info                  *runtime.Info
		trainJob              *trainerv1alpha1.TrainJob
		wantInfo              *runtime.Info
		wantObjs              []apiruntime.Object
		wantBuildError        error
		wantPreBuildSyncError error
	}{
		"no action when info is nil": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				Obj(),
		},
		"no action when trainJob is nil": {
			info:     &runtime.Info{Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)}},
			wantInfo: &runtime.Info{Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)}},
		},
		"succeeded to build NetworkPolicy isolating the TrainJob Pods": {
			info: &runtime.Info{Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)}},
			wantInfo: &runtime.Info{Scheduler: &runtime.Scheduler{PodLabels: map[string]string{
				constants.LabelTrainJobName: "test-job",
			}}},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				Obj(),
			wantObjs: []apiruntime.Object{
				&networkingv1.NetworkPolicy{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "networking.k8s.io/v1",
						Kind:       "NetworkPolicy",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job" + constants.TrainJobNetworkPolicySuffix,
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainerv1alpha1.GroupVersion.String(),
							Kind:               trainerv1alpha1.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
					Spec: networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{constants.LabelTrainJobName: "test-job"},
						},
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
						Ingress: []networkingv1.NetworkPolicyIngressRule{{
							From: []networkingv1.NetworkPolicyPeer{{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{constants.LabelTrainJobName: "test-job"},
								},
							}},
						}},
					},
				},
			},
		},
		"NetworkPolicy is scoped to the TrainJob namespace and name": {
			info: &runtime.Info{Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)}},
			wantInfo: &runtime.Info{Scheduler: &runtime.Scheduler{PodLabels: map[string]string{
				constants.LabelTrainJobName: "alpha",
			}}},
			trainJob: utiltesting.MakeTrainJobWrapper("tenant-a", "alpha").
				UID("alpha-uid").
				Obj(),
			wantObjs: []apiruntime.Object{
				&networkingv1.NetworkPolicy{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "networking.k8s.io/v1",
						Kind:       "NetworkPolicy",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "alpha" + constants.TrainJobNetworkPolicySuffix,
						Namespace: "tenant-a",
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainerv1alpha1.GroupVersion.String(),
							Kind:               trainerv1alpha1.TrainJobKind,
							Name:               "alpha",
							UID:                "alpha-uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
					Spec: networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{constants.LabelTrainJobName: "alpha"},
						},
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
						Ingress: []networkingv1.NetworkPolicyIngressRule{{
							From: []networkingv1.NetworkPolicyPeer{{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{constants.LabelTrainJobName: "alpha"},
								},
							}},
						}},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)

			clientBuilder := utiltesting.NewClientBuilder()
			cli := clientBuilder.Build()
			plugin, err := New(ctx, cli, utiltesting.AsIndex(clientBuilder), nil)
			if err != nil {
				t.Fatalf("Failed to create plugin: %v", err)
			}

			err = plugin.(framework.PreComponentBuilderPlugin).PreBuildSync(tc.info, tc.trainJob)
			if diff := gocmp.Diff(tc.wantPreBuildSyncError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error from PreBuildSync (-want, +got): %s", diff)
			}
			if diff := gocmp.Diff(tc.wantInfo, tc.info); len(diff) != 0 {
				t.Errorf("Unexpected info from PreBuildSync (-want, +got): %s", diff)
			}

			var objs []apiruntime.ApplyConfiguration
			objs, err = plugin.(framework.ComponentBuilderPlugin).Build(ctx, tc.info, tc.trainJob)
			if diff := gocmp.Diff(tc.wantBuildError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error from Build (-want, +got): %s", diff)
			}
			typedObjs, err := utiltesting.ToObject(cli.Scheme(), objs...)
			if err != nil {
				t.Errorf("Failed to convert object: %v", err)
			}
			if diff := gocmp.Diff(tc.wantObjs, typedObjs); len(diff) != 0 {
				t.Errorf("Unexpected objects from Build (-want, +got): %s", diff)
			}
		})
	}
}
