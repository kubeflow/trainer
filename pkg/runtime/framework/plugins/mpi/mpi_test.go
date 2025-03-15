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
	"github.com/kubeflow/trainer/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/runtime"
	"github.com/kubeflow/trainer/pkg/runtime/framework"
	utiltesting "github.com/kubeflow/trainer/pkg/util/testing"
)

func TestMPI(t *testing.T) {
	objCmpOpts := []gocmp.Option{
		cmpopts.SortSlices(func(a, b apiruntime.Object) int {
			return cmp.Compare(a.GetObjectKind().GroupVersionKind().String(), b.GetObjectKind().GroupVersionKind().String())
		}),
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
		"no action when mlPolicy is nil": {
			info: &runtime.Info{
				Labels: map[string]string{"key": "value"},
			},
			wantInfo: &runtime.Info{
				Labels: map[string]string{"key": "value"},
			},
		},
		"no action when mlPolicy mpi is null": {
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().Obj(),
				},
			},
			wantInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().Obj(),
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
							Name: constants.JobLauncher,
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-launcher-0-0.trainJob")
							},
						},
						{
							Name: constants.JobTrainerNode,
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-trainer-node-1-0.trainJob")
							},
						},
						{
							Name: constants.JobTrainerNode,
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-trainer-node-1-1.trainJob")
							},
						},
					},
				},
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
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
							Name: constants.JobLauncher,
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
							Name: constants.JobTrainerNode,
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
								yield("trainJob-trainer-node-1-0.trainJob")
							},
						},
						{
							Name: constants.JobTrainerNode,
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
								yield("trainJob-trainer-node-1-1.trainJob")
							},
						},
					},
				},
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(2).
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
						constants.MPIHostfileName: `trainJob-trainer-node-1-0.trainJob slots=1
trainJob-trainer-node-1-1.trainJob slots=1
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
							Name: constants.JobLauncher,
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-launcher-0-0.trainJob")
							},
						},
						{
							Name: constants.JobTrainerNode,
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-trainer-node-1-0.trainJob")
							},
						},
					},
				},
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
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
							Name: constants.JobLauncher,
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
							Name: constants.JobTrainerNode,
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
								yield("trainJob-trainer-node-1-0.trainJob")
							},
						},
					},
				},
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
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
						constants.MPIHostfileName: `trainJob-trainer-node-1-0.trainJob slots=2
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
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						MPIPolicy(ptr.To[int32](1), ptr.To(trainer.MPIImplementationOpenMPI), ptr.To("/root/.ssh"), ptr.To(true)).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name: constants.JobLauncher,
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-launcher-0-0.trainJob")
							},
						},
						{
							Name: constants.JobTrainerNode,
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-trainer-node-1-0.trainJob")
							},
						},
					},
				},
				Scheduler: &runtime.Scheduler{PodLabels: make(map[string]string)},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "trainJob").
				UID("trainJob").
				Trainer(
					utiltesting.MakeTrainJobTrainerWrapper().
						NumNodes(1).
						Obj()).
				Obj(),
			wantInfo: &runtime.Info{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
						MPIPolicy(ptr.To[int32](1), ptr.To(trainer.MPIImplementationOpenMPI), ptr.To("/root/.ssh"), ptr.To(true)).
						Obj(),
				},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name: constants.JobLauncher,
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
							Name: constants.JobTrainerNode,
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
								yield("trainJob-trainer-node-1-0.trainJob")
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
trainJob-trainer-node-1-0.trainJob slots=1
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
							Name: constants.JobLauncher,
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-launcher-0-0.trainJob")
							},
						},
					},
				},
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
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
							Name: constants.JobLauncher,
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
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
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
							Name: constants.JobLauncher,
							Endpoints: func(yield func(string) bool) {
								yield("trainJob-launcher-0-0.trainJob")
							},
						},
					},
				},
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
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
							Name: constants.JobLauncher,
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
					MLPolicy: utiltesting.MakeMLPolicyWrapper().
						WithNumNodes(1).
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
