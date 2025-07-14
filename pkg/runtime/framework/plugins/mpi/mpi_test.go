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
	"cmp"
	"context"
	"errors"
	"fmt"
	"testing"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestMPI(t *testing.T) {
	objCmpOpts := []gocmp.Option{
		cmpopts.SortSlices(func(a, b apiruntime.Object) int {
			return cmp.Compare(a.GetObjectKind().GroupVersionKind().String(), b.GetObjectKind().GroupVersionKind().String())
		}),
		cmpopts.SortSlices(func(a, b corev1.EnvVar) int { return cmp.Compare(a.Name, b.Name) }),
		gocmp.Comparer(utiltesting.MPISecretDataComparer),
	}
	errorGetSSHAuthSecretFromAPI := errors.New("failed to get SSH Auth Secret from API during Build")

	cases := map[string]struct {
		info              *runtime.Info
		trainJob          *trainer.TrainJob
		objs              []client.Object
		wantInfo          *runtime.Info
		wantObjs          []apiruntime.Object
		wantMLPolicyError error
		wantBuildError    error
	}{
		"no action when info is nil": {},
		"no action when mlPolicySource is nil": {
			info: &runtime.Info{
				Labels: map[string]string{"key": "value"},
			},
			wantInfo: &runtime.Info{
				Labels: map[string]string{"key": "value"},
			},
		},
		"no action when mlPolicySource mpi is null": {
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().Obj(),
				},
			},
			wantInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().Obj(),
				},
			},
		},
		"trainJob numNodes is respected rather than mlPolicy one": {
			info: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:  constants.Launcher,
							Count: ptr.To[int32](1),
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-launcher-0-0.trainJob")
							},
						},
						{
							Name:     constants.Node,
							Ancestor: ptr.To(constants.AncestorTrainer),
							Count:    ptr.To[int32](1),
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-node-1-0.trainJob")
								yield("trainJob-node-1-1.trainJob")
							},
						},
					},
				},
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						MPIPolicy(ptr.To[int32](1), ptr.To(trainer.MPIImplementationOpenMPI), ptr.To("/root/.ssh"), nil).
						Obj(),
				},
				Scheduler: &runtime.Scheduler{
					PodLabels: make(map[string]string),
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
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:  constants.Launcher,
							Count: ptr.To[int32](1),
							Volumes: []corev1ac.VolumeApplyConfiguration{
								*corev1ac.Volume().
									WithName(constants.MPISSHAuthVolumeName).
									WithSecret(corev1ac.SecretVolumeSource().
										WithSecretName(fmt.Sprintf("trainJob%s", constants.MPISSHAuthSecretSuffix)).
										WithItems(
											corev1ac.KeyToPath().
												WithKey(corev1.SSHAuthPrivateKey).
												WithPath(constants.MPISSHPrivateKeyFile),
											corev1ac.KeyToPath().
												WithKey(constants.MPISSHPublicKey).
												WithPath(constants.MPISSHPublicKeyFile),
											corev1ac.KeyToPath().
												WithKey(constants.MPISSHPublicKey).
												WithPath(constants.MPISSHAuthorizedKeys),
										),
									),
								*corev1ac.Volume().
									WithName(constants.MPIHostfileVolumeName).
									WithConfigMap(corev1ac.ConfigMapVolumeSource().
										WithName(fmt.Sprintf("trainJob%s", constants.MPIHostfileConfigMapSuffix)).
										WithItems(
											corev1ac.KeyToPath().
												WithKey(constants.MPIHostfileName).
												WithPath(constants.MPIHostfileName).
												WithMode(0444),
										),
									),
							},
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-launcher-0-0.trainJob")
							},
						},
						{
							Name:     constants.Node,
							Ancestor: ptr.To(constants.AncestorTrainer),
							Count:    ptr.To[int32](2),
							Volumes: []corev1ac.VolumeApplyConfiguration{
								*corev1ac.Volume().
									WithName(constants.MPISSHAuthVolumeName).
									WithSecret(corev1ac.SecretVolumeSource().
										WithSecretName(fmt.Sprintf("trainJob%s", constants.MPISSHAuthSecretSuffix)).
										WithItems(
											corev1ac.KeyToPath().
												WithKey(corev1.SSHAuthPrivateKey).
												WithPath(constants.MPISSHPrivateKeyFile),
											corev1ac.KeyToPath().
												WithKey(constants.MPISSHPublicKey).
												WithPath(constants.MPISSHPublicKeyFile),
											corev1ac.KeyToPath().
												WithKey(constants.MPISSHPublicKey).
												WithPath(constants.MPISSHAuthorizedKeys),
										),
									),
							},
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-node-1-0.trainJob")
								yield("trainJob-node-1-1.trainJob")
							},
						},
					},
				},
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						MPIPolicy(ptr.To[int32](1), ptr.To(trainer.MPIImplementationOpenMPI), ptr.To("/root/.ssh"), nil).
						Obj(),
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
			wantObjs: []apiruntime.Object{
				utiltesting.MakeSecretWrapper(fmt.Sprintf("trainJob%s", constants.MPISSHAuthSecretSuffix), metav1.NamespaceDefault).
					WithImmutable(true).
					WithType(corev1.SecretTypeSSHAuth).
					WithData(map[string][]byte{
						constants.MPISSHPublicKey: []byte("EXIST"),
						corev1.SSHAuthPrivateKey:  []byte("EXIST"),
					}).
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "trainJob", "trainJob").
					Obj(),
				utiltesting.MakeConfigMapWrapper(fmt.Sprintf("trainJob%s", constants.MPIHostfileConfigMapSuffix), metav1.NamespaceDefault).
					WithData(map[string]string{
						constants.MPIHostfileName: `trainJob-node-1-0.trainJob slots=1
trainJob-node-1-1.trainJob slots=1
`,
					}).
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "trainJob", "trainJob").
					Obj(),
			},
		},
		"trainJob numProcPerNode is respected rather than mlPolicy one": {
			info: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:  constants.Launcher,
							Count: ptr.To[int32](1),
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-launcher-0-0.trainJob")
							},
						},
						{
							Name:     constants.Node,
							Ancestor: ptr.To(constants.AncestorTrainer),
							Count:    ptr.To[int32](1),
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-node-1-0.trainJob")
							},
						},
					},
				},
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						MPIPolicy(ptr.To[int32](1), ptr.To(trainer.MPIImplementationOpenMPI), ptr.To("/root/.ssh"), nil).
						Obj(),
				},
				Scheduler: &runtime.Scheduler{
					PodLabels: make(map[string]string),
				},
			},
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
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:  constants.Launcher,
							Count: ptr.To[int32](1),
							Volumes: []corev1ac.VolumeApplyConfiguration{
								*corev1ac.Volume().
									WithName(constants.MPISSHAuthVolumeName).
									WithSecret(corev1ac.SecretVolumeSource().
										WithSecretName(fmt.Sprintf("trainJob%s", constants.MPISSHAuthSecretSuffix)).
										WithItems(
											corev1ac.KeyToPath().
												WithKey(corev1.SSHAuthPrivateKey).
												WithPath(constants.MPISSHPrivateKeyFile),
											corev1ac.KeyToPath().
												WithKey(constants.MPISSHPublicKey).
												WithPath(constants.MPISSHPublicKeyFile),
											corev1ac.KeyToPath().
												WithKey(constants.MPISSHPublicKey).
												WithPath(constants.MPISSHAuthorizedKeys),
										),
									),
								*corev1ac.Volume().
									WithName(constants.MPIHostfileVolumeName).
									WithConfigMap(corev1ac.ConfigMapVolumeSource().
										WithName(fmt.Sprintf("trainJob%s", constants.MPIHostfileConfigMapSuffix)).
										WithItems(
											corev1ac.KeyToPath().
												WithKey(constants.MPIHostfileName).
												WithPath(constants.MPIHostfileName).
												WithMode(0444),
										),
									),
							},
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-launcher-0-0.trainJob")
							},
						},
						{
							Name:     constants.Node,
							Ancestor: ptr.To(constants.AncestorTrainer),
							Count:    ptr.To[int32](1),
							Volumes: []corev1ac.VolumeApplyConfiguration{
								*corev1ac.Volume().
									WithName(constants.MPISSHAuthVolumeName).
									WithSecret(corev1ac.SecretVolumeSource().
										WithSecretName(fmt.Sprintf("trainJob%s", constants.MPISSHAuthSecretSuffix)).
										WithItems(
											corev1ac.KeyToPath().
												WithKey(corev1.SSHAuthPrivateKey).
												WithPath(constants.MPISSHPrivateKeyFile),
											corev1ac.KeyToPath().
												WithKey(constants.MPISSHPublicKey).
												WithPath(constants.MPISSHPublicKeyFile),
											corev1ac.KeyToPath().
												WithKey(constants.MPISSHPublicKey).
												WithPath(constants.MPISSHAuthorizedKeys),
										),
									),
							},
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-node-1-0.trainJob")
							},
						},
					},
				},
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						MPIPolicy(ptr.To[int32](2), ptr.To(trainer.MPIImplementationOpenMPI), ptr.To("/root/.ssh"), nil).
						Obj(),
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
			wantObjs: []apiruntime.Object{
				utiltesting.MakeSecretWrapper(fmt.Sprintf("trainJob%s", constants.MPISSHAuthSecretSuffix), metav1.NamespaceDefault).
					WithImmutable(true).
					WithType(corev1.SecretTypeSSHAuth).
					WithData(map[string][]byte{
						constants.MPISSHPublicKey: []byte("EXIST"),
						corev1.SSHAuthPrivateKey:  []byte("EXIST"),
					}).
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "trainJob", "trainJob").
					Obj(),
				utiltesting.MakeConfigMapWrapper(fmt.Sprintf("trainJob%s", constants.MPIHostfileConfigMapSuffix), metav1.NamespaceDefault).
					WithData(map[string]string{
						constants.MPIHostfileName: `trainJob-node-1-0.trainJob slots=2
`,
					}).
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "trainJob", "trainJob").
					Obj(),
			},
		},
		"runLauncherAsNode is true": {
			info: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						MPIPolicy(ptr.To[int32](1), ptr.To(trainer.MPIImplementationOpenMPI), ptr.To("/root/.ssh"), ptr.To(true)).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:  constants.Launcher,
							Count: ptr.To[int32](1),
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-launcher-0-0.trainJob")
							},
							Containers: []runtime.Container{{
								Name: constants.Node,
							}},
						},
						{
							Name:     constants.Node,
							Ancestor: ptr.To(constants.AncestorTrainer),
							Count:    ptr.To[int32](1),
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-node-1-0.trainJob")
							},
							Containers: []runtime.Container{{
								Name: constants.Node,
							}},
						},
					},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "trainJob").
				UID("trainJob").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(10).
						Obj()).
				Obj(),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						MPIPolicy(ptr.To[int32](1), ptr.To(trainer.MPIImplementationOpenMPI), ptr.To("/root/.ssh"), ptr.To(true)).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:  constants.Launcher,
							Count: ptr.To[int32](1),
							Containers: []runtime.Container{{
								Name: constants.Node,
								VolumeMounts: []corev1ac.VolumeMountApplyConfiguration{
									*corev1ac.VolumeMount().
										WithName(constants.MPISSHAuthVolumeName).
										WithMountPath("/root/.ssh"),
									*corev1ac.VolumeMount().
										WithName(constants.MPIHostfileVolumeName).
										WithMountPath("/etc/mpi"),
								},
								Env: []corev1ac.EnvVarApplyConfiguration{
									*corev1ac.EnvVar().
										WithName(constants.OpenMPIEnvHostFileLocation).
										WithValue(fmt.Sprintf("%s/%s", constants.MPIHostfileDir, constants.MPIHostfileName)),
									*corev1ac.EnvVar().
										WithName(constants.OpenMPIEnvKeepFQDNHostNames).
										WithValue("true"),
									*corev1ac.EnvVar().
										WithName(constants.OpenMPIEnvDefaultSlots).
										WithValue("1"),
									*corev1ac.EnvVar().
										WithName(constants.OpenMPIEnvKeyRSHArgs).
										WithValue(constants.OpenMPIEnvDefaultValueRSHArgs),
								},
							}},
							Volumes: []corev1ac.VolumeApplyConfiguration{
								*corev1ac.Volume().
									WithName(constants.MPISSHAuthVolumeName).
									WithSecret(corev1ac.SecretVolumeSource().
										WithSecretName(fmt.Sprintf("trainJob%s", constants.MPISSHAuthSecretSuffix)).
										WithItems(
											corev1ac.KeyToPath().
												WithKey(corev1.SSHAuthPrivateKey).
												WithPath(constants.MPISSHPrivateKeyFile),
											corev1ac.KeyToPath().
												WithKey(constants.MPISSHPublicKey).
												WithPath(constants.MPISSHPublicKeyFile),
											corev1ac.KeyToPath().
												WithKey(constants.MPISSHPublicKey).
												WithPath(constants.MPISSHAuthorizedKeys),
										),
									),
								*corev1ac.Volume().
									WithName(constants.MPIHostfileVolumeName).
									WithConfigMap(corev1ac.ConfigMapVolumeSource().
										WithName(fmt.Sprintf("trainJob%s", constants.MPIHostfileConfigMapSuffix)).
										WithItems(
											corev1ac.KeyToPath().
												WithKey(constants.MPIHostfileName).
												WithPath(constants.MPIHostfileName).
												WithMode(0444),
										),
									),
							},
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-launcher-0-0.trainJob")
							},
						},
						{
							Name:     constants.Node,
							Ancestor: ptr.To(constants.AncestorTrainer),
							Count:    ptr.To[int32](9),
							Containers: []runtime.Container{{
								Name: constants.Node,
								VolumeMounts: []corev1ac.VolumeMountApplyConfiguration{
									*corev1ac.VolumeMount().
										WithName(constants.MPISSHAuthVolumeName).
										WithMountPath("/root/.ssh"),
								},
							}},
							Volumes: []corev1ac.VolumeApplyConfiguration{
								*corev1ac.Volume().
									WithName(constants.MPISSHAuthVolumeName).
									WithSecret(corev1ac.SecretVolumeSource().
										WithSecretName(fmt.Sprintf("trainJob%s", constants.MPISSHAuthSecretSuffix)).
										WithItems(
											corev1ac.KeyToPath().
												WithKey(corev1.SSHAuthPrivateKey).
												WithPath(constants.MPISSHPrivateKeyFile),
											corev1ac.KeyToPath().
												WithKey(constants.MPISSHPublicKey).
												WithPath(constants.MPISSHPublicKeyFile),
											corev1ac.KeyToPath().
												WithKey(constants.MPISSHPublicKey).
												WithPath(constants.MPISSHAuthorizedKeys),
										),
									),
							},
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-node-1-0.trainJob")
							},
						},
					},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
			wantObjs: []apiruntime.Object{
				utiltesting.MakeSecretWrapper(fmt.Sprintf("trainJob%s", constants.MPISSHAuthSecretSuffix), metav1.NamespaceDefault).
					WithImmutable(true).
					WithType(corev1.SecretTypeSSHAuth).
					WithData(map[string][]byte{
						constants.MPISSHPublicKey: []byte("EXIST"),
						corev1.SSHAuthPrivateKey:  []byte("EXIST"),
					}).
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "trainJob", "trainJob").
					Obj(),
				utiltesting.MakeConfigMapWrapper(fmt.Sprintf("trainJob%s", constants.MPIHostfileConfigMapSuffix), metav1.NamespaceDefault).
					WithData(map[string]string{
						constants.MPIHostfileName: `trainJob-launcher-0-0.trainJob slots=1
trainJob-node-1-0.trainJob slots=1
`,
					}).
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "trainJob", "trainJob").
					Obj(),
			},
		},
		"sshAuth secret already has existed in the cluster": {
			objs: []client.Object{
				utiltesting.MakeSecretWrapper(sshAuthSecretName("trainJob"), metav1.NamespaceDefault).
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "trainJob", "trainJob").
					WithImmutable(true).
					Obj(),
			},
			info: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name: constants.Launcher,
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-launcher-0-0.trainJob")
							},
						},
					},
				},
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						MPIPolicy(ptr.To[int32](1), ptr.To(trainer.MPIImplementationOpenMPI), ptr.To("/root/.ssh"), ptr.To(true)).
						Obj(),
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "trainJob").
				UID("trainJob").
				Obj(),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name: constants.Launcher,
							Volumes: []corev1ac.VolumeApplyConfiguration{
								*corev1ac.Volume().
									WithName(constants.MPISSHAuthVolumeName).
									WithSecret(corev1ac.SecretVolumeSource().
										WithSecretName(fmt.Sprintf("trainJob%s", constants.MPISSHAuthSecretSuffix)).
										WithItems(
											corev1ac.KeyToPath().
												WithKey(corev1.SSHAuthPrivateKey).
												WithPath(constants.MPISSHPrivateKeyFile),
											corev1ac.KeyToPath().
												WithKey(constants.MPISSHPublicKey).
												WithPath(constants.MPISSHPublicKeyFile),
											corev1ac.KeyToPath().
												WithKey(constants.MPISSHPublicKey).
												WithPath(constants.MPISSHAuthorizedKeys),
										),
									),
								*corev1ac.Volume().
									WithName(constants.MPIHostfileVolumeName).
									WithConfigMap(corev1ac.ConfigMapVolumeSource().
										WithName(fmt.Sprintf("trainJob%s", constants.MPIHostfileConfigMapSuffix)).
										WithItems(
											corev1ac.KeyToPath().
												WithKey(constants.MPIHostfileName).
												WithPath(constants.MPIHostfileName).
												WithMode(0444),
										),
									),
							},
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-launcher-0-0.trainJob")
							},
						},
					},
				},
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						MPIPolicy(ptr.To[int32](1), ptr.To(trainer.MPIImplementationOpenMPI), ptr.To("/root/.ssh"), ptr.To(true)).
						Obj(),
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
			wantObjs: []apiruntime.Object{
				utiltesting.MakeConfigMapWrapper(fmt.Sprintf("trainJob%s", constants.MPIHostfileConfigMapSuffix), metav1.NamespaceDefault).
					WithData(map[string]string{
						constants.MPIHostfileName: `trainJob-launcher-0-0.trainJob slots=1
`,
					}).
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "trainJob", "trainJob").
					Obj(),
			},
		},
		"failed to get sshAuth secret due to API error": {
			info: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name: constants.Launcher,
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-launcher-0-0.trainJob")
							},
						},
					},
				},
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						MPIPolicy(ptr.To[int32](1), ptr.To(trainer.MPIImplementationOpenMPI), ptr.To("/root/.ssh"), ptr.To(true)).
						Obj(),
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "trainJob").
				UID("trainJob").
				Obj(),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name: constants.Launcher,
							Volumes: []corev1ac.VolumeApplyConfiguration{
								*corev1ac.Volume().
									WithName(constants.MPISSHAuthVolumeName).
									WithSecret(corev1ac.SecretVolumeSource().
										WithSecretName(fmt.Sprintf("trainJob%s", constants.MPISSHAuthSecretSuffix)).
										WithItems(
											corev1ac.KeyToPath().
												WithKey(corev1.SSHAuthPrivateKey).
												WithPath(constants.MPISSHPrivateKeyFile),
											corev1ac.KeyToPath().
												WithKey(constants.MPISSHPublicKey).
												WithPath(constants.MPISSHPublicKeyFile),
											corev1ac.KeyToPath().
												WithKey(constants.MPISSHPublicKey).
												WithPath(constants.MPISSHAuthorizedKeys),
										),
									),
								*corev1ac.Volume().
									WithName(constants.MPIHostfileVolumeName).
									WithConfigMap(corev1ac.ConfigMapVolumeSource().
										WithName(fmt.Sprintf("trainJob%s", constants.MPIHostfileConfigMapSuffix)).
										WithItems(
											corev1ac.KeyToPath().
												WithKey(constants.MPIHostfileName).
												WithPath(constants.MPIHostfileName).
												WithMode(0444),
										),
									),
							},
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-launcher-0-0.trainJob")
							},
						},
					},
				},
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						MPIPolicy(ptr.To[int32](1), ptr.To(trainer.MPIImplementationOpenMPI), ptr.To("/root/.ssh"), ptr.To(true)).
						Obj(),
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
			wantBuildError: errorGetSSHAuthSecretFromAPI,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)
			b := utiltesting.NewClientBuilder().WithObjects(tc.objs...)
			b.WithInterceptorFuncs(interceptor.Funcs{
				Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if _, ok := obj.(*corev1.Secret); ok && errors.Is(tc.wantBuildError, errorGetSSHAuthSecretFromAPI) {
						return errorGetSSHAuthSecretFromAPI
					}
					return client.Get(ctx, key, obj, opts...)
				},
			})
			cli := b.Build()

			p, err := New(ctx, cli, nil)
			if err != nil {
				t.Fatalf("Failed to initialize MPI plugin: %v", err)
			}
			err = p.(framework.EnforceMLPolicyPlugin).EnforceMLPolicy(tc.info, tc.trainJob)
			if diff := gocmp.Diff(tc.wantMLPolicyError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error from EnforceMLPolicy (-want, +got): %s", diff)
			}
			if diff := gocmp.Diff(tc.wantInfo, tc.info,
				cmpopts.SortSlices(func(a, b string) bool { return a < b }),
				cmpopts.SortMaps(func(a, b int) bool { return a < b }),
				utiltesting.PodSetEndpointsCmpOpts,
			); len(diff) != 0 {
				t.Errorf("Unexpected info from EnforceMLPolicy (-want, +got): %s", diff)
			}
			var objs []any
			objs, err = p.(framework.ComponentBuilderPlugin).Build(ctx, tc.info, tc.trainJob)
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

