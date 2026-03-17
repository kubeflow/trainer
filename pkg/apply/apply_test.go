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

package apply

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestUpsertEnvVars(t *testing.T) {
	cases := map[string]struct {
		existing []corev1ac.EnvVarApplyConfiguration
		toUpsert []corev1ac.EnvVarApplyConfiguration
		want     []corev1ac.EnvVarApplyConfiguration
	}{
		"insert new env var": {
			existing: []corev1ac.EnvVarApplyConfiguration{},
			toUpsert: []corev1ac.EnvVarApplyConfiguration{
				*corev1ac.EnvVar().WithName("TEST").WithValue("value"),
			},
			want: []corev1ac.EnvVarApplyConfiguration{
				*corev1ac.EnvVar().WithName("TEST").WithValue("value"),
			},
		},
		"update existing env var": {
			existing: []corev1ac.EnvVarApplyConfiguration{
				*corev1ac.EnvVar().WithName("TEST").WithValue("old"),
			},
			toUpsert: []corev1ac.EnvVarApplyConfiguration{
				*corev1ac.EnvVar().WithName("TEST").WithValue("new"),
			},
			want: []corev1ac.EnvVarApplyConfiguration{
				*corev1ac.EnvVar().WithName("TEST").WithValue("new"),
			},
		},
		"insert multiple env vars": {
			existing: []corev1ac.EnvVarApplyConfiguration{
				*corev1ac.EnvVar().WithName("EXISTING").WithValue("value"),
			},
			toUpsert: []corev1ac.EnvVarApplyConfiguration{
				*corev1ac.EnvVar().WithName("NEW1").WithValue("value1"),
				*corev1ac.EnvVar().WithName("NEW2").WithValue("value2"),
			},
			want: []corev1ac.EnvVarApplyConfiguration{
				*corev1ac.EnvVar().WithName("EXISTING").WithValue("value"),
				*corev1ac.EnvVar().WithName("NEW1").WithValue("value1"),
				*corev1ac.EnvVar().WithName("NEW2").WithValue("value2"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			envVars := make([]corev1ac.EnvVarApplyConfiguration, len(tc.existing))
			copy(envVars, tc.existing)
			UpsertEnvVars(&envVars, tc.toUpsert...)
			if diff := cmp.Diff(tc.want, envVars); diff != "" {
				t.Errorf("Unexpected env vars (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUpsertPort(t *testing.T) {
	cases := map[string]struct {
		existing []corev1ac.ContainerPortApplyConfiguration
		toUpsert []corev1ac.ContainerPortApplyConfiguration
		want     []corev1ac.ContainerPortApplyConfiguration
	}{
		"match by port number": {
			existing: []corev1ac.ContainerPortApplyConfiguration{
				*corev1ac.ContainerPort().WithContainerPort(8080),
			},
			toUpsert: []corev1ac.ContainerPortApplyConfiguration{
				*corev1ac.ContainerPort().
					WithContainerPort(8080).
					WithProtocol("TCP"),
			},
			want: []corev1ac.ContainerPortApplyConfiguration{
				*corev1ac.ContainerPort().
					WithContainerPort(8080).
					WithProtocol("TCP"),
			},
		},
		"match by name": {
			existing: []corev1ac.ContainerPortApplyConfiguration{
				*corev1ac.ContainerPort().WithName("http"),
			},
			toUpsert: []corev1ac.ContainerPortApplyConfiguration{
				*corev1ac.ContainerPort().
					WithName("http").
					WithContainerPort(8080),
			},
			want: []corev1ac.ContainerPortApplyConfiguration{
				*corev1ac.ContainerPort().
					WithName("http").
					WithContainerPort(8080),
			},
		},
		"insert new port": {
			existing: []corev1ac.ContainerPortApplyConfiguration{
				*corev1ac.ContainerPort().WithContainerPort(8080),
			},
			toUpsert: []corev1ac.ContainerPortApplyConfiguration{
				*corev1ac.ContainerPort().WithContainerPort(9090),
			},
			want: []corev1ac.ContainerPortApplyConfiguration{
				*corev1ac.ContainerPort().WithContainerPort(9090),
			},
		},
		"insert new port with different names": {
			existing: []corev1ac.ContainerPortApplyConfiguration{
				*corev1ac.ContainerPort().
					WithName("http").
					WithContainerPort(8080),
			},
			toUpsert: []corev1ac.ContainerPortApplyConfiguration{
				*corev1ac.ContainerPort().
					WithName("metrics").
					WithContainerPort(9090),
			},
			want: []corev1ac.ContainerPortApplyConfiguration{
				*corev1ac.ContainerPort().
					WithName("http").
					WithContainerPort(8080),
				*corev1ac.ContainerPort().
					WithName("metrics").
					WithContainerPort(9090),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ports := make([]corev1ac.ContainerPortApplyConfiguration, len(tc.existing))
			copy(ports, tc.existing)
			UpsertPort(&ports, tc.toUpsert...)
			if diff := cmp.Diff(tc.want, ports); diff != "" {
				t.Errorf("Unexpected ports (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUpsertVolumes(t *testing.T) {
	cases := map[string]struct {
		existing []corev1ac.VolumeApplyConfiguration
		toUpsert []corev1ac.VolumeApplyConfiguration
		want     []corev1ac.VolumeApplyConfiguration
	}{
		"update existing volume": {
			existing: []corev1ac.VolumeApplyConfiguration{
				*corev1ac.Volume().WithName("data"),
			},
			toUpsert: []corev1ac.VolumeApplyConfiguration{
				*corev1ac.Volume().
					WithName("data").
					WithEmptyDir(corev1ac.EmptyDirVolumeSource()),
			},
			want: []corev1ac.VolumeApplyConfiguration{
				*corev1ac.Volume().
					WithName("data").
					WithEmptyDir(corev1ac.EmptyDirVolumeSource()),
			},
		},
		"insert new volume": {
			existing: []corev1ac.VolumeApplyConfiguration{
				*corev1ac.Volume().WithName("existing"),
			},
			toUpsert: []corev1ac.VolumeApplyConfiguration{
				*corev1ac.Volume().
					WithName("new").
					WithEmptyDir(corev1ac.EmptyDirVolumeSource()),
			},
			want: []corev1ac.VolumeApplyConfiguration{
				*corev1ac.Volume().WithName("existing"),
				*corev1ac.Volume().
					WithName("new").
					WithEmptyDir(corev1ac.EmptyDirVolumeSource()),
			},
		},
		"update multiple volumes": {
			existing: []corev1ac.VolumeApplyConfiguration{
				*corev1ac.Volume().WithName("vol1"),
				*corev1ac.Volume().WithName("vol2"),
			},
			toUpsert: []corev1ac.VolumeApplyConfiguration{
				*corev1ac.Volume().
					WithName("vol1").
					WithEmptyDir(corev1ac.EmptyDirVolumeSource()),
				*corev1ac.Volume().
					WithName("vol3").
					WithEmptyDir(corev1ac.EmptyDirVolumeSource()),
			},
			want: []corev1ac.VolumeApplyConfiguration{
				*corev1ac.Volume().
					WithName("vol1").
					WithEmptyDir(corev1ac.EmptyDirVolumeSource()),
				*corev1ac.Volume().WithName("vol2"),
				*corev1ac.Volume().
					WithName("vol3").
					WithEmptyDir(corev1ac.EmptyDirVolumeSource()),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			volumes := make([]corev1ac.VolumeApplyConfiguration, len(tc.existing))
			copy(volumes, tc.existing)
			UpsertVolumes(&volumes, tc.toUpsert...)
			if diff := cmp.Diff(tc.want, volumes); diff != "" {
				t.Errorf("Unexpected volumes (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUpsertVolumeMounts(t *testing.T) {
	cases := map[string]struct {
		existing []corev1ac.VolumeMountApplyConfiguration
		toUpsert []corev1ac.VolumeMountApplyConfiguration
		want     []corev1ac.VolumeMountApplyConfiguration
	}{
		"update existing volume mount": {
			existing: []corev1ac.VolumeMountApplyConfiguration{
				*corev1ac.VolumeMount().
					WithName("data").
					WithMountPath("/old-data"),
			},
			toUpsert: []corev1ac.VolumeMountApplyConfiguration{
				*corev1ac.VolumeMount().
					WithName("data").
					WithMountPath("/old-data").
					WithReadOnly(true),
			},
			want: []corev1ac.VolumeMountApplyConfiguration{
				*corev1ac.VolumeMount().
					WithName("data").
					WithMountPath("/old-data").
					WithReadOnly(true),
			},
		},
		"insert new volume mount": {
			existing: []corev1ac.VolumeMountApplyConfiguration{
				*corev1ac.VolumeMount().
					WithName("existing").
					WithMountPath("/existing"),
			},
			toUpsert: []corev1ac.VolumeMountApplyConfiguration{
				*corev1ac.VolumeMount().
					WithName("new").
					WithMountPath("/new"),
			},
			want: []corev1ac.VolumeMountApplyConfiguration{
				*corev1ac.VolumeMount().
					WithName("existing").
					WithMountPath("/existing"),
				*corev1ac.VolumeMount().
					WithName("new").
					WithMountPath("/new"),
			},
		},
		"update based on mount path": {
			existing: []corev1ac.VolumeMountApplyConfiguration{
				*corev1ac.VolumeMount().
					WithName("vol1").
					WithMountPath("/data"),
				*corev1ac.VolumeMount().
					WithName("vol2").
					WithMountPath("/config"),
			},
			toUpsert: []corev1ac.VolumeMountApplyConfiguration{
				*corev1ac.VolumeMount().
					WithName("vol3").
					WithMountPath("/data").
					WithReadOnly(true),
			},
			want: []corev1ac.VolumeMountApplyConfiguration{
				*corev1ac.VolumeMount().
					WithName("vol3").
					WithMountPath("/data").
					WithReadOnly(true),
				*corev1ac.VolumeMount().
					WithName("vol2").
					WithMountPath("/config"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mounts := make([]corev1ac.VolumeMountApplyConfiguration, len(tc.existing))
			copy(mounts, tc.existing)
			UpsertVolumeMounts(&mounts, tc.toUpsert...)
			if diff := cmp.Diff(tc.want, mounts); diff != "" {
				t.Errorf("Unexpected mounts (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEnvVar(t *testing.T) {
	cases := map[string]struct {
		input corev1.EnvVar
		want  *corev1ac.EnvVarApplyConfiguration
	}{
		"simple value": {
			input: corev1.EnvVar{
				Name:  "SIMPLE",
				Value: "value",
			},
			want: corev1ac.EnvVar().WithName("SIMPLE").WithValue("value"),
		},
		"configmap ref": {
			input: corev1.EnvVar{
				Name: "FROM_CONFIG",
				ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "config"},
						Key:                  "key",
						Optional:             ptr.To(true),
					},
				},
			},
			want: corev1ac.EnvVar().WithName("FROM_CONFIG").WithValueFrom(
				corev1ac.EnvVarSource().WithConfigMapKeyRef(
					corev1ac.ConfigMapKeySelector().
						WithName("config").
						WithKey("key").
						WithOptional(true),
				),
			),
		},
		"field ref": {
			input: corev1.EnvVar{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
			want: corev1ac.EnvVar().WithName("POD_NAME").WithValueFrom(
				corev1ac.EnvVarSource().WithFieldRef(
					corev1ac.ObjectFieldSelector().WithFieldPath("metadata.name"),
				),
			),
		},
		"resource field ref": {
			input: corev1.EnvVar{
				Name: "CPU_REQUEST",
				ValueFrom: &corev1.EnvVarSource{
					ResourceFieldRef: &corev1.ResourceFieldSelector{
						ContainerName: "container",
						Resource:      "requests.cpu",
						Divisor:       resource.MustParse("1m"),
					},
				},
			},
			want: corev1ac.EnvVar().WithName("CPU_REQUEST").WithValueFrom(
				corev1ac.EnvVarSource().WithResourceFieldRef(
					corev1ac.ResourceFieldSelector().
						WithContainerName("container").
						WithResource("requests.cpu").
						WithDivisor(resource.MustParse("1m")),
				),
			),
		},
		"secret ref": {
			input: corev1.EnvVar{
				Name: "FROM_SECRET",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "secret"},
						Key:                  "key",
						Optional:             ptr.To(true),
					},
				},
			},
			want: corev1ac.EnvVar().WithName("FROM_SECRET").WithValueFrom(
				corev1ac.EnvVarSource().WithSecretKeyRef(
					corev1ac.SecretKeySelector().
						WithName("secret").
						WithKey("key").
						WithOptional(true),
				),
			),
		},
		"field ref and resource field ref combined": {
			input: corev1.EnvVar{
				Name: "COMBINED",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
					ResourceFieldRef: &corev1.ResourceFieldSelector{
						ContainerName: "container",
						Resource:      "requests.cpu",
						Divisor:       resource.MustParse("1m"),
					},
				},
			},
			want: corev1ac.EnvVar().WithName("COMBINED").WithValueFrom(
				corev1ac.EnvVarSource().WithFieldRef(
					corev1ac.ObjectFieldSelector().WithFieldPath("metadata.name"),
				).WithResourceFieldRef(
					corev1ac.ResourceFieldSelector().
						WithContainerName("container").
						WithResource("requests.cpu").
						WithDivisor(resource.MustParse("1m")),
				),
			),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := EnvVar(tc.input)
			if diff := cmp.Diff(tc.want, result); diff != "" {
				t.Errorf("Unexpected EnvVar (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEnvVars(t *testing.T) {
	cases := map[string]struct {
		input []corev1.EnvVar
		want  []corev1ac.EnvVarApplyConfiguration
	}{
		"empty input": {
			input: []corev1.EnvVar{},
			want:  nil,
		},
		"single env var": {
			input: []corev1.EnvVar{
				{
					Name:  "SIMPLE",
					Value: "value",
				},
			},
			want: []corev1ac.EnvVarApplyConfiguration{
				*corev1ac.EnvVar().WithName("SIMPLE").WithValue("value"),
			},
		},
		"multiple env vars": {
			input: []corev1.EnvVar{
				{
					Name:  "VAR1",
					Value: "value1",
				},
				{
					Name: "VAR2",
					ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "config"},
							Key:                  "key",
							Optional:             ptr.To(true),
						},
					},
				},
			},
			want: []corev1ac.EnvVarApplyConfiguration{
				*corev1ac.EnvVar().WithName("VAR1").WithValue("value1"),
				*corev1ac.EnvVar().WithName("VAR2").WithValueFrom(
					corev1ac.EnvVarSource().WithConfigMapKeyRef(
						corev1ac.ConfigMapKeySelector().
							WithName("config").
							WithKey("key").
							WithOptional(true),
					),
				),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := EnvVars(tc.input...)
			if diff := cmp.Diff(tc.want, result); diff != "" {
				t.Errorf("Unexpected EnvVars (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFromTypedObjWithFields(t *testing.T) {
	cases := map[string]struct {
		input     client.Object
		fields    []string
		want      interface{}
		wantError error
	}{
		"extract simple field from Pod": {
			input: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
			fields: []string{"metadata", "name"},
			want:   ptr.To(interface{}("test-pod")),
		},
		"extract container from Pod spec": {
			input: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "test:latest",
							Env: []corev1.EnvVar{
								{Name: "TEST_VAR", Value: "test-value"},
							},
						},
					},
				},
			},
			fields: []string{"spec", "containers"},
			want: ptr.To(interface{}([]interface{}{
				map[string]interface{}{
					"name":      "test-container",
					"image":     "test:latest",
					"resources": map[string]interface{}{},
					"env": []interface{}{
						map[string]interface{}{
							"name":  "TEST_VAR",
							"value": "test-value",
						},
					},
				},
			})),
		},
		"field not found": {
			input: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
				},
			},
			fields:    []string{"spec", "nonexistent"},
			wantError: errorRequestedFieldPathNotFound,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if tc.wantError != nil {
				_, err := FromTypedObjWithFields[interface{}](tc.input, tc.fields...)
				if err == nil {
					t.Errorf("expected error %v but got none", tc.wantError)
				}
				if !errors.Is(err, tc.wantError) {
					t.Errorf("expected error %v, got %v", tc.wantError, err)
				}
				return
			}

			result, err := FromTypedObjWithFields[interface{}](tc.input, tc.fields...)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.want, result); diff != "" {
				t.Errorf("Unexpected result (-want +got):\n%s", diff)
			}
		})
	}
}
