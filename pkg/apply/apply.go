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
	corev1 "k8s.io/api/core/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/utils/ptr"
)

func UpsertEnvVar(envVars *[]corev1ac.EnvVarApplyConfiguration, envVar ...corev1ac.EnvVarApplyConfiguration) {
	for _, e := range envVar {
		upsert(envVars, e, byEnvVarName)
	}
}

func UpsertEnvVars(envVars *[]corev1ac.EnvVarApplyConfiguration, upEnvVars ...corev1ac.EnvVarApplyConfiguration) {
	for _, e := range upEnvVars {
		upsert(envVars, e, byEnvVarName)
	}
}

func UpsertPort(ports *[]corev1ac.ContainerPortApplyConfiguration, port ...corev1ac.ContainerPortApplyConfiguration) {
	for _, p := range port {
		upsert(ports, p, byContainerPortOrName)
	}
}

func UpsertVolumes(volumes *[]corev1ac.VolumeApplyConfiguration, upVolumes ...corev1ac.VolumeApplyConfiguration) {
	for _, v := range upVolumes {
		upsert(volumes, v, byVolumeName)
	}
}

func UpsertVolumeMounts(mounts *[]corev1ac.VolumeMountApplyConfiguration, upMounts ...corev1ac.VolumeMountApplyConfiguration) {
	for _, m := range upMounts {
		upsert(mounts, m, byVolumeMountPath)
	}
}

func byEnvVarName(a, b corev1ac.EnvVarApplyConfiguration) bool {
	return ptr.Equal(a.Name, b.Name)
}

func byContainerPortOrName(a, b corev1ac.ContainerPortApplyConfiguration) bool {
	return ptr.Equal(a.ContainerPort, b.ContainerPort) || ptr.Equal(a.Name, b.Name)
}

func byVolumeName(a, b corev1ac.VolumeApplyConfiguration) bool {
	return ptr.Equal(a.Name, b.Name)
}

func byVolumeMountPath(a, b corev1ac.VolumeMountApplyConfiguration) bool {
	return ptr.Equal(a.MountPath, b.MountPath)
}

type compare[T any] func(T, T) bool

func upsert[T any](items *[]T, item T, predicate compare[T]) {
	for i, t := range *items {
		if predicate(t, item) {
			(*items)[i] = item
			return
		}
	}
	*items = append(*items, item)
}

func EnvVar(e corev1.EnvVar) *corev1ac.EnvVarApplyConfiguration {
	envVar := corev1ac.EnvVar().WithName(e.Name)
	if from := e.ValueFrom; from != nil {
		source := corev1ac.EnvVarSource()
		if ref := from.FieldRef; ref != nil {
			source.WithFieldRef(corev1ac.ObjectFieldSelector().WithFieldPath(ref.FieldPath))
		}
		if ref := from.ResourceFieldRef; ref != nil {
			source.WithResourceFieldRef(corev1ac.ResourceFieldSelector().
				WithContainerName(ref.ContainerName).
				WithResource(ref.Resource).
				WithDivisor(ref.Divisor))
		}
		if ref := from.ConfigMapKeyRef; ref != nil {
			key := corev1ac.ConfigMapKeySelector().WithKey(ref.Key).WithName(ref.Name)
			if optional := ref.Optional; optional != nil {
				key.WithOptional(*optional)
			}
			source.WithConfigMapKeyRef(key)
		}
		if ref := from.SecretKeyRef; ref != nil {
			key := corev1ac.SecretKeySelector().WithKey(ref.Key).WithName(ref.Name)
			if optional := ref.Optional; optional != nil {
				key.WithOptional(*optional)
			}
			source.WithSecretKeyRef(key)
		}
		envVar.WithValueFrom(source)
	} else {
		envVar.WithValue(e.Value)
	}
	return envVar
}

func EnvVars(e ...corev1.EnvVar) []corev1ac.EnvVarApplyConfiguration {
	var envs []corev1ac.EnvVarApplyConfiguration
	for _, env := range e {
		envs = append(envs, *EnvVar(env))
	}
	return envs
}

