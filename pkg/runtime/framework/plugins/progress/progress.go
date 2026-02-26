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

package progress

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/apply"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	progresspkg "github.com/kubeflow/trainer/v2/pkg/progress"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
	utilruntime "github.com/kubeflow/trainer/v2/pkg/util/runtime"
)

const (
	Name = "Progress"

	// Environment variable names
	envNameStatusURL = "KUBEFLOW_TRAINER_STATUS_URL"
	envNameCACert    = "KUBEFLOW_TRAINER_STATUS_CA_CERT"
	envNameToken     = "KUBEFLOW_TRAINER_STATUS_TOKEN"

	// Volume and mount configuration
	progressMountPath = "/var/run/secrets/kubeflow/trainer"
	caCertFileName    = "ca.crt"
	tokenFileName     = "token"
	tokenVolumeName   = "kubeflow-trainer-token"

	// Service account token configuration
	tokenExpirySeconds = 3600

	// Server tls config
	caCertKey = "ca.crt"
)

type Progress struct {
	client client.Client
	cfg    *configapi.Configuration
}

var _ framework.ComponentBuilderPlugin = (*Progress)(nil)
var _ framework.EnforceMLPolicyPlugin = (*Progress)(nil)

func New(_ context.Context, c client.Client, _ client.FieldIndexer, cfg *configapi.Configuration) (framework.Plugin, error) {
	return &Progress{client: c, cfg: cfg}, nil
}

func (p *Progress) Name() string {
	return Name
}

func (p *Progress) EnforceMLPolicy(info *runtime.Info, trainJob *trainer.TrainJob) error {
	if info == nil || trainJob == nil {
		return nil
	}

	// Add label to identify which TrainJob the pod belongs to
	if info.Scheduler == nil {
		info.Scheduler = &runtime.Scheduler{}
	}
	if info.Scheduler.PodLabels == nil {
		info.Scheduler.PodLabels = make(map[string]string)
	}
	info.Scheduler.PodLabels[progresspkg.LabelTrainJobName] = trainJob.Name

	envVars, err := p.createEnvVars(trainJob)
	if err != nil {
		return err
	}
	volumeMount := createTokenVolumeMount()
	volume := createTokenVolume(trainJob)

	// Inject into all trainer containers
	trainerPS := info.FindPodSetByAncestor(constants.AncestorTrainer)
	if trainerPS != nil {
		for i := range trainerPS.Containers {
			apply.UpsertEnvVars(&trainerPS.Containers[i].Env, envVars...)
			apply.UpsertVolumeMounts(&trainerPS.Containers[i].VolumeMounts, volumeMount)
		}
		apply.UpsertVolumes(&trainerPS.Volumes, volume)
	}

	return nil
}

func (p *Progress) Build(ctx context.Context, info *runtime.Info, trainJob *trainer.TrainJob) ([]apiruntime.ApplyConfiguration, error) {
	if info == nil || trainJob == nil {
		return nil, nil
	}

	configMap, err := p.buildProgressServerCaCrtConfigMap(ctx, trainJob)
	if err != nil {
		return nil, err
	}

	return []apiruntime.ApplyConfiguration{configMap}, nil
}

func (p *Progress) createEnvVars(trainJob *trainer.TrainJob) ([]corev1ac.EnvVarApplyConfiguration, error) {
	if p.cfg.ProgressServer.Port == nil {
		return nil, fmt.Errorf("missing progress server port")
	}
	// TODO: consider renaming the CertManagement.WebhookServiceName name?
	svc := fmt.Sprintf("https://%s.%s.svc:%d", p.cfg.CertManagement.WebhookServiceName, utilruntime.GetOperatorNamespace(), *p.cfg.ProgressServer.Port)
	path := progresspkg.StatusUrl(trainJob.Namespace, trainJob.Name)
	statusURL := svc + path

	return []corev1ac.EnvVarApplyConfiguration{
		*corev1ac.EnvVar().
			WithName(envNameStatusURL).
			WithValue(statusURL),
		*corev1ac.EnvVar().
			WithName(envNameCACert).
			WithValue(fmt.Sprintf("%s/%s", progressMountPath, caCertFileName)),
		*corev1ac.EnvVar().
			WithName(envNameToken).
			WithValue(fmt.Sprintf("%s/%s", progressMountPath, tokenFileName)),
	}, nil
}

func createTokenVolumeMount() corev1ac.VolumeMountApplyConfiguration {
	return *corev1ac.VolumeMount().
		WithName(tokenVolumeName).
		WithMountPath(progressMountPath).
		WithReadOnly(true)
}

func createTokenVolume(trainJob *trainer.TrainJob) corev1ac.VolumeApplyConfiguration {
	configMapName := fmt.Sprintf("%s-tls-config", trainJob.Name)

	return *corev1ac.Volume().
		WithName(tokenVolumeName).
		WithProjected(
			corev1ac.ProjectedVolumeSource().
				WithSources(
					corev1ac.VolumeProjection().
						WithServiceAccountToken(
							corev1ac.ServiceAccountTokenProjection().
								WithAudience(progresspkg.TokenAudience).
								WithExpirationSeconds(tokenExpirySeconds).
								WithPath(tokenFileName),
						),
					corev1ac.VolumeProjection().
						WithConfigMap(
							corev1ac.ConfigMapProjection().
								WithName(configMapName).
								WithItems(
									corev1ac.KeyToPath().
										WithKey(caCertKey).
										WithPath(caCertFileName),
								),
						),
				),
		)
}

// buildProgressServerCaCrtConfigMap creates a ConfigMap that will copy the ca.crt from the webhook secret
func (p *Progress) buildProgressServerCaCrtConfigMap(ctx context.Context, trainJob *trainer.TrainJob) (*corev1ac.ConfigMapApplyConfiguration, error) {
	configMapName := fmt.Sprintf("%s-tls-config", trainJob.Name)

	// Get the CA cert from the webhook secret
	secret := &corev1.Secret{}
	secretKey := client.ObjectKey{
		Namespace: utilruntime.GetOperatorNamespace(),
		Name:      p.cfg.CertManagement.WebhookSecretName,
	}

	var caCertData string
	if err := p.client.Get(ctx, secretKey, secret); err == nil {
		if caCert, ok := secret.Data[caCertKey]; ok && len(caCert) > 0 {
			caCertData = string(caCert)
		} else {
			return nil, fmt.Errorf("failed to find progress server ca.crt in tls secret")
		}
	} else {
		return nil, fmt.Errorf("failed to look up progress server tls secret: %w", err)
	}

	configMap := corev1ac.ConfigMap(configMapName, trainJob.Namespace).
		WithData(map[string]string{
			caCertKey: caCertData,
		}).
		WithOwnerReferences(
			metav1ac.OwnerReference().
				WithAPIVersion(trainer.GroupVersion.String()).
				WithKind(trainer.TrainJobKind).
				WithName(trainJob.Name).
				WithUID(trainJob.UID).
				WithController(true).
				WithBlockOwnerDeletion(true),
		)

	return configMap, nil
}
