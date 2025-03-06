package apply

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/utils/ptr"
)

func TestUpsertEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		existing []corev1ac.EnvVarApplyConfiguration
		toUpsert []*corev1ac.EnvVarApplyConfiguration
		expected []corev1ac.EnvVarApplyConfiguration
	}{
		{
			name:     "insert new env var",
			existing: []corev1ac.EnvVarApplyConfiguration{},
			toUpsert: []*corev1ac.EnvVarApplyConfiguration{
				corev1ac.EnvVar().WithName("TEST").WithValue("value"),
			},
			expected: []corev1ac.EnvVarApplyConfiguration{
				*corev1ac.EnvVar().WithName("TEST").WithValue("value"),
			},
		},
		{
			name: "update existing env var",
			existing: []corev1ac.EnvVarApplyConfiguration{
				*corev1ac.EnvVar().WithName("TEST").WithValue("old"),
			},
			toUpsert: []*corev1ac.EnvVarApplyConfiguration{
				corev1ac.EnvVar().WithName("TEST").WithValue("new"),
			},
			expected: []corev1ac.EnvVarApplyConfiguration{
				*corev1ac.EnvVar().WithName("TEST").WithValue("new"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envVars := tt.existing
			UpsertEnvVar(&envVars, tt.toUpsert...)
			assert.Equal(t, tt.expected, envVars)
		})
	}
}

func TestUpsertEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		existing []corev1ac.EnvVarApplyConfiguration
		toUpsert []corev1ac.EnvVarApplyConfiguration
		expected []corev1ac.EnvVarApplyConfiguration
	}{
		{
			name: "insert new env var",
			existing: []corev1ac.EnvVarApplyConfiguration{
				*corev1ac.EnvVar().WithName("TEST").WithValue("old"),
			},
			toUpsert: []corev1ac.EnvVarApplyConfiguration{
				*corev1ac.EnvVar().WithName("TEST").WithValue("new"),
			},
			expected: []corev1ac.EnvVarApplyConfiguration{
				*corev1ac.EnvVar().WithName("TEST").WithValue("new"),
			},
		},
		{
			name: "update existing env var",
			existing: []corev1ac.EnvVarApplyConfiguration{
				*corev1ac.EnvVar().WithName("TEST").WithValue("old"),
			},
			toUpsert: []corev1ac.EnvVarApplyConfiguration{
				*corev1ac.EnvVar().WithName("TEST").WithValue("new"),
			},
			expected: []corev1ac.EnvVarApplyConfiguration{
				*corev1ac.EnvVar().WithName("TEST").WithValue("new"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envVars := tt.existing
			UpsertEnvVars(&envVars, tt.toUpsert)
			assert.Equal(t, tt.expected, envVars)
		})
	}

}

func TestUpsertPort(t *testing.T) {
	tests := []struct {
		name     string
		existing []corev1ac.ContainerPortApplyConfiguration
		toUpsert []*corev1ac.ContainerPortApplyConfiguration
		expected []corev1ac.ContainerPortApplyConfiguration
	}{
		{
			name: "match by port number",
			existing: []corev1ac.ContainerPortApplyConfiguration{
				*corev1ac.ContainerPort().WithContainerPort(8080),
			},
			toUpsert: []*corev1ac.ContainerPortApplyConfiguration{
				corev1ac.ContainerPort().WithContainerPort(8080).WithProtocol("TCP"),
			},
			expected: []corev1ac.ContainerPortApplyConfiguration{
				*corev1ac.ContainerPort().WithContainerPort(8080).WithProtocol("TCP"),
			},
		},
		{
			name: "match by name",
			existing: []corev1ac.ContainerPortApplyConfiguration{
				*corev1ac.ContainerPort().WithName("http"),
			},
			toUpsert: []*corev1ac.ContainerPortApplyConfiguration{
				corev1ac.ContainerPort().WithName("http").WithContainerPort(8080),
			},
			expected: []corev1ac.ContainerPortApplyConfiguration{
				*corev1ac.ContainerPort().WithName("http").WithContainerPort(8080),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ports := tt.existing
			UpsertPort(&ports, tt.toUpsert...)
			assert.Equal(t, tt.expected, ports)
		})
	}
}

func TestEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		input    corev1.EnvVar
		expected *corev1ac.EnvVarApplyConfiguration
	}{
		{
			name: "simple value",
			input: corev1.EnvVar{
				Name:  "SIMPLE",
				Value: "value",
			},
			expected: corev1ac.EnvVar().WithName("SIMPLE").WithValue("value"),
		},
		{
			name: "configmap ref",
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
			expected: corev1ac.EnvVar().WithName("FROM_CONFIG").WithValueFrom(
				corev1ac.EnvVarSource().WithConfigMapKeyRef(
					corev1ac.ConfigMapKeySelector().
						WithName("config").
						WithKey("key").
						WithOptional(true),
				),
			),
		},
		{
			name: "field ref",
			input: corev1.EnvVar{
				Name: "POD_NAME",
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
			expected: corev1ac.EnvVar().WithName("POD_NAME").WithValueFrom(
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
		{
			name: "secret ref",
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
			expected: corev1ac.EnvVar().WithName("FROM_SECRET").WithValueFrom(
				corev1ac.EnvVarSource().WithSecretKeyRef(
					corev1ac.SecretKeySelector().
						WithName("secret").
						WithKey("key").
						WithOptional(true),
				),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EnvVar(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUpsertVolumes(t *testing.T) {
	tests := []struct {
		name     string
		existing []corev1ac.VolumeApplyConfiguration
		toUpsert []corev1ac.VolumeApplyConfiguration
		expected []corev1ac.VolumeApplyConfiguration
	}{
		{
			name: "update existing volume",
			existing: []corev1ac.VolumeApplyConfiguration{
				*corev1ac.Volume().WithName("data"),
			},
			toUpsert: []corev1ac.VolumeApplyConfiguration{
				*corev1ac.Volume().WithName("data").WithEmptyDir(corev1ac.EmptyDirVolumeSource()),
			},
			expected: []corev1ac.VolumeApplyConfiguration{
				*corev1ac.Volume().WithName("data").WithEmptyDir(corev1ac.EmptyDirVolumeSource()),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			volumes := tt.existing
			UpsertVolumes(&volumes, tt.toUpsert)
			assert.Equal(t, tt.expected, volumes)
		})
	}
}

func TestUpsertVolumesMounts(t *testing.T) {
	tests := []struct {
		name     string
		existing []corev1ac.VolumeMountApplyConfiguration
		toUpsert []corev1ac.VolumeMountApplyConfiguration
		expected []corev1ac.VolumeMountApplyConfiguration
	}{
		{
			name: "update existing volume mount",
			existing: []corev1ac.VolumeMountApplyConfiguration{
				*corev1ac.VolumeMount().WithName("data"),
			},
			toUpsert: []corev1ac.VolumeMountApplyConfiguration{
				*corev1ac.VolumeMount().WithName("data").WithMountPath("/data"),
			},
			expected: []corev1ac.VolumeMountApplyConfiguration{
				*corev1ac.VolumeMount().WithName("data").WithMountPath("/data"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mounts := tt.existing
			UpsertVolumeMounts(&mounts, tt.toUpsert)
			assert.Equal(t, tt.expected, mounts)
		})
	}
}

func TestEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		input    corev1.EnvVar
		expected []corev1ac.EnvVarApplyConfiguration
	}{
		{
			name: "simple value",
			input: corev1.EnvVar{

				Name:  "SIMPLE",
				Value: "value",
			},
			expected: []corev1ac.EnvVarApplyConfiguration{
				*corev1ac.EnvVar().WithName("SIMPLE").WithValue("value"),
			},
		},
		{
			name: "configmap ref",
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
			expected: []corev1ac.EnvVarApplyConfiguration{
				*corev1ac.EnvVar().WithName("FROM_CONFIG").WithValueFrom(
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EnvVars(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}

}