func Volume(v corev1.Volume) *corev1ac.VolumeApplyConfiguration {
	vol := corev1ac.Volume().
		WithName(v.Name)
	if pvc := v.VolumeSource.PersistentVolumeClaim; pvc != nil {
		vol.WithPersistentVolumeClaim(corev1ac.PersistentVolumeClaimVolumeSource().
			WithClaimName(pvc.ClaimName).
			WithReadOnly(pvc.ReadOnly))
	}
	if sec := v.VolumeSource.Secret; sec != nil {
		secSource := corev1ac.SecretVolumeSource().WithSecretName(sec.SecretName)
		if sec.Optional != nil {
			secSource.WithOptional(*sec.Optional)
		}
		if sec.DefaultMode != nil {
			secSource.WithDefaultMode(*sec.DefaultMode)
		}
		var secItems []*corev1ac.KeyToPathApplyConfiguration
		for _, item := range sec.Items {
			keyToPath := corev1ac.KeyToPath().WithKey(item.Key).WithPath(item.Path)
			if item.Mode != nil {
				keyToPath.WithMode(*item.Mode)
			}
			secItems = append(secItems, keyToPath)
		}
		secSource.WithItems(secItems...)
		vol.WithSecret(secSource)
	}
	if cm := v.VolumeSource.ConfigMap; cm != nil {
		cmSource := corev1ac.ConfigMapVolumeSource().WithName(cm.Name)
		if cm.Optional != nil {
			cmSource.WithOptional(*cm.Optional)
		}
		if cm.DefaultMode != nil {
			cmSource.WithDefaultMode(*cm.DefaultMode)
		}
		var cmItems []*corev1ac.KeyToPathApplyConfiguration
		for _, item := range cm.Items {
			keyToPath := corev1ac.KeyToPath().WithKey(item.Key).WithPath(item.Path)
			if item.Mode != nil {
				keyToPath.WithMode(*item.Mode)
			}
			cmItems = append(cmItems, keyToPath)
		}
		cmSource.WithItems(cmItems...)
		vol.WithConfigMap(cmSource)
	}
	// TODO: Add other volume sources
	// Remaining items:
	// - HostPath
	// - EmptyDir
	// - NFS
	// - ISCSI
	// - DownwardAPI
	// - FC
	// - Projected
	// - CSI
	// - Ephemeral
	// - Image
	return vol
}

func Volumes(v ...corev1.Volume) []corev1ac.VolumeApplyConfiguration {
	var vols []corev1ac.VolumeApplyConfiguration
	for _, vol := range v {
		vols = append(vols, *Volume(vol))
	}
	return vols
}

func VolumeMount(vm corev1.VolumeMount) *corev1ac.VolumeMountApplyConfiguration {
	volMount := corev1ac.VolumeMount().
		WithName(vm.Name).
		WithReadOnly(vm.ReadOnly).
		WithMountPath(vm.MountPath)
	if len(vm.SubPath) != 0 {
		volMount.WithSubPath(vm.SubPath)
	}
	if len(vm.SubPathExpr) != 0 {
		volMount.WithSubPathExpr(vm.SubPathExpr)
	}
	if vm.MountPropagation != nil {
		volMount.WithMountPropagation(*vm.MountPropagation)
	}
	if vm.RecursiveReadOnly != nil {
		volMount.WithRecursiveReadOnly(*vm.RecursiveReadOnly)
	}
	return volMount
}

func VolumeMounts(vm ...corev1.VolumeMount) []corev1ac.VolumeMountApplyConfiguration {
	var volMounts []corev1ac.VolumeMountApplyConfiguration
	for _, volMount := range vm {
		volMounts = append(volMounts, *VolumeMount(volMount))
	}
	return volMounts
}

func ContainerPort(p corev1.ContainerPort) *corev1ac.ContainerPortApplyConfiguration {
	port := corev1ac.ContainerPort().
		WithName(p.Name).
		WithProtocol(p.Protocol)
	if p.ContainerPort != 0 {
		port.WithContainerPort(p.ContainerPort)
	}
	if len(p.HostIP) != 0 {
		port.WithHostIP(p.HostIP)
	}
	if p.HostPort != 0 {
		port.WithHostPort(p.HostPort)
	}
	return port
}

func ContainerPorts(p ...corev1.ContainerPort) []corev1ac.ContainerPortApplyConfiguration {
	var ports []corev1ac.ContainerPortApplyConfiguration
	for _, port := range p {
		ports = append(ports, *ContainerPort(port))
	}
	return ports
}
