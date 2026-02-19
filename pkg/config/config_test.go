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

package config

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	componentconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	runtimeconfig "sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
)

func TestLoad(t *testing.T) {
	testScheme := runtime.NewScheme()
	if err := configapi.AddToScheme(testScheme); err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()

	emptyConfig := filepath.Join(tmpDir, "empty.yaml")
	if err := os.WriteFile(emptyConfig, []byte(`
apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
`), os.FileMode(0600)); err != nil {
		t.Fatal(err)
	}

	customOverwriteConfig := filepath.Join(tmpDir, "custom-overwrite.yaml")
	if err := os.WriteFile(customOverwriteConfig, []byte(`
apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
health:
  healthProbeBindAddress: :8082
metrics:
  bindAddress: :9443
webhook:
  port: 9444
clientConnection:
  qps: 100
  burst: 200
`), os.FileMode(0600)); err != nil {
		t.Fatal(err)
	}

	certManagementCustomConfig := filepath.Join(tmpDir, "cert-custom.yaml")
	if err := os.WriteFile(certManagementCustomConfig, []byte(`
apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
certManagement:
  enable: true
  webhookServiceName: custom-webhook-service
  webhookSecretName: custom-webhook-secret
`), os.FileMode(0600)); err != nil {
		t.Fatal(err)
	}

	certManagementDisabledConfig := filepath.Join(tmpDir, "cert-disabled.yaml")
	if err := os.WriteFile(certManagementDisabledConfig, []byte(`
apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
certManagement:
  enable: false
`), os.FileMode(0600)); err != nil {
		t.Fatal(err)
	}

	leaderElectionConfig := filepath.Join(tmpDir, "leader-election.yaml")
	if err := os.WriteFile(leaderElectionConfig, []byte(`
apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
leaderElection:
  leaderElect: true
  resourceName: trainer-leader
  resourceNamespace: kubeflow
  resourceLock: leases
  leaseDuration: 15s
  renewDeadline: 10s
  retryPeriod: 2s
`), os.FileMode(0600)); err != nil {
		t.Fatal(err)
	}

	controllerConcurrencyConfig := filepath.Join(tmpDir, "controller-concurrency.yaml")
	if err := os.WriteFile(controllerConcurrencyConfig, []byte(`
apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
controller:
  groupKindConcurrency:
    TrainJob.trainer.kubeflow.org: 10
    TrainingRuntime.trainer.kubeflow.org: 5
    ClusterTrainingRuntime.trainer.kubeflow.org: 3
`), os.FileMode(0600)); err != nil {
		t.Fatal(err)
	}

	insecureMetricsConfig := filepath.Join(tmpDir, "insecure-metrics.yaml")
	if err := os.WriteFile(insecureMetricsConfig, []byte(`
apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
metrics:
  bindAddress: :8080
  secureServing: false
`), os.FileMode(0600)); err != nil {
		t.Fatal(err)
	}

	webhookHostConfig := filepath.Join(tmpDir, "webhook-host.yaml")
	if err := os.WriteFile(webhookHostConfig, []byte(`
apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
webhook:
  port: 9443
  host: localhost
`), os.FileMode(0600)); err != nil {
		t.Fatal(err)
	}

	healthConfig := filepath.Join(tmpDir, "health.yaml")
	if err := os.WriteFile(healthConfig, []byte(`
apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
health:
  healthProbeBindAddress: :9090
  readinessEndpointName: ready
  livenessEndpointName: alive
`), os.FileMode(0600)); err != nil {
		t.Fatal(err)
	}

	progressServerConfig := filepath.Join(tmpDir, "progress.yaml")
	if err := os.WriteFile(progressServerConfig, []byte(`
apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
progressServer:
  port: 12443
  qps: 1
  burst: 2
`), os.FileMode(0600)); err != nil {
		t.Fatal(err)
	}

	completeConfig := filepath.Join(tmpDir, "complete.yaml")
	if err := os.WriteFile(completeConfig, []byte(`
apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
webhook:
  port: 9443
  host: 0.0.0.0
metrics:
  bindAddress: :8443
  secureServing: true
health:
  healthProbeBindAddress: :8081
  readinessEndpointName: readyz
  livenessEndpointName: healthz
leaderElection:
  leaderElect: true
  resourceName: trainer.kubeflow.org
  resourceNamespace: kubeflow
  resourceLock: leases
  leaseDuration: 15s
  renewDeadline: 10s
  retryPeriod: 2s
controller:
  groupKindConcurrency:
    TrainJob.trainer.kubeflow.org: 5
    TrainingRuntime.trainer.kubeflow.org: 1
certManagement:
  enable: true
  webhookServiceName: kubeflow-trainer-controller-manager
  webhookSecretName: kubeflow-trainer-webhook-cert
clientConnection:
  qps: 50
  burst: 100
progressServer:
  port: 12443
  qps: 1
  burst: 2
`), os.FileMode(0600)); err != nil {
		t.Fatal(err)
	}

	wrongAPIVersionConfig := filepath.Join(tmpDir, "wrong-apiversion.yaml")
	if err := os.WriteFile(wrongAPIVersionConfig, []byte(`
apiVersion: config.wrong.group/v1
kind: Configuration
`), os.FileMode(0600)); err != nil {
		t.Fatal(err)
	}

	wrongKindConfig := filepath.Join(tmpDir, "wrong-kind.yaml")
	if err := os.WriteFile(wrongKindConfig, []byte(`
apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: WrongKind
`), os.FileMode(0600)); err != nil {
		t.Fatal(err)
	}

	unknownFieldsConfig := filepath.Join(tmpDir, "unknown-fields.yaml")
	if err := os.WriteFile(unknownFieldsConfig, []byte(`
apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
unknownField: value
webhook:
  port: 9443
  unknownWebhookField: value
`), os.FileMode(0600)); err != nil {
		t.Fatal(err)
	}

	invalidWebhookPortConfig := filepath.Join(tmpDir, "invalid-port.yaml")
	if err := os.WriteFile(invalidWebhookPortConfig, []byte(`
apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
webhook:
  port: 99999
`), os.FileMode(0600)); err != nil {
		t.Fatal(err)
	}

	negativeQPSConfig := filepath.Join(tmpDir, "negative-qps.yaml")
	if err := os.WriteFile(negativeQPSConfig, []byte(`
apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
clientConnection:
  qps: -10
`), os.FileMode(0600)); err != nil {
		t.Fatal(err)
	}

	malformedYAMLConfig := filepath.Join(tmpDir, "malformed.yaml")
	if err := os.WriteFile(malformedYAMLConfig, []byte(`
this is not: valid: yaml: content
`), os.FileMode(0600)); err != nil {
		t.Fatal(err)
	}

	// Default expected values.
	typeMeta := metav1.TypeMeta{
		APIVersion: configapi.GroupVersion.String(),
		Kind:       "Configuration",
	}

	defaultCertManagement := &configapi.CertManagement{
		Enable:             ptr.To(true),
		WebhookServiceName: "kubeflow-trainer-controller-manager",
		WebhookSecretName:  "kubeflow-trainer-webhook-cert",
	}

	defaultClientConnection := &configapi.ClientConnection{
		QPS:   ptr.To[float32](50),
		Burst: ptr.To[int32](100),
	}

	defaultProgressServer := &configapi.ProgressServer{
		Port:  ptr.To[int32](10443),
		QPS:   ptr.To[float32](5),
		Burst: ptr.To[int32](10),
	}

	defaultWebhook := configapi.ControllerWebhook{
		Port: ptr.To[int32](9443),
	}

	defaultMetrics := configapi.ControllerMetrics{
		BindAddress:   ":8443",
		SecureServing: ptr.To(true),
	}

	defaultHealth := configapi.ControllerHealth{
		HealthProbeBindAddress: ":8081",
		ReadinessEndpointName:  "readyz",
		LivenessEndpointName:   "healthz",
	}

	defaultOptions := ctrl.Options{
		HealthProbeBindAddress: ":8081",
		Metrics: metricsserver.Options{
			BindAddress:   ":8443",
			SecureServing: true,
		},
		WebhookServer: &webhook.DefaultServer{
			Options: webhook.Options{
				Port: 9443,
			},
		},
	}

	// Comparison options.
	ctrlOptsCmpOpts := []cmp.Option{
		cmpopts.IgnoreUnexported(ctrl.Options{}, webhook.DefaultServer{}, net.ListenConfig{}),
		cmpopts.IgnoreFields(ctrl.Options{}, "Scheme", "Logger", "Cache"),
		cmpopts.IgnoreFields(runtimeconfig.Controller{}, "Logger"),
		cmpopts.IgnoreFields(metricsserver.Options{}, "TLSOpts"),
		cmpopts.IgnoreFields(webhook.Options{}, "TLSOpts"),
	}

	testcases := []struct {
		name              string
		configFile        string
		wantConfiguration configapi.Configuration
		wantOptions       ctrl.Options
		wantErr           bool
	}{
		{
			name:       "default config",
			configFile: "",
			wantConfiguration: configapi.Configuration{
				Webhook:          defaultWebhook,
				Metrics:          defaultMetrics,
				Health:           defaultHealth,
				CertManagement:   defaultCertManagement,
				ClientConnection: defaultClientConnection,
				ProgressServer:   defaultProgressServer,
			},
			wantOptions: defaultOptions,
		},
		{
			name:       "empty config file",
			configFile: emptyConfig,
			wantConfiguration: configapi.Configuration{
				TypeMeta:         typeMeta,
				Webhook:          defaultWebhook,
				Metrics:          defaultMetrics,
				Health:           defaultHealth,
				CertManagement:   defaultCertManagement,
				ClientConnection: defaultClientConnection,
				ProgressServer:   defaultProgressServer,
			},
			wantOptions: defaultOptions,
		},
		{
			name:       "bad path",
			configFile: ".",
			wantErr:    true,
		},
		{
			name:       "nonexistent file",
			configFile: "/nonexistent/file.yaml",
			wantErr:    true,
		},
		{
			name:       "malformed YAML",
			configFile: malformedYAMLConfig,
			wantErr:    true,
		},
		{
			name:       "custom overwrite config",
			configFile: customOverwriteConfig,
			wantConfiguration: configapi.Configuration{
				TypeMeta: typeMeta,
				Webhook: configapi.ControllerWebhook{
					Port: ptr.To[int32](9444),
				},
				Metrics: configapi.ControllerMetrics{
					BindAddress:   ":9443",
					SecureServing: ptr.To(true),
				},
				Health: configapi.ControllerHealth{
					HealthProbeBindAddress: ":8082",
					ReadinessEndpointName:  "readyz",
					LivenessEndpointName:   "healthz",
				},
				CertManagement: defaultCertManagement,
				ClientConnection: &configapi.ClientConnection{
					QPS:   ptr.To[float32](100),
					Burst: ptr.To[int32](200),
				},
				ProgressServer: defaultProgressServer,
			},
			wantOptions: ctrl.Options{
				HealthProbeBindAddress: ":8082",
				Metrics: metricsserver.Options{
					BindAddress:   ":9443",
					SecureServing: true,
				},
				WebhookServer: &webhook.DefaultServer{
					Options: webhook.Options{
						Port: 9444,
					},
				},
			},
		},
		{
			name:       "leader election config",
			configFile: leaderElectionConfig,
			wantConfiguration: configapi.Configuration{
				TypeMeta:         typeMeta,
				Webhook:          defaultWebhook,
				Metrics:          defaultMetrics,
				Health:           defaultHealth,
				CertManagement:   defaultCertManagement,
				ClientConnection: defaultClientConnection,
				ProgressServer:   defaultProgressServer,
				LeaderElection: &componentconfigv1alpha1.LeaderElectionConfiguration{
					LeaderElect:       ptr.To(true),
					ResourceName:      "trainer-leader",
					ResourceNamespace: "kubeflow",
					ResourceLock:      "leases",
					LeaseDuration:     metav1.Duration{Duration: 15 * time.Second},
					RenewDeadline:     metav1.Duration{Duration: 10 * time.Second},
					RetryPeriod:       metav1.Duration{Duration: 2 * time.Second},
				},
			},
			wantOptions: ctrl.Options{
				HealthProbeBindAddress: ":8081",
				Metrics: metricsserver.Options{
					BindAddress:   ":8443",
					SecureServing: true,
				},
				WebhookServer: &webhook.DefaultServer{
					Options: webhook.Options{
						Port: 9443,
					},
				},
				LeaderElection:             true,
				LeaderElectionID:           "trainer-leader",
				LeaderElectionNamespace:    "kubeflow",
				LeaderElectionResourceLock: "leases",
				LeaseDuration:              ptr.To(15 * time.Second),
				RenewDeadline:              ptr.To(10 * time.Second),
				RetryPeriod:                ptr.To(2 * time.Second),
			},
		},
		{
			name:       "controller concurrency config",
			configFile: controllerConcurrencyConfig,
			wantConfiguration: configapi.Configuration{
				TypeMeta:         typeMeta,
				Webhook:          defaultWebhook,
				Metrics:          defaultMetrics,
				Health:           defaultHealth,
				CertManagement:   defaultCertManagement,
				ClientConnection: defaultClientConnection,
				ProgressServer:   defaultProgressServer,
				Controller: &configapi.ControllerConfigurationSpec{
					GroupKindConcurrency: map[string]int32{
						"TrainJob.trainer.kubeflow.org":               10,
						"TrainingRuntime.trainer.kubeflow.org":        5,
						"ClusterTrainingRuntime.trainer.kubeflow.org": 3,
					},
				},
			},
			wantOptions: ctrl.Options{
				HealthProbeBindAddress: ":8081",
				Metrics: metricsserver.Options{
					BindAddress:   ":8443",
					SecureServing: true,
				},
				WebhookServer: &webhook.DefaultServer{
					Options: webhook.Options{
						Port: 9443,
					},
				},
				Controller: runtimeconfig.Controller{
					GroupKindConcurrency: map[string]int{
						"TrainJob.trainer.kubeflow.org":               10,
						"TrainingRuntime.trainer.kubeflow.org":        5,
						"ClusterTrainingRuntime.trainer.kubeflow.org": 3,
					},
				},
			},
		},
		{
			name:       "cert management with custom names",
			configFile: certManagementCustomConfig,
			wantConfiguration: configapi.Configuration{
				TypeMeta:         typeMeta,
				Webhook:          defaultWebhook,
				Metrics:          defaultMetrics,
				Health:           defaultHealth,
				ClientConnection: defaultClientConnection,
				ProgressServer:   defaultProgressServer,
				CertManagement: &configapi.CertManagement{
					Enable:             ptr.To(true),
					WebhookServiceName: "custom-webhook-service",
					WebhookSecretName:  "custom-webhook-secret",
				},
			},
			wantOptions: defaultOptions,
		},
		{
			name:       "cert management disabled",
			configFile: certManagementDisabledConfig,
			wantConfiguration: configapi.Configuration{
				TypeMeta:         typeMeta,
				Webhook:          defaultWebhook,
				Metrics:          defaultMetrics,
				Health:           defaultHealth,
				ClientConnection: defaultClientConnection,
				ProgressServer:   defaultProgressServer,
				CertManagement: &configapi.CertManagement{
					Enable:             ptr.To(false),
					WebhookServiceName: "kubeflow-trainer-controller-manager",
					WebhookSecretName:  "kubeflow-trainer-webhook-cert",
				},
			},
			wantOptions: defaultOptions,
		},
		{
			name:       "insecure metrics config",
			configFile: insecureMetricsConfig,
			wantConfiguration: configapi.Configuration{
				TypeMeta: typeMeta,
				Webhook:  defaultWebhook,
				Metrics: configapi.ControllerMetrics{
					BindAddress:   ":8080",
					SecureServing: ptr.To(false),
				},
				Health:           defaultHealth,
				CertManagement:   defaultCertManagement,
				ClientConnection: defaultClientConnection,
				ProgressServer:   defaultProgressServer,
			},
			wantOptions: ctrl.Options{
				HealthProbeBindAddress: ":8081",
				Metrics: metricsserver.Options{
					BindAddress:   ":8080",
					SecureServing: false,
				},
				WebhookServer: &webhook.DefaultServer{
					Options: webhook.Options{
						Port: 9443,
					},
				},
			},
		},
		{
			name:       "webhook host config",
			configFile: webhookHostConfig,
			wantConfiguration: configapi.Configuration{
				TypeMeta: typeMeta,
				Webhook: configapi.ControllerWebhook{
					Port: ptr.To[int32](9443),
					Host: ptr.To("localhost"),
				},
				Metrics:          defaultMetrics,
				Health:           defaultHealth,
				CertManagement:   defaultCertManagement,
				ClientConnection: defaultClientConnection,
				ProgressServer:   defaultProgressServer,
			},
			wantOptions: ctrl.Options{
				HealthProbeBindAddress: ":8081",
				Metrics: metricsserver.Options{
					BindAddress:   ":8443",
					SecureServing: true,
				},
				WebhookServer: &webhook.DefaultServer{
					Options: webhook.Options{
						Port: 9443,
						Host: "localhost",
					},
				},
			},
		},
		{
			name:       "progress server config",
			configFile: progressServerConfig,
			wantConfiguration: configapi.Configuration{
				TypeMeta:         typeMeta,
				Webhook:          defaultWebhook,
				Metrics:          defaultMetrics,
				Health:           defaultHealth,
				CertManagement:   defaultCertManagement,
				ClientConnection: defaultClientConnection,
				ProgressServer: &configapi.ProgressServer{
					Port:  ptr.To[int32](12443),
					QPS:   ptr.To[float32](1),
					Burst: ptr.To[int32](2),
				},
			},
			wantOptions: ctrl.Options{
				HealthProbeBindAddress: ":8081",
				Metrics: metricsserver.Options{
					BindAddress:   ":8443",
					SecureServing: true,
				},
				WebhookServer: &webhook.DefaultServer{
					Options: webhook.Options{
						Port: 9443,
					},
				},
			},
		},
		{
			name:       "health config with custom endpoints",
			configFile: healthConfig,
			wantConfiguration: configapi.Configuration{
				TypeMeta: typeMeta,
				Webhook:  defaultWebhook,
				Metrics:  defaultMetrics,
				Health: configapi.ControllerHealth{
					HealthProbeBindAddress: ":9090",
					ReadinessEndpointName:  "ready",
					LivenessEndpointName:   "alive",
				},
				CertManagement:   defaultCertManagement,
				ClientConnection: defaultClientConnection,
				ProgressServer:   defaultProgressServer,
			},
			wantOptions: ctrl.Options{
				HealthProbeBindAddress: ":9090",
				Metrics: metricsserver.Options{
					BindAddress:   ":8443",
					SecureServing: true,
				},
				WebhookServer: &webhook.DefaultServer{
					Options: webhook.Options{
						Port: 9443,
					},
				},
			},
		},
		{
			name:       "complete configuration",
			configFile: completeConfig,
			wantConfiguration: configapi.Configuration{
				TypeMeta: typeMeta,
				Webhook: configapi.ControllerWebhook{
					Port: ptr.To[int32](9443),
					Host: ptr.To("0.0.0.0"),
				},
				Metrics: defaultMetrics,
				Health:  defaultHealth,
				LeaderElection: &componentconfigv1alpha1.LeaderElectionConfiguration{
					LeaderElect:       ptr.To(true),
					ResourceName:      "trainer.kubeflow.org",
					ResourceNamespace: "kubeflow",
					ResourceLock:      "leases",
					LeaseDuration:     metav1.Duration{Duration: 15 * time.Second},
					RenewDeadline:     metav1.Duration{Duration: 10 * time.Second},
					RetryPeriod:       metav1.Duration{Duration: 2 * time.Second},
				},
				Controller: &configapi.ControllerConfigurationSpec{
					GroupKindConcurrency: map[string]int32{
						"TrainJob.trainer.kubeflow.org":        5,
						"TrainingRuntime.trainer.kubeflow.org": 1,
					},
				},
				CertManagement:   defaultCertManagement,
				ClientConnection: defaultClientConnection,
				ProgressServer: &configapi.ProgressServer{
					Port:  ptr.To[int32](12443),
					QPS:   ptr.To[float32](1),
					Burst: ptr.To[int32](2),
				},
			},
			wantOptions: ctrl.Options{
				HealthProbeBindAddress: ":8081",
				Metrics: metricsserver.Options{
					BindAddress:   ":8443",
					SecureServing: true,
				},
				WebhookServer: &webhook.DefaultServer{
					Options: webhook.Options{
						Port: 9443,
						Host: "0.0.0.0",
					},
				},
				LeaderElection:             true,
				LeaderElectionID:           "trainer.kubeflow.org",
				LeaderElectionNamespace:    "kubeflow",
				LeaderElectionResourceLock: "leases",
				LeaseDuration:              ptr.To(15 * time.Second),
				RenewDeadline:              ptr.To(10 * time.Second),
				RetryPeriod:                ptr.To(2 * time.Second),
				Controller: runtimeconfig.Controller{
					GroupKindConcurrency: map[string]int{
						"TrainJob.trainer.kubeflow.org":        5,
						"TrainingRuntime.trainer.kubeflow.org": 1,
					},
				},
			},
		},
		{
			name:       "wrong API version",
			configFile: wrongAPIVersionConfig,
			wantErr:    true,
		},
		{
			name:       "wrong kind",
			configFile: wrongKindConfig,
			wantErr:    true,
		},
		{
			name:       "unknown fields",
			configFile: unknownFieldsConfig,
			wantErr:    true,
		},
		{
			name:       "invalid webhook port",
			configFile: invalidWebhookPortConfig,
			wantErr:    true,
		},
		{
			name:       "negative QPS",
			configFile: negativeQPSConfig,
			wantErr:    true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			options, cfg, err := Load(testScheme, tc.configFile, false)
			if tc.wantErr {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.wantConfiguration, cfg); diff != "" {
				t.Errorf("Unexpected config (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantOptions, options, ctrlOptsCmpOpts...); diff != "" {
				t.Errorf("Unexpected options (-want +got):\n%s", diff)
			}
		})
	}
}

