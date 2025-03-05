/*
Copyright 2024 The Kubeflow Authors.

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
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strconv"

	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/pkg/apply"
	"github.com/kubeflow/trainer/pkg/constants"
	"github.com/kubeflow/trainer/pkg/runtime"
	"github.com/kubeflow/trainer/pkg/runtime/framework"
)

type MPI struct {
	client client.Client
	scheme *apiruntime.Scheme
}

var _ framework.CustomValidationPlugin = (*MPI)(nil)
var _ framework.EnforceMLPolicyPlugin = (*MPI)(nil)
var _ framework.WatchExtensionPlugin = (*MPI)(nil)
var _ framework.ComponentBuilderPlugin = (*MPI)(nil)

const Name = "MPI"

// +kubebuilder:rbac:groups="",resources=secrets,verbs=create;get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=create;get;list;watch;update;patch

func New(_ context.Context, client client.Client, _ client.FieldIndexer) (framework.Plugin, error) {
	return &MPI{
		client: client,
		scheme: client.Scheme(),
	}, nil
}

func (m *MPI) Name() string {
	return Name
}

// TODO: Need to implement validations for MPI Policy.
func (m *MPI) Validate(oldObj, newObj *trainer.TrainJob) (admission.Warnings, field.ErrorList) {
	return nil, nil
}

func (m *MPI) EnforceMLPolicy(info *runtime.Info, trainJob *trainer.TrainJob) error {
	if info == nil || info.RuntimePolicy.MLPolicy == nil || info.RuntimePolicy.MLPolicy.MPI == nil {
		return nil
	}

	// TrainJob contains the actual information for the Trainer.
	numNodes := info.RuntimePolicy.MLPolicy.NumNodes
	if trainJob.Spec.Trainer != nil && trainJob.Spec.Trainer.NumNodes != nil {
		numNodes = trainJob.Spec.Trainer.NumNodes
	}
	info.Trainer.NumNodes = numNodes

	numProcPerNode := strconv.Itoa(int(*info.RuntimePolicy.MLPolicy.MPI.NumProcPerNode))
	if trainJob.Spec.Trainer != nil && trainJob.Spec.Trainer.NumProcPerNode != nil {
		numProcPerNode = (*trainJob.Spec.Trainer.NumProcPerNode).String()
	}
	info.Trainer.NumProcPerNode = numProcPerNode

	// Add Secret and ConfigMap volumes to the Info object
	info.Trainer.Volumes = []corev1ac.VolumeApplyConfiguration{
		*corev1ac.Volume().
			WithName(constants.MPISSHAuthVolumeName).
			WithSecret(corev1ac.SecretVolumeSource().
				WithSecretName(trainJob.Name+constants.MPISSHAuthSecretSuffix).
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
				)),
		*corev1ac.Volume().
			WithName(constants.MPIHostfileVolumeName).
			WithConfigMap(corev1ac.ConfigMapVolumeSource().
				WithName(trainJob.Name + constants.MPIHostfileConfigMapSuffix)),
	}

	info.Trainer.VolumeMounts = []corev1ac.VolumeMountApplyConfiguration{
		*corev1ac.VolumeMount().
			WithName(constants.MPISSHAuthVolumeName).
			WithMountPath(*info.RuntimePolicy.MLPolicy.MPI.SSHAuthMountPath),
		*corev1ac.VolumeMount().
			WithName(constants.MPIHostfileVolumeName).
			WithMountPath(constants.MPIHostfileDir),
	}

	// Update envs for Info object.
	// TODO (andreyvelich): Add validation to check that TrainJob doesn't have MPI envs.
	// TODO (andreyvelich): We should validate that envs from different plugins don't conflict with each other.
	// Ref: https://github.com/kubeflow/trainer/pull/2308#discussion_r1823229940
	// TODO (andreyvelich): Support other MPI implementations.

	if trainJob.Spec.Trainer != nil {
		info.Trainer.Env = apply.EnvVars(trainJob.Spec.Trainer.Env...)
	}

	switch *info.RuntimePolicy.MLPolicy.MPI.MPIImplementation {
	case trainer.MPIImplementationOpenMPI:
		apply.UpsertEnvVar(&info.Trainer.Env, corev1ac.EnvVar().
			WithName(constants.OpenMPIEnvHostFileLocation).
			WithValue(fmt.Sprintf("%s/%s", constants.MPIHostfileDir, constants.MPIHostfileName)))
	default:
		return fmt.Errorf("MPI implementation for %v doesn't supported", info.RuntimePolicy.MLPolicy.MPI.MPIImplementation)
	}

	// Add container port for the headless service.
	info.Trainer.ContainerPort = corev1ac.ContainerPort().
		WithContainerPort(constants.ContainerTrainerPort)

	// Update total Pod requests for the PodGroupPolicy plugin.
	for rName := range info.TotalRequests {
		// For other Jobs like the Initializer, replica is always equal to 1.
		// TODO (andreyvelich): Add support for total requests from the TrainJob's ResourcesPerNode.
		if rName == constants.JobTrainerNode {
			info.TotalRequests[rName] = runtime.TotalResourceRequest{
				Replicas:    ptr.Deref(numNodes, constants.DefaultJobReplicas),
				PodRequests: info.TotalRequests[rName].PodRequests,
			}
		}
	}

	return nil
}

func (m *MPI) ReconcilerBuilders() []runtime.ReconcilerBuilder {
	return []runtime.ReconcilerBuilder{
		func(b *builder.Builder, cl client.Client, cache cache.Cache) *builder.Builder {
			return b.Owns(&corev1.ConfigMap{})
		},
		func(b *builder.Builder, cl client.Client, cache cache.Cache) *builder.Builder {
			return b.Owns(&corev1.Secret{})
		},
	}
}

func (m *MPI) Build(_ context.Context, info *runtime.Info, trainJob *trainer.TrainJob) ([]any, error) {
	if info == nil || info.RuntimePolicy.MLPolicy == nil || info.RuntimePolicy.MLPolicy.MPI == nil {
		return nil, nil
	}

	secret, err := m.buildSSHAuthSecret(trainJob)
	if err != nil {
		return nil, fmt.Errorf("failed to build Secret with SSH auth keys. Error: %v", err)
	}

	configMap, err := m.buildHostFileConfigMap(info, trainJob)
	if err != nil {
		return nil, fmt.Errorf("failed to build ConfigMap with hostfile. Error: %v", err)
	}

	return []any{secret, configMap}, nil
}

func (m *MPI) buildSSHAuthSecret(trainJob *trainer.TrainJob) (*corev1ac.SecretApplyConfiguration, error) {
	// Generate SSH private and public keys.
	privateKey, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private SSH key, Error: %v", err)
	}

	privateDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to covert private SSH key to DER format. Error: %v", err)
	}

	privatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privateDER,
	})

	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate public SSH key. Error:  %v", err)
	}

	// Create Secret to store ssh keys.
	secret := corev1ac.Secret(trainJob.Name+constants.MPISSHAuthSecretSuffix, trainJob.Namespace).
		WithType(corev1.SecretTypeSSHAuth).
		WithData(map[string][]byte{
			corev1.SSHAuthPrivateKey:  privatePEM,
			constants.MPISSHPublicKey: ssh.MarshalAuthorizedKey(publicKey),
		})

	secret.WithOwnerReferences(metav1ac.OwnerReference().
		WithAPIVersion(trainer.GroupVersion.String()).
		WithKind(trainer.TrainJobKind).
		WithName(trainJob.Name).
		WithUID(trainJob.UID).
		WithController(true).
		WithBlockOwnerDeletion(true))

	return secret, nil
}

func (m *MPI) buildHostFileConfigMap(info *runtime.Info, trainJob *trainer.TrainJob) (*corev1ac.ConfigMapApplyConfiguration, error) {
	// Generate hostfile for the MPI communication.
	var hostfile bytes.Buffer
	// TODO (andreyvelich): Support other MPI implementations.
	for i := range *info.Trainer.NumNodes {
		switch *info.RuntimePolicy.MLPolicy.MPI.MPIImplementation {
		case trainer.MPIImplementationOpenMPI:
			// hostfile.WriteString(fmt.Sprintf("%s-%s-0-%s.%s.%s.svc slots=%s\n", trainJob.Name, constants.JobTrainerNode, i, trainJob.Name, trainJob.Namespace, info.NumProcPerNode))
			hostfile.WriteString(fmt.Sprintf("%s-%s-0-%d.%s slots=%s\n", trainJob.Name, constants.JobTrainerNode, i, info.NumProcPerNode, trainJob.Name))
		}
	}

	// Create ConfigMap to store hostfile.
	configMap := corev1ac.ConfigMap(trainJob.Name+constants.MPIHostfileConfigMapSuffix, trainJob.Namespace).
		WithData(map[string]string{
			constants.MPIHostfileName: hostfile.String(),
		})

	configMap.WithOwnerReferences(metav1ac.OwnerReference().
		WithAPIVersion(trainer.GroupVersion.String()).
		WithKind(trainer.TrainJobKind).
		WithName(trainJob.Name).
		WithUID(trainJob.UID).
		WithController(true).
		WithBlockOwnerDeletion(true))

	return configMap, nil
}
