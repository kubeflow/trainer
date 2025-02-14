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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
)

func ContainerPort(p corev1.ContainerPort) *corev1ac.ContainerPortApplyConfiguration {
	port := corev1ac.ContainerPort()
	if p.ContainerPort > 0 {
		port.WithContainerPort(p.ContainerPort)
	}
	if p.HostPort > 0 {
		port.WithHostPort(p.HostPort)
	}
	if p.HostIP != "" {
		port.WithHostIP(p.HostIP)
	}
	if p.Name != "" {
		port.WithName(p.Name)
	}
	if p.Protocol != "" {
		port.WithProtocol(p.Protocol)
	}
	return port
}

func ContainerPorts(p ...corev1.ContainerPort) []*corev1ac.ContainerPortApplyConfiguration {
	var ports []*corev1ac.ContainerPortApplyConfiguration
	for _, port := range p {
		ports = append(ports, ContainerPort(port))
	}
	return ports
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

func EnvVars(e ...corev1.EnvVar) []*corev1ac.EnvVarApplyConfiguration {
	var envs []*corev1ac.EnvVarApplyConfiguration
	for _, env := range e {
		envs = append(envs, EnvVar(env))
	}
	return envs
}

func EnvFromSource(e corev1.EnvFromSource) *corev1ac.EnvFromSourceApplyConfiguration {
	envVarFrom := corev1ac.EnvFromSource()
	if e.Prefix != "" {
		envVarFrom.WithPrefix(e.Prefix)
	}
	if ref := e.ConfigMapRef; ref != nil {
		source := corev1ac.ConfigMapEnvSource().WithName(ref.Name)
		if ref.Optional != nil {
			source.WithOptional(*ref.Optional)
		}
		envVarFrom.WithConfigMapRef(source)
	}
	if ref := e.SecretRef; ref != nil {
		source := corev1ac.SecretEnvSource().WithName(ref.Name)
		if ref.Optional != nil {
			source.WithOptional(*ref.Optional)
		}
		envVarFrom.WithSecretRef(source)
	}
	return envVarFrom
}

func EnvFromSources(e ...corev1.EnvFromSource) []*corev1ac.EnvFromSourceApplyConfiguration {
	var envs []*corev1ac.EnvFromSourceApplyConfiguration
	for _, env := range e {
		envs = append(envs, EnvFromSource(env))
	}
	return envs
}

func Condition(c metav1.Condition) *metav1ac.ConditionApplyConfiguration {
	condition := metav1ac.Condition().
		WithObservedGeneration(c.ObservedGeneration)
	if c.Type != "" {
		condition.WithType(c.Type)
	}
	if c.Message != "" {
		condition.WithMessage(c.Message)
	}
	if c.Reason != "" {
		condition.WithReason(c.Reason)
	}
	if c.Status != "" {
		condition.WithStatus(c.Status)
	}
	if !c.LastTransitionTime.IsZero() {
		condition.WithLastTransitionTime(c.LastTransitionTime)
	}
	return condition
}

func Conditions(c ...metav1.Condition) []*metav1ac.ConditionApplyConfiguration {
	var conditions []*metav1ac.ConditionApplyConfiguration
	for _, condition := range c {
		conditions = append(conditions, Condition(condition))
	}
	return conditions
}

func Volume(v corev1.Volume) *corev1ac.VolumeApplyConfiguration {
	volume := corev1ac.Volume().WithName(v.Name)
	// FIXME
	return volume
}

func VolumeMount(m corev1.VolumeMount) *corev1ac.VolumeMountApplyConfiguration {
	volumeMount := corev1ac.VolumeMount().WithName(m.Name)
	if m.MountPath != "" {
		volumeMount.WithMountPath(m.MountPath)
	}
	return volumeMount
}

func VolumeMounts(v ...corev1.VolumeMount) []*corev1ac.VolumeMountApplyConfiguration {
	var mounts []*corev1ac.VolumeMountApplyConfiguration
	for _, mount := range v {
		mounts = append(mounts, VolumeMount(mount))
	}
	return mounts
}
