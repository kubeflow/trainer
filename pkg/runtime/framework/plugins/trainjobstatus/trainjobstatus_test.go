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

package trainjobstatus

import (
	"cmp"
	"context"
	"fmt"
	"testing"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestEnforcePodSpec(t *testing.T) {
	cases := map[string]struct {
		info      *runtime.Info
		trainJob  *trainer.TrainJob
		wantInfo  *runtime.Info
		wantError error
	}{
		"does nothing if no trainer pods": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:  "launcher",
							Count: ptr.To[int32](1),
							Containers: []runtime.Container{
								{Name: "launcher"},
							},
						},
					},
				},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().NumNodes(1).Obj()).
				Obj(),
			wantInfo: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:  "launcher",
							Count: ptr.To[int32](1),
							Containers: []runtime.Container{
								{Name: "launcher"},
							},
						},
					},
				},
			},
		},
		"injects runtime configuration into trainer containers": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:     "trainer",
							Ancestor: ptr.To(constants.AncestorTrainer),
							Count:    ptr.To[int32](2),
							Containers: []runtime.Container{
								{Name: constants.Node},
							},
						},
					},
				},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("test-uid").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().NumNodes(2).Obj()).
				Obj(),
			wantInfo: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:     "trainer",
							Ancestor: ptr.To(constants.AncestorTrainer),
							Count:    ptr.To[int32](2),
							Containers: []runtime.Container{
								{
									Name: constants.Node,
									Env: []corev1ac.EnvVarApplyConfiguration{
										*corev1ac.EnvVar().
											WithName(envNameStatusURL).
											WithValue("https://kubeflow-trainer-controller-manager.kubeflow-system.svc:10443/apis/trainer.kubeflow.org/v1alpha1/namespaces/default/trainjobs/test-job/status"),
										*corev1ac.EnvVar().
											WithName(envNameCACert).
											WithValue(fmt.Sprintf("%s/%s", configMountPath, caCertFileName)),
										*corev1ac.EnvVar().
											WithName(envNameToken).
											WithValue(fmt.Sprintf("%s/%s", configMountPath, tokenFileName)),
									},
									VolumeMounts: []corev1ac.VolumeMountApplyConfiguration{
										*corev1ac.VolumeMount().
											WithName(tokenVolumeName).
											WithMountPath(configMountPath).
											WithReadOnly(true),
									},
								},
							},
							Volumes: []corev1ac.VolumeApplyConfiguration{
								*corev1ac.Volume().
									WithName(tokenVolumeName).
									WithProjected(
										corev1ac.ProjectedVolumeSource().
											WithSources(
												corev1ac.VolumeProjection().
													WithServiceAccountToken(
														corev1ac.ServiceAccountTokenProjection().
															WithAudience("trainer.kubeflow.org/v1alpha1/namespaces/default/trainjobs/test-job/status").
															WithExpirationSeconds(tokenExpirySeconds).
															WithPath(tokenFileName),
													),
												corev1ac.VolumeProjection().
													WithConfigMap(
														corev1ac.ConfigMapProjection().
															WithName("test-job-tls-config").
															WithItems(
																corev1ac.KeyToPath().
																	WithKey(caCertKey).
																	WithPath(caCertFileName),
															),
													),
											),
									),
							},
						},
					},
				},
			},
		},
		"injects runtime configuration into multiple containers": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:     "trainer",
							Ancestor: ptr.To(constants.AncestorTrainer),
							Count:    ptr.To[int32](1),
							Containers: []runtime.Container{
								{Name: constants.Node},
								{Name: "sidecar"},
							},
						},
					},
				},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("test-uid").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().NumNodes(1).Obj()).
				Obj(),
			wantInfo: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:     "trainer",
							Ancestor: ptr.To(constants.AncestorTrainer),
							Count:    ptr.To[int32](1),
							Containers: []runtime.Container{
								{
									Name: constants.Node,
									Env: []corev1ac.EnvVarApplyConfiguration{
										*corev1ac.EnvVar().
											WithName(envNameStatusURL).
											WithValue("https://kubeflow-trainer-controller-manager.kubeflow-system.svc:10443/apis/trainer.kubeflow.org/v1alpha1/namespaces/default/trainjobs/test-job/status"),
										*corev1ac.EnvVar().
											WithName(envNameCACert).
											WithValue(fmt.Sprintf("%s/%s", configMountPath, caCertFileName)),
										*corev1ac.EnvVar().
											WithName(envNameToken).
											WithValue(fmt.Sprintf("%s/%s", configMountPath, tokenFileName)),
									},
									VolumeMounts: []corev1ac.VolumeMountApplyConfiguration{
										*corev1ac.VolumeMount().
											WithName(tokenVolumeName).
											WithMountPath(configMountPath).
											WithReadOnly(true),
									},
								},
								{
									Name: "sidecar",
									Env: []corev1ac.EnvVarApplyConfiguration{
										*corev1ac.EnvVar().
											WithName(envNameStatusURL).
											WithValue("https://kubeflow-trainer-controller-manager.kubeflow-system.svc:10443/apis/trainer.kubeflow.org/v1alpha1/namespaces/default/trainjobs/test-job/status"),
										*corev1ac.EnvVar().
											WithName(envNameCACert).
											WithValue(fmt.Sprintf("%s/%s", configMountPath, caCertFileName)),
										*corev1ac.EnvVar().
											WithName(envNameToken).
											WithValue(fmt.Sprintf("%s/%s", configMountPath, tokenFileName)),
									},
									VolumeMounts: []corev1ac.VolumeMountApplyConfiguration{
										*corev1ac.VolumeMount().
											WithName(tokenVolumeName).
											WithMountPath(configMountPath).
											WithReadOnly(true),
									},
								},
							},
							Volumes: []corev1ac.VolumeApplyConfiguration{
								*corev1ac.Volume().
									WithName(tokenVolumeName).
									WithProjected(
										corev1ac.ProjectedVolumeSource().
											WithSources(
												corev1ac.VolumeProjection().
													WithServiceAccountToken(
														corev1ac.ServiceAccountTokenProjection().
															WithAudience("trainer.kubeflow.org/v1alpha1/namespaces/default/trainjobs/test-job/status").
															WithExpirationSeconds(tokenExpirySeconds).
															WithPath(tokenFileName),
													),
												corev1ac.VolumeProjection().
													WithConfigMap(
														corev1ac.ConfigMapProjection().
															WithName("test-job-tls-config").
															WithItems(
																corev1ac.KeyToPath().
																	WithKey(caCertKey).
																	WithPath(caCertFileName),
															),
													),
											),
									),
							},
						},
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

			cli := utiltesting.NewClientBuilder().Build()
			cfg := &configapi.Configuration{
				CertManagement: &configapi.CertManagement{
					WebhookServiceName: "kubeflow-trainer-controller-manager",
					WebhookSecretName:  "kubeflow-trainer-webhook-cert",
				},
				StatusServer: &configapi.StatusServer{
					Port:  ptr.To[int32](10443),
					QPS:   ptr.To[float32](5),
					Burst: ptr.To[int32](10),
				},
			}

			p, err := New(ctx, cli, nil, cfg)
			if err != nil {
				t.Fatalf("Failed to initialize Status plugin: %v", err)
			}

			err = p.(framework.EnforcePodSpecPlugin).EnforcePodSpec(tc.info.TemplateSpec.PodSets, tc.trainJob)
			if diff := gocmp.Diff(tc.wantError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error from EnforcePodSpec (-want, +got): %s", diff)
			}

			if diff := gocmp.Diff(tc.wantInfo, tc.info,
				cmpopts.SortSlices(func(a, b string) bool { return a < b }),
				cmpopts.SortMaps(func(a, b int) bool { return a < b }),
			); len(diff) != 0 {
				t.Errorf("Unexpected info from EnforcePodSpec (-want, +got): %s", diff)
			}
		})
	}
}