func TestIsCertManagementEnabled(t *testing.T) {
	testcases := []struct {
		name string
		cfg  configapi.Configuration
		want bool
	}{
		{
			name: "CertManagement is nil",
			cfg:  configapi.Configuration{},
			want: true,
		},
		{
			name: "CertManagement.Enable is nil",
			cfg: configapi.Configuration{
				CertManagement: &configapi.CertManagement{},
			},
			want: true,
		},
		{
			name: "CertManagement.Enable is true",
			cfg: configapi.Configuration{
				CertManagement: &configapi.CertManagement{
					Enable: ptr.To(true),
				},
			},
			want: true,
		},
		{
			name: "CertManagement.Enable is false",
			cfg: configapi.Configuration{
				CertManagement: &configapi.CertManagement{
					Enable: ptr.To(false),
				},
			},
			want: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsCertManagementEnabled(&tc.cfg)
			if got != tc.want {
				t.Errorf("IsCertManagementEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLoadHTTP2(t *testing.T) {
	testScheme := runtime.NewScheme()
	if err := configapi.AddToScheme(testScheme); err != nil {
		t.Fatal(err)
	}

	testcases := []struct {
		name        string
		enableHTTP2 bool
		wantTLSOpts bool
	}{
		{
			name:        "HTTP/2 disabled sets TLSOpts",
			enableHTTP2: false,
			wantTLSOpts: true,
		},
		{
			name:        "HTTP/2 enabled does not set TLSOpts",
			enableHTTP2: true,
			wantTLSOpts: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			options, _, err := Load(testScheme, "", tc.enableHTTP2)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if tc.wantTLSOpts && len(options.Metrics.TLSOpts) == 0 {
				t.Error("Expected TLSOpts to be set for disabling HTTP/2")
			}
			if !tc.wantTLSOpts && len(options.Metrics.TLSOpts) > 0 {
				t.Error("Expected TLSOpts to be empty when HTTP/2 is enabled")
			}
		})
	}
}