func TestValidate(t *testing.T) {
	cases := map[string]struct {
		info         *runtime.Info
		oldObj       *trainer.TrainJob
		newObj       *trainer.TrainJob
		wantError    field.ErrorList
		wantWarnings admission.Warnings
	}{
		"no action when info is nil ": {},
		"no action when info does not have MLPolicySource": {
			info: runtime.NewInfo(),
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Obj(),
		},
		"info does not have MPIPolicySource": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(utiltesting.MakeMLPolicyWrapper().Obj()),
			),
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Obj(),
		},
		"numProcPerNode typed is string": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(utiltesting.MakeMLPolicyWrapper().
					WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
						MPIPolicy(ptr.To[int32](1), ptr.To(trainer.MPIImplementationOpenMPI), nil, nil).
						Obj(),
					).
					Obj()),
			),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().
					NumProcPerNode(intstr.FromString("invalid")).
					Obj(),
				).
				Obj(),
			wantError: field.ErrorList{
				field.Invalid(
					field.NewPath("spec").Child("trainer").Child("numProcPerNode"),
					intstr.FromString("invalid"),
					"must have an int value for MPI TrainJob",
				),
			},
		},
		"numProcPerNode typed is int": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(utiltesting.MakeMLPolicyWrapper().
					WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
						MPIPolicy(ptr.To[int32](1), ptr.To(trainer.MPIImplementationOpenMPI), nil, nil).
						Obj(),
					).
					Obj(),
				),
			),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().
					NumProcPerNode(intstr.FromInt32(8)).
					Obj(),
				).
				Obj(),
		},
		"runtime does not have Launcher but TrainJob has only 1 numNodes": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(utiltesting.MakeMLPolicyWrapper().
					WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
						MPIPolicy(ptr.To[int32](1), ptr.To(trainer.MPIImplementationOpenMPI), nil, ptr.To(true)).
						Obj(),
					).
					Obj(),
				),
				runtime.WithPodSet(constants.Node, ptr.To(constants.AncestorTrainer), 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").Trainer(&trainer.Trainer{NumNodes: ptr.To(int32(1))}).Obj(),
		},
		"runtime does not have Launcher even though MPI with runLauncherAsNode has 2 numNodes": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(utiltesting.MakeMLPolicyWrapper().
					WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
						MPIPolicy(ptr.To[int32](1), ptr.To(trainer.MPIImplementationOpenMPI), nil, ptr.To(true)).
						Obj(),
					).
					Obj(),
				),
				runtime.WithPodSet(constants.Node, nil, 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").Trainer(&trainer.Trainer{NumNodes: ptr.To(int32(2))}).Obj(),
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("spec").Child("trainer", "numNodes"), ptr.To(int32(2)), "must have 1 when MPI trainingRuntime with enabled runLauncherAsNode does not have either launcher and node"),
			},
		},
		"runtime does not have Node even though MPI with runLauncherAsNode has 2 numNodes": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(utiltesting.MakeMLPolicyWrapper().
					WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
						MPIPolicy(ptr.To[int32](1), ptr.To(trainer.MPIImplementationOpenMPI), nil, ptr.To(true)).
						Obj(),
					).
					Obj(),
				),
				runtime.WithPodSet(constants.Launcher, nil, 1, corev1.PodSpec{}, corev1ac.PodSpec().
					WithContainers(corev1ac.Container().WithName(constants.Node)),
				),
			),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").Trainer(&trainer.Trainer{NumNodes: ptr.To(int32(2))}).Obj(),
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("spec").Child("trainer", "numNodes"), ptr.To(int32(2)), "must have 1 when MPI trainingRuntime with enabled runLauncherAsNode does not have either launcher and node"),
			},
		},
		"runtime does not have Launcher and Node even though MPI with runLauncherAsNode has 2 numNodes": {
			info: runtime.NewInfo(
				runtime.WithMLPolicySource(utiltesting.MakeMLPolicyWrapper().
					WithMLPolicySource(*utiltesting.MakeMLPolicySourceWrapper().
						MPIPolicy(ptr.To[int32](1), ptr.To(trainer.MPIImplementationOpenMPI), nil, ptr.To(true)).
						Obj(),
					).
					Obj(),
				),
			),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").Trainer(&trainer.Trainer{NumNodes: ptr.To(int32(2))}).Obj(),
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("spec").Child("trainer", "numNodes"), ptr.To(int32(2)), "must have 1 when MPI trainingRuntime with enabled runLauncherAsNode does not have either launcher and node"),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)
			p, err := New(ctx, utiltesting.NewClientBuilder().Build(), nil)
			if err != nil {
				t.Fatalf("Failed to initialize MPI plugin: %v", err)
			}
			warnings, errs := p.(framework.CustomValidationPlugin).Validate(ctx, tc.info, tc.oldObj, tc.newObj)
			if diff := gocmp.Diff(tc.wantError, errs); len(diff) != 0 {
				t.Errorf("Unexpected error from Validate (-want, +got): %s", diff)
			}
			if diff := gocmp.Diff(tc.wantWarnings, warnings); len(diff) != 0 {
				t.Errorf("Unexpected warnings from Validate (-want, +got): %s", diff)
			}
		})
	}
}
