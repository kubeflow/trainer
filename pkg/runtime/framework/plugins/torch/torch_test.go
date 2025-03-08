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

package torch

import (
	"context"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/apply"
	"github.com/kubeflow/trainer/pkg/constants"
	"github.com/kubeflow/trainer/pkg/runtime"
	"github.com/kubeflow/trainer/pkg/runtime/framework"
	utiltesting "github.com/kubeflow/trainer/pkg/util/testing"
)

func TestTorch(t *testing.T) {
	cases := map[string]struct {
		trainJob  *trainer.TrainJob
		info      *runtime.Info
		wantInfo  *runtime.Info
		wantError error
	}{
		"no action when info is null": {},
		"no action when mlPolicy is null": {
			info: runtime.NewInfo(
				runtime.WithLabels(map[string]string{"key": "value"}),
			),
			wantInfo: runtime.NewInfo(
				runtime.WithLabels(map[string]string{"key": "value"}),
			),
		},
		"no action when mlPolicy torch is null": {
			info: runtime.NewInfo(
				runtime.WithMLPolicy(utiltesting.MakeMLPolicyWrapper().Obj()),
			),
			wantInfo: runtime.NewInfo(
				runtime.WithMLPolicy(utiltesting.MakeMLPolicyWrapper().Obj()),
			),
		},
		"trainJob numNodes is respected rather then runtime mlPolicy numNodes": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().NumNodes(200).Obj(),
				).
				Obj(),
			info: runtime.NewInfo(
				runtime.WithMLPolicy(utiltesting.MakeMLPolicyWrapper().
					TorchPolicy("auto", nil).
					WithNumNodes(100).
					Obj(),
				),
			),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						TorchPolicy("auto", nil).
						WithNumNodes(100).
						Obj(),
				},
				Trainer: runtime.Trainer{
					NumNodes: ptr.To[int32](200),
					Env: apply.EnvVars([]corev1.EnvVar{
						{
							Name:  constants.TorchEnvNumNodes,
							Value: "200",
						},
						{
							Name:  constants.TorchEnvNumProcPerNode,
							Value: "auto",
						},
						{
							Name: constants.TorchEnvNodeRank,
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: constants.JobCompletionIndexFieldPath,
								},
							},
						},
						{
							Name:  constants.TorchEnvMasterAddr,
							Value: "test-trainer-node-0-0.test",
						},
						{
							Name:  constants.TorchEnvMasterPort,
							Value: strconv.Itoa(int(constants.ContainerTrainerPort)),
						},
					}...),
					ContainerPort: &corev1ac.ContainerPortApplyConfiguration{
						ContainerPort: ptr.To(constants.ContainerTrainerPort),
					},
				},
				Scheduler: &runtime.Scheduler{TotalRequests: make(map[string]runtime.TotalResourceRequest)},
			},
		},
		"trainJob trainer env is respected rather than runtime trainer env": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(200).
						ContainerEnv(corev1.EnvVar{Name: "CONFLICT", Value: "FROM_TRAINJOB"}).
						Obj(),
				).
				Obj(),
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().TorchPolicy("gpu", nil).Obj(),
				},
				Trainer: runtime.Trainer{
					Env: []corev1ac.EnvVarApplyConfiguration{{
						Name:  ptr.To("CONFLICT"),
						Value: ptr.To("FROM_RUNTIME_INFO"),
					}},
				},
				Scheduler: &runtime.Scheduler{TotalRequests: make(map[string]runtime.TotalResourceRequest)},
			},
			wantInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						TorchPolicy("gpu", nil).
						Obj(),
				},
				Trainer: runtime.Trainer{
					NumNodes: ptr.To[int32](200),
					Env: []corev1ac.EnvVarApplyConfiguration{
						{
							Name:  ptr.To("CONFLICT"),
							Value: ptr.To("FROM_TRAINJOB"),
						},
						{
							Name:  ptr.To(constants.TorchEnvNumNodes),
							Value: ptr.To("200"),
						},
						{
							Name:  ptr.To(constants.TorchEnvNumProcPerNode),
							Value: ptr.To("gpu"),
						},
						{
							Name: ptr.To(constants.TorchEnvNodeRank),
							ValueFrom: &corev1ac.EnvVarSourceApplyConfiguration{
								FieldRef: &corev1ac.ObjectFieldSelectorApplyConfiguration{
									FieldPath: ptr.To(constants.JobCompletionIndexFieldPath),
								},
							},
						},
						{
							Name:  ptr.To(constants.TorchEnvMasterAddr),
							Value: ptr.To("test-trainer-node-0-0.test"),
						},
						{
							Name:  ptr.To(constants.TorchEnvMasterPort),
							Value: ptr.To(strconv.Itoa(int(constants.ContainerTrainerPort))),
						},
					},
					ContainerPort: &corev1ac.ContainerPortApplyConfiguration{ContainerPort: ptr.To(constants.ContainerTrainerPort)},
				},
				Scheduler: &runtime.Scheduler{TotalRequests: make(map[string]runtime.TotalResourceRequest)},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)
			cliBuilder := utiltesting.NewClientBuilder()
			p, err := New(ctx, cliBuilder.Build(), nil)
			if err != nil {
				t.Fatalf("Failed to initialize Torch plugin: %v", err)
			}
			err = p.(framework.EnforceMLPolicyPlugin).EnforceMLPolicy(tc.info, tc.trainJob)
			if diff := cmp.Diff(tc.wantError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error from EnforceMLPolicy (-want, +got): %s", diff)
			}
			if diff := cmp.Diff(tc.wantInfo, tc.info,
				cmpopts.SortSlices(func(a, b string) bool { return a < b }),
				cmpopts.SortMaps(func(a, b string) bool { return a < b }),
			); len(diff) != 0 {
				t.Errorf("Unexpected RuntimeInfo (-want, +got): %s", diff)
			}
		})
	}
}