func TestBuild(t *testing.T) {
	objCmpOpts := []gocmp.Option{
		cmpopts.SortSlices(func(a, b apiruntime.Object) int {
			return cmp.Compare(a.GetObjectKind().GroupVersionKind().String(), b.GetObjectKind().GroupVersionKind().String())
		}),
	}

	cases := map[string]struct {
		info      *runtime.Info
		trainJob  *trainer.TrainJob
		objs      []client.Object
		wantObjs  []apiruntime.Object
		wantError string
	}{
		"no action when info is nil": {
			info:     nil,
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").Obj(),
			wantObjs: nil,
		},
		"no action when trainJob is nil": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:     "trainer",
							Ancestor: ptr.To(constants.AncestorTrainer),
							Count:    ptr.To[int32](1),
							Containers: []runtime.Container{
								{Name: constants.Node},
							},
						},
					},
				},
			},
			trainJob: nil,
			wantObjs: nil,
		},
		"creates ConfigMap with CA cert from secret": {
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kubeflow-trainer-webhook-cert",
						Namespace: "kubeflow-system", // Webhook secret is in operator namespace
					},
					Data: map[string][]byte{
						caCertKey: []byte("test-ca-cert-data"),
					},
				},
			},
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:     "trainer",
							Ancestor: ptr.To(constants.AncestorTrainer),
							Count:    ptr.To[int32](2),
							Containers: []runtime.Container{
								{Name: constants.Node},
							},
						},
					},
				},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("test-uid").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().NumNodes(2).Obj()).
				Obj(),
			wantObjs: []apiruntime.Object{
				&corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "ConfigMap",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-tls-config",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         trainer.GroupVersion.String(),
								Kind:               trainer.TrainJobKind,
								Name:               "test-job",
								UID:                "test-uid",
								Controller:         ptr.To(true),
								BlockOwnerDeletion: ptr.To(true),
							},
						},
					},
					Data: map[string]string{
						caCertKey: "test-ca-cert-data",
					},
				},
			},
		},
		"returns error when webhook secret not found": {
			objs: []client.Object{},
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:     "trainer",
							Ancestor: ptr.To(constants.AncestorTrainer),
							Count:    ptr.To[int32](1),
							Containers: []runtime.Container{
								{Name: constants.Node},
							},
						},
					},
				},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("test-uid").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().NumNodes(1).Obj()).
				Obj(),
			wantError: "failed to look up status server tls secret",
		},
		"returns error when CA cert is missing in secret": {
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kubeflow-trainer-webhook-cert",
						Namespace: "kubeflow-system",
					},
					Data: map[string][]byte{
						// No ca.crt key
						"other-key": []byte("other-data"),
					},
				},
			},
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:     "trainer",
							Ancestor: ptr.To(constants.AncestorTrainer),
							Count:    ptr.To[int32](1),
							Containers: []runtime.Container{
								{Name: constants.Node},
							},
						},
					},
				},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("test-uid").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().NumNodes(1).Obj()).
				Obj(),
			wantError: "failed to find status server ca.crt in tls secret",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)

			b := utiltesting.NewClientBuilder().WithObjects(tc.objs...)
			cli := b.Build()

			cfg := &configapi.Configuration{
				CertManagement: &configapi.CertManagement{
					WebhookServiceName: "kubeflow-trainer-controller-manager",
					WebhookSecretName:  "kubeflow-trainer-webhook-cert",
				},
				StatusServer: &configapi.StatusServer{
					Port:  ptr.To[int32](10443),
					QPS:   ptr.To[float32](5),
					Burst: ptr.To[int32](10),
				},
			}

			p, err := New(ctx, cli, nil, cfg)
			if err != nil {
				t.Fatalf("Failed to initialize Status plugin: %v", err)
			}

			var objs []apiruntime.ApplyConfiguration
			objs, err = p.(framework.ComponentBuilderPlugin).Build(ctx, tc.info, tc.trainJob)

			if tc.wantError != "" {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tc.wantError)
				} else if len(err.Error()) < len(tc.wantError) || err.Error()[:len(tc.wantError)] != tc.wantError {
					t.Errorf("Expected error containing %q, got %q", tc.wantError, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error from Build: %v", err)
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
		info     *runtime.Info
		trainJob *trainer.TrainJob
		wantErrs field.ErrorList
	}{
		"no error when info is nil": {
			info:     nil,
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").Obj(),
		},
		"no error when trainer podSet is not found": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:  "launcher",
							Count: ptr.To[int32](1),
							Containers: []runtime.Container{
								{Name: "launcher"},
							},
						},
					},
				},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").Obj(),
		},
		"no error when no conflict exists": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:     "trainer",
							Ancestor: ptr.To(constants.AncestorTrainer),
							Count:    ptr.To[int32](1),
							Containers: []runtime.Container{
								{
									Name: constants.Node,
									VolumeMounts: []corev1ac.VolumeMountApplyConfiguration{
										*corev1ac.VolumeMount().WithName("data").WithMountPath("/data"),
									},
								},
							},
							Volumes: []corev1ac.VolumeApplyConfiguration{
								*corev1ac.Volume().WithName("data"),
							},
						},
					},
				},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").Obj(),
		},
		"no error with multiple containers without conflicts": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:     "trainer",
							Ancestor: ptr.To(constants.AncestorTrainer),
							Count:    ptr.To[int32](1),
							Containers: []runtime.Container{
								{
									Name: constants.Node,
									VolumeMounts: []corev1ac.VolumeMountApplyConfiguration{
										*corev1ac.VolumeMount().WithName("data").WithMountPath("/data"),
									},
								},
								{
									Name: "sidecar",
									VolumeMounts: []corev1ac.VolumeMountApplyConfiguration{
										*corev1ac.VolumeMount().WithName("logs").WithMountPath("/var/log"),
									},
								},
							},
							Volumes: []corev1ac.VolumeApplyConfiguration{
								*corev1ac.Volume().WithName("data"),
								*corev1ac.Volume().WithName("logs"),
							},
						},
					},
				},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").Obj(),
		},
		"error when volume named kubeflow-trainer-token exists": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:     "trainer",
							Ancestor: ptr.To(constants.AncestorTrainer),
							Count:    ptr.To[int32](1),
							Containers: []runtime.Container{
								{Name: constants.Node},
							},
							Volumes: []corev1ac.VolumeApplyConfiguration{
								*corev1ac.Volume().WithName(tokenVolumeName),
							},
						},
					},
				},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").Obj(),
			wantErrs: field.ErrorList{
				field.Forbidden(
					field.NewPath("spec", "template", "spec", "volumes"),
					fmt.Sprintf("volume name %q in podSet %q is reserved for %s", tokenVolumeName, "trainer", Name),
				),
			},
		},
		"error when mount path is reserved with another mount name": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:     "trainer",
							Ancestor: ptr.To(constants.AncestorTrainer),
							Count:    ptr.To[int32](1),
							Containers: []runtime.Container{
								{
									Name: constants.Node,
									VolumeMounts: []corev1ac.VolumeMountApplyConfiguration{
										*corev1ac.VolumeMount().WithName("custom-token-volume").WithMountPath(configMountPath),
									},
								},
							},
						},
					},
				},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").Obj(),
			wantErrs: field.ErrorList{
				field.Forbidden(
					field.NewPath("spec", "template", "spec", "containers").Index(0).Child("volumeMounts"),
					fmt.Sprintf("volume mount path %q in container %q of podSet %q is reserved for %s", configMountPath, constants.Node, "trainer", Name),
				),
			},
		},
		"error when mount named kubeflow-trainer-token at another path": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:     "trainer",
							Ancestor: ptr.To(constants.AncestorTrainer),
							Count:    ptr.To[int32](1),
							Containers: []runtime.Container{
								{
									Name: constants.Node,
									VolumeMounts: []corev1ac.VolumeMountApplyConfiguration{
										*corev1ac.VolumeMount().WithName(tokenVolumeName).WithMountPath("/other/path"),
									},
								},
							},
						},
					},
				},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").Obj(),
			wantErrs: field.ErrorList{
				field.Forbidden(
					field.NewPath("spec", "template", "spec", "containers").Index(0).Child("volumeMounts"),
					fmt.Sprintf("volume mount name %q in container %q of podSet %q is reserved for %s", tokenVolumeName, constants.Node, "trainer", Name),
				),
			},
		},
		"error when both name and path conflict": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:     "trainer",
							Ancestor: ptr.To(constants.AncestorTrainer),
							Count:    ptr.To[int32](1),
							Containers: []runtime.Container{
								{
									Name: constants.Node,
									VolumeMounts: []corev1ac.VolumeMountApplyConfiguration{
										*corev1ac.VolumeMount().WithName(tokenVolumeName).WithMountPath(configMountPath),
									},
								},
							},
							Volumes: []corev1ac.VolumeApplyConfiguration{
								*corev1ac.Volume().WithName(tokenVolumeName),
							},
						},
					},
				},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").Obj(),
			wantErrs: field.ErrorList{
				field.Forbidden(
					field.NewPath("spec", "template", "spec", "volumes"),
					fmt.Sprintf("volume name %q in podSet %q is reserved for %s", tokenVolumeName, "trainer", Name),
				),
				field.Forbidden(
					field.NewPath("spec", "template", "spec", "containers").Index(0).Child("volumeMounts"),
					fmt.Sprintf("volume mount name %q in container %q of podSet %q is reserved for %s", tokenVolumeName, constants.Node, "trainer", Name),
				),
				field.Forbidden(
					field.NewPath("spec", "template", "spec", "containers").Index(0).Child("volumeMounts"),
					fmt.Sprintf("volume mount path %q in container %q of podSet %q is reserved for %s", configMountPath, constants.Node, "trainer", Name),
				),
			},
		},
		"error in non-first container identifies that container": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:     "trainer",
							Ancestor: ptr.To(constants.AncestorTrainer),
							Count:    ptr.To[int32](1),
							Containers: []runtime.Container{
								{
									Name: constants.Node,
									VolumeMounts: []corev1ac.VolumeMountApplyConfiguration{
										*corev1ac.VolumeMount().WithName("data").WithMountPath("/data"),
									},
								},
								{
									Name: "sidecar",
									VolumeMounts: []corev1ac.VolumeMountApplyConfiguration{
										*corev1ac.VolumeMount().WithName("other-name").WithMountPath(configMountPath),
									},
								},
							},
						},
					},
				},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").Obj(),
			wantErrs: field.ErrorList{
				field.Forbidden(
					field.NewPath("spec", "template", "spec", "containers").Index(1).Child("volumeMounts"),
					fmt.Sprintf("volume mount path %q in container %q of podSet %q is reserved for %s", configMountPath, "sidecar", "trainer", Name),
				),
			},
		},
		"non-trainer podset with same volume name does not error": {
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:     "dataset-initializer",
							Ancestor: ptr.To(constants.DatasetInitializer),
							Count:    ptr.To[int32](1),
							Containers: []runtime.Container{
								{
									Name: "dataset-initializer",
									VolumeMounts: []corev1ac.VolumeMountApplyConfiguration{
										*corev1ac.VolumeMount().WithName(tokenVolumeName).WithMountPath(configMountPath),
									},
								},
							},
							Volumes: []corev1ac.VolumeApplyConfiguration{
								*corev1ac.Volume().WithName(tokenVolumeName),
							},
						},
						{
							Name:     "trainer",
							Ancestor: ptr.To(constants.AncestorTrainer),
							Count:    ptr.To[int32](1),
							Containers: []runtime.Container{
								{
									Name: constants.Node,
									VolumeMounts: []corev1ac.VolumeMountApplyConfiguration{
										*corev1ac.VolumeMount().WithName("data").WithMountPath("/data"),
									},
								},
							},
							Volumes: []corev1ac.VolumeApplyConfiguration{
								*corev1ac.Volume().WithName("data"),
							},
						},
					},
				},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").Obj(),
			wantErrs: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			var cancel func()
			ctx, cancel = context.WithCancel(ctx)
			t.Cleanup(cancel)

			cli := utiltesting.NewClientBuilder().Build()
			cfg := &configapi.Configuration{
				CertManagement: &configapi.CertManagement{
					WebhookServiceName: "kubeflow-trainer-controller-manager",
					WebhookSecretName:  "kubeflow-trainer-webhook-cert",
				},
				StatusServer: &configapi.StatusServer{
					Port:  ptr.To[int32](10443),
					QPS:   ptr.To[float32](5),
					Burst: ptr.To[int32](10),
				},
			}

			p, err := New(ctx, cli, nil, cfg)
			if err != nil {
				t.Fatalf("Failed to initialize Status plugin: %v", err)
			}

			_, errs := p.(framework.CustomValidationPlugin).Validate(ctx, tc.info, nil, tc.trainJob)
			if diff := gocmp.Diff(tc.wantErrs, errs); len(diff) != 0 {
				t.Errorf("Unexpected validation errors (-want, +got):\n%s", diff)
			}
		})
	}
}
