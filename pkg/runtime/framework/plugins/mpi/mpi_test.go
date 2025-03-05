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

package mpi

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/kubeflow/trainer/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/runtime"
	"github.com/kubeflow/trainer/pkg/runtime/framework"
	utiltesting "github.com/kubeflow/trainer/pkg/util/testing"
)

func TestMPI(t *testing.T) {
	objCmpOpts := []cmp.Option{
		cmpopts.SortSlices(func(a, b apiruntime.Object) bool {
			return a.GetObjectKind().GroupVersionKind().String() < b.GetObjectKind().GroupVersionKind().String()
		}),
		cmp.Comparer(secretDataComparer),
	}

	cases := map[string]struct {
		info              *runtime.Info
		trainJob          *trainer.TrainJob
		wantInfo          *runtime.Info
		wantObjs          []apiruntime.Object
		wantMLPolicyError error
		wantBuildError    error
	}{
		"no action when info is nil": {},
		"no action when mlPolicy is nil": {
			info: runtime.NewInfo(
				runtime.WithLabels(map[string]string{"key": "value"}),
			),
			wantInfo: runtime.NewInfo(
				runtime.WithLabels(map[string]string{"key": "value"}),
			),
		},
		"no action when mlPolicy mpi is null": {
			info: runtime.NewInfo(
				runtime.WithMLPolicy(utiltesting.MakeMLPolicyWrapper().
					Obj()),
			),
			wantInfo: runtime.NewInfo(
				runtime.WithMLPolicy(utiltesting.MakeMLPolicyWrapper().
					Obj()),
			),
		},
		"trainJob numNodes is respected rather than mlPolicy one": {
			info: runtime.NewInfo(
				runtime.WithMLPolicy(
					utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						MPIPolicy(ptr.To[int32](1), ptr.To(trainer.MPIImplementationOpenMPI), ptr.To("/root/.ssh"), nil).
						Obj(),
				),
			),
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "trainJob").
				UID("trainJob").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(2).
						Obj()).
				Obj(),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						MPIPolicy(ptr.To[int32](1), ptr.To(trainer.MPIImplementationOpenMPI), ptr.To("/root/.ssh"), nil).
						Obj(),
				},
				Trainer: runtime.Trainer{
					NumNodes:       ptr.To[int32](2),
					NumProcPerNode: "1",
					Env: []corev1ac.EnvVarApplyConfiguration{{
						Name:  ptr.To(constants.OpenMPIEnvHostFileLocation),
						Value: ptr.To(fmt.Sprintf("%s/%s", constants.MPIHostfileDir, constants.MPIHostfileName)),
					}},
					ContainerPort: &corev1ac.ContainerPortApplyConfiguration{
						ContainerPort: ptr.To[int32](constants.ContainerTrainerPort),
					},
					Volumes: []corev1ac.VolumeApplyConfiguration{
						*corev1ac.Volume().
							WithName(constants.MPISSHAuthVolumeName).
							WithSecret(corev1ac.SecretVolumeSource().
								WithSecretName(fmt.Sprintf("trainJob%s", constants.MPISSHAuthSecretSuffix)).
								WithItems(
									corev1ac.KeyToPath().WithKey(corev1.SSHAuthPrivateKey).WithPath(constants.MPISSHPrivateKeyFile),
									corev1ac.KeyToPath().WithKey(constants.MPISSHPublicKey).WithPath(constants.MPISSHPublicKeyFile),
									corev1ac.KeyToPath().WithKey(constants.MPISSHPublicKey).WithPath(constants.MPISSHAuthorizedKeys),
								),
							),
						*corev1ac.Volume().
							WithName(constants.MPIHostfileVolumeName).
							WithConfigMap(corev1ac.ConfigMapVolumeSource().
								WithName(fmt.Sprintf("trainJob-%s", constants.MPIHostfileVolumeName))),
					},
					VolumeMounts: []corev1ac.VolumeMountApplyConfiguration{
						*corev1ac.VolumeMount().WithName(constants.MPISSHAuthVolumeName).WithMountPath("/root/.ssh"),
						*corev1ac.VolumeMount().WithName(constants.MPIHostfileVolumeName).WithMountPath(constants.MPIHostfileDir),
					},
				},
				Scheduler: &runtime.Scheduler{TotalRequests: make(map[string]runtime.TotalResourceRequest)},
			},
			wantObjs: []apiruntime.Object{
				utiltesting.MakeSecretWrapper(fmt.Sprintf("trainJob%s", constants.MPISSHAuthSecretSuffix), metav1.NamespaceDefault).
					WithType(corev1.SecretTypeSSHAuth).
					WithData(map[string][]byte{
						constants.MPISSHPublicKey: []byte("EXIST"),
						corev1.SSHAuthPrivateKey:  []byte("EXIST"),
					}).
					OwnerReference(trainer.SchemeGroupVersion.WithKind("TrainJob"), "trainJob", "trainJob").
					Obj(),
				utiltesting.MakeConfigMapWrapper(fmt.Sprintf("trainJob%s", constants.MPIHostfileConfigMapSuffix), metav1.NamespaceDefault).
					WithData(map[string]string{
						constants.MPIHostfileName: `trainJob-trainer-node-0-0.1 slots=trainJob
trainJob-trainer-node-0-1.1 slots=trainJob
`,
					}).
					OwnerReference(trainer.SchemeGroupVersion.WithKind("TrainJob"), "trainJob", "trainJob").
					Obj(),
			},
		},
		"trainJob numProcPerNode is respected rather than mlPolicy one": {
			info: runtime.NewInfo(
				runtime.WithMLPolicy(
					utiltesting.MakeMLPolicyWrapper().
						MPIPolicy(ptr.To[int32](1), ptr.To(trainer.MPIImplementationOpenMPI), ptr.To("/root/.ssh"), nil).
						Obj(),
				),
			),
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "trainJob").
				UID("trainJob").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(1).
						NumProcPerNode(intstr.FromInt32(2)).
						Obj()).
				Obj(),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						MPIPolicy(ptr.To[int32](1), ptr.To(trainer.MPIImplementationOpenMPI), ptr.To("/root/.ssh"), nil).
						Obj(),
				},
				Trainer: runtime.Trainer{
					NumNodes:       ptr.To[int32](1),
					NumProcPerNode: "2",
					Env: []corev1ac.EnvVarApplyConfiguration{{
						Name:  ptr.To(constants.OpenMPIEnvHostFileLocation),
						Value: ptr.To(fmt.Sprintf("%s/%s", constants.MPIHostfileDir, constants.MPIHostfileName)),
					}},
					ContainerPort: &corev1ac.ContainerPortApplyConfiguration{
						ContainerPort: ptr.To[int32](constants.ContainerTrainerPort),
					},
					Volumes: []corev1ac.VolumeApplyConfiguration{
						*corev1ac.Volume().
							WithName(constants.MPISSHAuthVolumeName).
							WithSecret(corev1ac.SecretVolumeSource().
								WithSecretName(fmt.Sprintf("trainJob%s", constants.MPISSHAuthSecretSuffix)).
								WithItems(
									corev1ac.KeyToPath().WithKey(corev1.SSHAuthPrivateKey).WithPath(constants.MPISSHPrivateKeyFile),
									corev1ac.KeyToPath().WithKey(constants.MPISSHPublicKey).WithPath(constants.MPISSHPublicKeyFile),
									corev1ac.KeyToPath().WithKey(constants.MPISSHPublicKey).WithPath(constants.MPISSHAuthorizedKeys),
								),
							),
						*corev1ac.Volume().
							WithName(constants.MPIHostfileVolumeName).
							WithConfigMap(corev1ac.ConfigMapVolumeSource().
								WithName(fmt.Sprintf("trainJob-%s", constants.MPIHostfileVolumeName))),
					},
					VolumeMounts: []corev1ac.VolumeMountApplyConfiguration{
						*corev1ac.VolumeMount().WithName(constants.MPISSHAuthVolumeName).WithMountPath("/root/.ssh"),
						*corev1ac.VolumeMount().WithName(constants.MPIHostfileVolumeName).WithMountPath(constants.MPIHostfileDir),
					},
				},
				Scheduler: &runtime.Scheduler{TotalRequests: make(map[string]runtime.TotalResourceRequest)},
			},
			wantObjs: []apiruntime.Object{
				utiltesting.MakeSecretWrapper(fmt.Sprintf("trainJob%s", constants.MPISSHAuthSecretSuffix), metav1.NamespaceDefault).
					WithType(corev1.SecretTypeSSHAuth).
					WithData(map[string][]byte{
						constants.MPISSHPublicKey: []byte("EXIST"),
						corev1.SSHAuthPrivateKey:  []byte("EXIST"),
					}).
					OwnerReference(trainer.SchemeGroupVersion.WithKind("TrainJob"), "trainJob", "trainJob").
					Obj(),
				utiltesting.MakeConfigMapWrapper(fmt.Sprintf("trainJob%s", constants.MPIHostfileConfigMapSuffix), metav1.NamespaceDefault).
					WithData(map[string]string{
						constants.MPIHostfileName: `trainJob-trainer-node-0-0.2 slots=trainJob
`,
					}).
					OwnerReference(trainer.SchemeGroupVersion.WithKind("TrainJob"), "trainJob", "trainJob").
					Obj(),
			},
		},
		"calculated numNodes are propagated to info totalRequests": {
			info: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						MPIPolicy(ptr.To[int32](1), ptr.To(trainer.MPIImplementationOpenMPI), ptr.To("/root/.ssh"), nil).
						Obj(),
				},
				Scheduler: &runtime.Scheduler{TotalRequests: map[string]runtime.TotalResourceRequest{
					constants.JobTrainerNode: {
						Replicas: 1,
						PodRequests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("10"),
						},
					},
					constants.JobInitializer: {},
				}},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "trainJob").
				UID("trainJob").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(2).
						Obj()).
				Obj(),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						MPIPolicy(ptr.To[int32](1), ptr.To(trainer.MPIImplementationOpenMPI), ptr.To("/root/.ssh"), nil).
						Obj(),
				},
				Trainer: runtime.Trainer{
					NumNodes:       ptr.To[int32](2),
					NumProcPerNode: "1",
					Env: []corev1ac.EnvVarApplyConfiguration{{
						Name:  ptr.To(constants.OpenMPIEnvHostFileLocation),
						Value: ptr.To(fmt.Sprintf("%s/%s", constants.MPIHostfileDir, constants.MPIHostfileName)),
					}},
					ContainerPort: &corev1ac.ContainerPortApplyConfiguration{
						ContainerPort: ptr.To[int32](constants.ContainerTrainerPort),
					},
					Volumes: []corev1ac.VolumeApplyConfiguration{
						*corev1ac.Volume().
							WithName(constants.MPISSHAuthVolumeName).
							WithSecret(corev1ac.SecretVolumeSource().
								WithSecretName(fmt.Sprintf("trainJob%s", constants.MPISSHAuthSecretSuffix)).
								WithItems(
									corev1ac.KeyToPath().WithKey(corev1.SSHAuthPrivateKey).WithPath(constants.MPISSHPrivateKeyFile),
									corev1ac.KeyToPath().WithKey(constants.MPISSHPublicKey).WithPath(constants.MPISSHPublicKeyFile),
									corev1ac.KeyToPath().WithKey(constants.MPISSHPublicKey).WithPath(constants.MPISSHAuthorizedKeys),
								),
							),
						*corev1ac.Volume().
							WithName(constants.MPIHostfileVolumeName).
							WithConfigMap(corev1ac.ConfigMapVolumeSource().
								WithName(fmt.Sprintf("trainJob-%s", constants.MPIHostfileVolumeName))),
					},
					VolumeMounts: []corev1ac.VolumeMountApplyConfiguration{
						*corev1ac.VolumeMount().WithName(constants.MPISSHAuthVolumeName).WithMountPath("/root/.ssh"),
						*corev1ac.VolumeMount().WithName(constants.MPIHostfileVolumeName).WithMountPath(constants.MPIHostfileDir),
					},
				},
				Scheduler: &runtime.Scheduler{TotalRequests: map[string]runtime.TotalResourceRequest{
					constants.JobTrainerNode: {
						Replicas: 2,
						PodRequests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("10"),
						},
					},
					constants.JobInitializer: {},
				}},
			},
			wantObjs: []apiruntime.Object{
				utiltesting.MakeSecretWrapper(fmt.Sprintf("trainJob%s", constants.MPISSHAuthSecretSuffix), metav1.NamespaceDefault).
					WithType(corev1.SecretTypeSSHAuth).
					WithData(map[string][]byte{
						constants.MPISSHPublicKey: []byte("EXIST"),
						corev1.SSHAuthPrivateKey:  []byte("EXIST"),
					}).
					OwnerReference(trainer.SchemeGroupVersion.WithKind("TrainJob"), "trainJob", "trainJob").
					Obj(),
				utiltesting.MakeConfigMapWrapper(fmt.Sprintf("trainJob%s", constants.MPIHostfileConfigMapSuffix), metav1.NamespaceDefault).
					WithData(map[string]string{
						constants.MPIHostfileName: `trainJob-trainer-node-0-0.1 slots=trainJob
trainJob-trainer-node-0-1.1 slots=trainJob
`,
					}).
					OwnerReference(trainer.SchemeGroupVersion.WithKind("TrainJob"), "trainJob", "trainJob").
					Obj(),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)
			cli := utiltesting.NewClientBuilder().Build()
			p, err := New(ctx, cli, nil)
			if err != nil {
				t.Fatalf("Failed to initialize MPI plugin: %v", err)
			}
			err = p.(framework.EnforceMLPolicyPlugin).EnforceMLPolicy(tc.info, tc.trainJob)
			if diff := cmp.Diff(tc.wantMLPolicyError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error from EnforceMLPolicy (-want, +got): %s", diff)
			}
			if diff := cmp.Diff(tc.wantInfo, tc.info,
				cmpopts.SortSlices(func(a, b string) bool { return a < b }),
				cmpopts.SortMaps(func(a, b int) bool { return a < b }),
			); len(diff) != 0 {
				t.Errorf("Unexpected info from EnforceMLPolicy (-want, +got): %s", diff)
			}
			var objs []any
			objs, err = p.(framework.ComponentBuilderPlugin).Build(ctx, tc.info, tc.trainJob)
			if diff := cmp.Diff(tc.wantBuildError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error from Build (-want, +got): %s", diff)
			}
			var typedObjs []apiruntime.Object
			typedObjs, err = utiltesting.ToObject(cli.Scheme(), objs...)
			if err != nil {
				t.Errorf("Failed to convert object: %v", err)
			}
			if diff := cmp.Diff(typedObjs, tc.wantObjs, objCmpOpts...); len(diff) != 0 {
				t.Errorf("Unexpected objects from Build (-want, +got): %s", diff)
			}
		})
	}
}

func secretDataComparer(a, b map[string][]byte) bool {
	isKeysEqual := true
	if (a != nil && b != nil) &&
		((len(a[constants.MPISSHPublicKey]) > 0) != (len(b[constants.MPISSHPublicKey]) > 0) ||
			(len(a[corev1.SSHAuthPrivateKey]) > 0) != (len(b[corev1.SSHAuthPrivateKey]) > 0)) {
		isKeysEqual = false
	}
	return isKeysEqual && cmp.Equal(a, b, cmpopts.IgnoreMapEntries(func(k string, _ []byte) bool {
		return k == constants.MPISSHPublicKey || k == corev1.SSHAuthPrivateKey
	}))
}
