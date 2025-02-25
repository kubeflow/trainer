package jobset

import (
	corev1 "k8s.io/api/core/v1"
)

const (

	// VolumeNameInitializer is the name for the initializer Pod's Volume and VolumeMount.
	// TODO (andreyvelich): Add validation to check that initializer Pod has the correct volume.
	VolumeNameInitializer string = "initializer"

	// InitializerEnvStorageUri is the env name for the initializer storage uri.
	InitializerEnvStorageUri string = "STORAGE_URI"
)

var (
	// This is the temporary container that we use in the initializer ReplicatedJob.
	// TODO (andreyvelich): Once JobSet supports execution policy, we can remove it.
	ContainerBusyBox corev1.Container = corev1.Container{
		Name:  "busybox",
		Image: "busybox:stable-glibc",
	}

	// VolumeMountModelInitializer is the volume mount for the model initializer container.
	// TODO (andreyvelich): Add validation to check that initializer ReplicatedJob has the following volumes.
	VolumeMountModelInitializer = corev1.VolumeMount{
		Name:      VolumeNameInitializer,
		MountPath: "/workspace/model",
	}

	// VolumeMountModelInitializer is the volume mount for the dataset initializer container.
	VolumeMountDatasetInitializer = corev1.VolumeMount{
		Name:      VolumeNameInitializer,
		MountPath: "/workspace/dataset",
	}

	// VolumeInitializer is the volume for the initializer ReplicatedJob.
	// TODO (andreyvelich): We should make VolumeSource configurable.
	VolumeInitializer = corev1.Volume{
		Name: VolumeNameInitializer,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: VolumeNameInitializer,
			},
		},
	}
)
