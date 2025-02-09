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
	"maps"
	"strconv"

	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
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

// +kubebuilder:rbac:groups="",resources=secrets,verbs=create;get;list;update;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=create;get;list;update;watch

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
	info.Volumes = []corev1.Volume{
		{
			Name: constants.MPISSHAuthVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: trainJob.Name + constants.MPISSHAuthSecretSuffix,
					Items: []corev1.KeyToPath{
						{
							Key:  corev1.SSHAuthPrivateKey,
							Path: constants.MPISSHPrivateKeyFile,
						},
						{
							Key:  constants.MPISSHPublicKey,
							Path: constants.MPISSHPublicKeyFile,
						},
						{
							Key:  constants.MPISSHPublicKey,
							Path: constants.MPISSHAuthorizedKeys,
						},
					},
				},
			},
		},
		{
			Name: constants.MPIHostfileVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: trainJob.Name + constants.MPIHostfileConfigMapSuffix,
					},
				},
			},
		},
	}
	info.VolumeMounts = []corev1.VolumeMount{
		{
			Name:      constants.MPISSHAuthVolumeName,
			MountPath: info.RuntimePolicy.MLPolicy.MPI.SSHAuthMountPath,
		},
		{
			Name:      constants.MPIHostfileVolumeName,
			MountPath: constants.MPIHostfileDir,
		},
	}

	// Update envs for Info object.
	// TODO (andreyvelich): Add validation to check that TrainJob doesn't have MPI envs.
	// TODO (andreyvelich): We should validate that envs from different plugins don't conflict with each other.
	// Ref: https://github.com/kubeflow/trainer/pull/2308#discussion_r1823229940
	// TODO (andreyvelich): Support other MPI implementations.
	infoEnvs := []corev1.EnvVar{}
	switch info.RuntimePolicy.MLPolicy.MPI.MPIImplementation {
	case trainer.MPIImplementationOpenMPI:
		infoEnvs = append(infoEnvs, []corev1.EnvVar{
			{
				Name:  constants.OpenMPIEnvHostFileLocation,
				Value: fmt.Sprintf("%s/%s", constants.MPIHostfileDir, constants.MPIHostfileName),
			}}...)
	default:
		return fmt.Errorf("MPI implementation for %s doesn't supported", info.RuntimePolicy.MLPolicy.MPI.MPIImplementation)
	}

	// Set for all Info envs.
	envNames := sets.New[string]()
	for _, env := range infoEnvs {
		envNames.Insert(env.Name)
	}
	// Info envs take precedence over TrainJob envs.
	if trainJob.Spec.Trainer != nil {
		for _, env := range trainJob.Spec.Trainer.Env {
			if !envNames.Has(env.Name) {
				info.Trainer.Env = append(info.Trainer.Env, corev1.EnvVar{Name: env.Name, Value: env.Value})
			}
		}
	}

	// Insert MPI distributed envs into the list end.
	info.Trainer.Env = append(info.Trainer.Env, infoEnvs...)

	// Add container port for the headless service.
	info.Trainer.ContainerPort = &corev1.ContainerPort{
		ContainerPort: constants.ContainerTrainerPort,
	}

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

func (m *MPI) Build(ctx context.Context, info *runtime.Info, trainJob *trainer.TrainJob) ([]any, error) {
	if info == nil || info.RuntimePolicy.MLPolicy == nil || info.RuntimePolicy.MLPolicy.MPI == nil {
		return nil, nil
	}

	secret, err := m.buildSSHAuthSecret(ctx, trainJob)
	if err != nil {
		return nil, fmt.Errorf("failed to build Secret with SSH auth keys. Error: %v", err)
	}

	configMap, err := m.buildHostFileConfigMap(ctx, info, trainJob)
	if err != nil {
		return nil, fmt.Errorf("failed to build ConfigMap with hostfile. Error: %v", err)
	}

	return []any{secret, configMap}, nil
}

func (m *MPI) buildSSHAuthSecret(ctx context.Context, trainJob *trainer.TrainJob) (*corev1.Secret, error) {
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
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      trainJob.Name + constants.MPISSHAuthSecretSuffix,
			Namespace: trainJob.Namespace,
		},
		Type: corev1.SecretTypeSSHAuth,
		Data: map[string][]byte{
			corev1.SSHAuthPrivateKey:  privatePEM,
			constants.MPISSHPublicKey: ssh.MarshalAuthorizedKey(publicKey),
		},
	}
	if err := ctrlutil.SetControllerReference(trainJob, secret, m.scheme); err != nil {
		return nil, err
	}
	oldSecret := &corev1.Secret{}
	if err := m.client.Get(ctx, client.ObjectKeyFromObject(secret), oldSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		oldSecret = nil
	}
	if needsCreateOrUpdateSecret(oldSecret, secret, ptr.Deref(trainJob.Spec.Suspend, false)) {
		return secret, nil
	}
	return nil, nil
}

func needsCreateOrUpdateSecret(old, new *corev1.Secret, trainJobIsSuspended bool) bool {
	return old == nil ||
		trainJobIsSuspended &&
			(!equality.Semantic.DeepEqual(old.Data, new.Data) || !maps.Equal(old.Labels, new.Labels) || !maps.Equal(old.Annotations, new.Annotations))
}

func (m *MPI) buildHostFileConfigMap(ctx context.Context, info *runtime.Info, trainJob *trainer.TrainJob) (*corev1.ConfigMap, error) {
	// Generate hostfile for the MPI communication.
	var hostfile bytes.Buffer
	// TODO (andreyvelich): Support other MPI implementations.
	for i := range *info.Trainer.NumNodes {
		switch info.RuntimePolicy.MLPolicy.MPI.MPIImplementation {
		case trainer.MPIImplementationOpenMPI:
			// hostfile.WriteString(fmt.Sprintf("%s-%s-0-%s.%s.%s.svc slots=%s\n", trainJob.Name, constants.JobTrainerNode, i, trainJob.Name, trainJob.Namespace, info.NumProcPerNode))
			hostfile.WriteString(fmt.Sprintf("%s-%s-0-%d.%s slots=%s\n", trainJob.Name, constants.JobTrainerNode, i, info.NumProcPerNode, trainJob.Name))
		}
	}

	// Create ConfigMap to store hostfile.
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      trainJob.Name + constants.MPIHostfileConfigMapSuffix,
			Namespace: trainJob.Namespace,
		},
		Data: map[string]string{
			constants.MPIHostfileName: hostfile.String(),
		},
	}
	if err := ctrlutil.SetControllerReference(trainJob, configMap, m.scheme); err != nil {
		return nil, err
	}
	oldConfigMap := &corev1.ConfigMap{}
	if err := m.client.Get(ctx, client.ObjectKeyFromObject(configMap), oldConfigMap); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		oldConfigMap = nil
	}
	if needsCreateOrUpdateConfigMap(oldConfigMap, configMap, ptr.Deref(trainJob.Spec.Suspend, false)) {
		return configMap, nil
	}
	return nil, nil
}

func needsCreateOrUpdateConfigMap(old, new *corev1.ConfigMap, trainJobIsSuspended bool) bool {
	return old == nil ||
		trainJobIsSuspended &&
			(!equality.Semantic.DeepEqual(old.Data, new.Data) || !maps.Equal(old.Labels, new.Labels) || !maps.Equal(old.Annotations, new.Annotations))
}
