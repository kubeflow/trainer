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
	"os"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
)

func setupScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = configapi.AddToScheme(scheme)
	return scheme
}

func TestLoad_Defaults(t *testing.T) {
	scheme := setupScheme()

	options, cfg, err := Load(scheme, "", false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify defaults are applied
	if cfg.Webhook.Port == nil || *cfg.Webhook.Port != 9443 {
		t.Errorf("Expected webhook port 9443, got %v", cfg.Webhook.Port)
	}

	if cfg.Metrics.BindAddress != ":8443" {
		t.Errorf("Expected metrics bind address :8443, got %s", cfg.Metrics.BindAddress)
	}

	if cfg.Health.HealthProbeBindAddress != ":8081" {
		t.Errorf("Expected health probe bind address :8081, got %s", cfg.Health.HealthProbeBindAddress)
	}

	// Verify options are set
	if options.Scheme == nil {
		t.Error("Expected scheme to be set in options")
	}
}

func TestLoad_FromFile(t *testing.T) {
	scheme := setupScheme()

	// Create a temporary config file
	content := `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
health:
  healthProbeBindAddress: :8082
metrics:
  bindAddress: :9443
webhook:
  port: 9444
certManagement:
  enable: false
clientConnection:
  qps: 100
  burst: 200
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	options, cfg, err := Load(scheme, tmpFile.Name(), false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify custom values are loaded
	if cfg.Health.HealthProbeBindAddress != ":8082" {
		t.Errorf("Expected health probe :8082, got %s", cfg.Health.HealthProbeBindAddress)
	}

	if cfg.Metrics.BindAddress != ":9443" {
		t.Errorf("Expected metrics :9443, got %s", cfg.Metrics.BindAddress)
	}

	if cfg.Webhook.Port == nil || *cfg.Webhook.Port != 9444 {
		t.Errorf("Expected webhook port 9444, got %v", cfg.Webhook.Port)
	}

	if cfg.CertManagement == nil || cfg.CertManagement.Enable == nil || *cfg.CertManagement.Enable {
		t.Error("Expected certManagement.enable to be false")
	}

	if cfg.ClientConnection == nil || *cfg.ClientConnection.QPS != 100 {
		t.Error("Expected QPS 100")
	}

	// Verify options are set correctly
	if options.Scheme == nil {
		t.Error("Expected scheme to be set")
	}
}

func TestLoad_InvalidFile(t *testing.T) {
	scheme := setupScheme()

	_, _, err := Load(scheme, "/nonexistent/file.yaml", false)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestLoad_MalformedYAML(t *testing.T) {
	scheme := setupScheme()

	// Create a temporary config file with malformed YAML
	content := `this is not: valid: yaml: content`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	_, _, err = Load(scheme, tmpFile.Name(), false)
	if err == nil {
		t.Error("Expected error for malformed YAML")
	}
}

func TestValidate_InvalidWebhookPort(t *testing.T) {
	scheme := setupScheme()

	content := `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
webhook:
  port: 99999
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	_, _, err = Load(scheme, tmpFile.Name(), false)
	if err == nil {
		t.Error("Expected validation error for invalid webhook port")
	}
}

func TestValidate_NegativeQPS(t *testing.T) {
	scheme := setupScheme()

	content := `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
clientConnection:
  qps: -10
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	_, _, err = Load(scheme, tmpFile.Name(), false)
	if err == nil {
		t.Error("Expected validation error for negative QPS")
	}
}

func TestValidate_InvalidConcurrency(t *testing.T) {
	scheme := setupScheme()

	content := `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
controller:
  groupKindConcurrency:
    TrainJob.trainer.kubeflow.org: -5
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	_, _, err = Load(scheme, tmpFile.Name(), false)
	if err == nil {
		t.Error("Expected validation error for negative concurrency")
	}
}

func TestIsCertManagementEnabled(t *testing.T) {
	tests := []struct {
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCertManagementEnabled(&tt.cfg)
			if got != tt.want {
				t.Errorf("IsCertManagementEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHTTP2SecurityDisabled(t *testing.T) {
	scheme := setupScheme()

	// Test that HTTP/2 is disabled by default
	options, _, err := Load(scheme, "", false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify that TLSOpts is set (which should disable HTTP/2)
	if len(options.Metrics.TLSOpts) == 0 {
		t.Error("Expected TLSOpts to be set for disabling HTTP/2")
	}
}

func TestHTTP2SecurityEnabled(t *testing.T) {
	scheme := setupScheme()

	// Test that HTTP/2 can be enabled with flag
	options, _, err := Load(scheme, "", true)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// When enableHTTP2 is true, TLSOpts should be nil or empty
	if len(options.Metrics.TLSOpts) > 0 {
		t.Error("Expected TLSOpts to be empty when HTTP/2 is enabled")
	}
}

func TestValidate_Success(t *testing.T) {
	cfg := configapi.Configuration{
		Webhook: configapi.ControllerWebhook{
			Port: ptr.To(9443),
		},
		ClientConnection: &configapi.ClientConnection{
			QPS:   ptr.To[float32](50),
			Burst: ptr.To[int32](100),
		},
		Controller: &configapi.ControllerConfigurationSpec{
			GroupKindConcurrency: map[string]int{
				"TrainJob.trainer.kubeflow.org": 5,
			},
		},
	}

	errs := validate(&cfg)
	if len(errs) > 0 {
		t.Errorf("Expected no validation errors, got: %v", errs)
	}
}

func TestValidate_ComprehensiveValidation(t *testing.T) {
	testCases := map[string]struct {
		cfg     configapi.Configuration
		wantErr bool
		errPath string // Expected error field path
	}{
		"valid minimal config": {
			cfg: configapi.Configuration{
				Webhook: configapi.ControllerWebhook{
					Port: ptr.To(9443),
				},
			},
			wantErr: false,
		},
		"valid with all fields": {
			cfg: configapi.Configuration{
				Webhook: configapi.ControllerWebhook{
					Port: ptr.To(8443),
					Host: "0.0.0.0",
				},
				Metrics: configapi.ControllerMetrics{
					BindAddress:   ":8080",
					SecureServing: ptr.To(true),
				},
				Health: configapi.ControllerHealth{
					HealthProbeBindAddress: ":8081",
					ReadinessEndpointName:  "ready",
					LivenessEndpointName:   "alive",
				},
				ClientConnection: &configapi.ClientConnection{
					QPS:   ptr.To[float32](100.5),
					Burst: ptr.To[int32](200),
				},
				Controller: &configapi.ControllerConfigurationSpec{
					GroupKindConcurrency: map[string]int{
						"TrainJob.trainer.kubeflow.org":               10,
						"TrainingRuntime.trainer.kubeflow.org":        5,
						"ClusterTrainingRuntime.trainer.kubeflow.org": 3,
					},
				},
			},
			wantErr: false,
		},
		"webhook port at minimum boundary (1)": {
			cfg: configapi.Configuration{
				Webhook: configapi.ControllerWebhook{
					Port: ptr.To(1),
				},
			},
			wantErr: false,
		},
		"webhook port at maximum boundary (65535)": {
			cfg: configapi.Configuration{
				Webhook: configapi.ControllerWebhook{
					Port: ptr.To(65535),
				},
			},
			wantErr: false,
		},
		"webhook port below minimum (0)": {
			cfg: configapi.Configuration{
				Webhook: configapi.ControllerWebhook{
					Port: ptr.To(0),
				},
			},
			wantErr: true,
			errPath: "webhook.port",
		},
		"webhook port above maximum (65536)": {
			cfg: configapi.Configuration{
				Webhook: configapi.ControllerWebhook{
					Port: ptr.To(65536),
				},
			},
			wantErr: true,
			errPath: "webhook.port",
		},
		"negative webhook port": {
			cfg: configapi.Configuration{
				Webhook: configapi.ControllerWebhook{
					Port: ptr.To(-1),
				},
			},
			wantErr: true,
			errPath: "webhook.port",
		},
		"QPS at zero boundary": {
			cfg: configapi.Configuration{
				ClientConnection: &configapi.ClientConnection{
					QPS: ptr.To[float32](0),
				},
			},
			wantErr: false,
		},
		"QPS with decimal value": {
			cfg: configapi.Configuration{
				ClientConnection: &configapi.ClientConnection{
					QPS: ptr.To[float32](50.5),
				},
			},
			wantErr: false,
		},
		"negative QPS": {
			cfg: configapi.Configuration{
				ClientConnection: &configapi.ClientConnection{
					QPS: ptr.To[float32](-0.1),
				},
			},
			wantErr: true,
			errPath: "clientConnection.qps",
		},
		"Burst at zero boundary": {
			cfg: configapi.Configuration{
				ClientConnection: &configapi.ClientConnection{
					Burst: ptr.To[int32](0),
				},
			},
			wantErr: false,
		},
		"negative Burst": {
			cfg: configapi.Configuration{
				ClientConnection: &configapi.ClientConnection{
					Burst: ptr.To[int32](-1),
				},
			},
			wantErr: true,
			errPath: "clientConnection.burst",
		},
		"very large Burst value": {
			cfg: configapi.Configuration{
				ClientConnection: &configapi.ClientConnection{
					Burst: ptr.To[int32](999999),
				},
			},
			wantErr: false,
		},
		"concurrency minimum valid value (1)": {
			cfg: configapi.Configuration{
				Controller: &configapi.ControllerConfigurationSpec{
					GroupKindConcurrency: map[string]int{
						"TrainJob.trainer.kubeflow.org": 1,
					},
				},
			},
			wantErr: false,
		},
		"concurrency zero value": {
			cfg: configapi.Configuration{
				Controller: &configapi.ControllerConfigurationSpec{
					GroupKindConcurrency: map[string]int{
						"TrainJob.trainer.kubeflow.org": 0,
					},
				},
			},
			wantErr: true,
			errPath: "controller.groupKindConcurrency[TrainJob.trainer.kubeflow.org]",
		},
		"concurrency negative value": {
			cfg: configapi.Configuration{
				Controller: &configapi.ControllerConfigurationSpec{
					GroupKindConcurrency: map[string]int{
						"TrainJob.trainer.kubeflow.org": -1,
					},
				},
			},
			wantErr: true,
			errPath: "controller.groupKindConcurrency[TrainJob.trainer.kubeflow.org]",
		},
		"multiple concurrency errors": {
			cfg: configapi.Configuration{
				Controller: &configapi.ControllerConfigurationSpec{
					GroupKindConcurrency: map[string]int{
						"TrainJob.trainer.kubeflow.org":        0,
						"TrainingRuntime.trainer.kubeflow.org": -5,
					},
				},
			},
			wantErr: true,
		},
		"mixed valid and invalid concurrency": {
			cfg: configapi.Configuration{
				Controller: &configapi.ControllerConfigurationSpec{
					GroupKindConcurrency: map[string]int{
						"TrainJob.trainer.kubeflow.org":        5,
						"TrainingRuntime.trainer.kubeflow.org": -1,
					},
				},
			},
			wantErr: true,
		},
		"multiple validation errors": {
			cfg: configapi.Configuration{
				Webhook: configapi.ControllerWebhook{
					Port: ptr.To(70000),
				},
				ClientConnection: &configapi.ClientConnection{
					QPS:   ptr.To[float32](-10),
					Burst: ptr.To[int32](-20),
				},
				Controller: &configapi.ControllerConfigurationSpec{
					GroupKindConcurrency: map[string]int{
						"TrainJob.trainer.kubeflow.org": -1,
					},
				},
			},
			wantErr: true,
		},
		"nil client connection": {
			cfg: configapi.Configuration{
				ClientConnection: nil,
			},
			wantErr: false,
		},
		"nil controller config": {
			cfg: configapi.Configuration{
				Controller: nil,
			},
			wantErr: false,
		},
		"empty GroupKindConcurrency map": {
			cfg: configapi.Configuration{
				Controller: &configapi.ControllerConfigurationSpec{
					GroupKindConcurrency: map[string]int{},
				},
			},
			wantErr: false,
		},
		"high concurrency value": {
			cfg: configapi.Configuration{
				Controller: &configapi.ControllerConfigurationSpec{
					GroupKindConcurrency: map[string]int{
						"TrainJob.trainer.kubeflow.org": 1000,
					},
				},
			},
			wantErr: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			errs := validate(&tc.cfg)
			if tc.wantErr && len(errs) == 0 {
				t.Errorf("Expected validation error but got none")
			}
			if !tc.wantErr && len(errs) > 0 {
				t.Errorf("Expected no validation errors, got: %v", errs)
			}
			if tc.wantErr && tc.errPath != "" && len(errs) > 0 {
				// Check if any error matches the expected path
				found := false
				for _, err := range errs {
					if err.Field == tc.errPath {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error for field %s, but got errors: %v", tc.errPath, errs)
				}
			}
		})
	}
}

func TestLoad_WithLeaderElection(t *testing.T) {
	scheme := setupScheme()

	content := `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
leaderElection:
  leaderElect: true
  resourceName: trainer-leader
  resourceNamespace: kubeflow
  resourceLock: leases
  leaseDuration: 15s
  renewDeadline: 10s
  retryPeriod: 2s
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	options, cfg, err := Load(scheme, tmpFile.Name(), false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify leader election is enabled
	if !options.LeaderElection {
		t.Error("Expected leader election to be enabled")
	}

	if options.LeaderElectionID != "trainer-leader" {
		t.Errorf("Expected leader election ID 'trainer-leader', got %s", options.LeaderElectionID)
	}

	if cfg.LeaderElection == nil {
		t.Fatal("Expected LeaderElection config to be set")
	}

	if cfg.LeaderElection.ResourceLock != "leases" {
		t.Errorf("Expected resource lock 'leases', got %s", cfg.LeaderElection.ResourceLock)
	}
}

func TestLoad_MetricsConfiguration(t *testing.T) {
	scheme := setupScheme()

	testCases := map[string]struct {
		yaml            string
		wantBindAddress string
		wantSecure      bool
	}{
		"default metrics config": {
			yaml: `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
`,
			wantBindAddress: ":8443",
			wantSecure:      true,
		},
		"custom metrics port": {
			yaml: `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
metrics:
  bindAddress: :9090
`,
			wantBindAddress: ":9090",
			wantSecure:      true,
		},
		"metrics with insecure serving": {
			yaml: `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
metrics:
  bindAddress: :8080
  secureServing: false
`,
			wantBindAddress: ":8080",
			wantSecure:      false,
		},
		"disabled metrics": {
			yaml: `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
metrics:
  bindAddress: "0"
`,
			wantBindAddress: "0",
			wantSecure:      true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "config-*.yaml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.Write([]byte(tc.yaml)); err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}
			tmpFile.Close()

			options, cfg, err := Load(scheme, tmpFile.Name(), false)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.Metrics.BindAddress != tc.wantBindAddress {
				t.Errorf("Expected bind address %s, got %s", tc.wantBindAddress, cfg.Metrics.BindAddress)
			}

			if options.Metrics.BindAddress != tc.wantBindAddress {
				t.Errorf("Expected options bind address %s, got %s", tc.wantBindAddress, options.Metrics.BindAddress)
			}

			if options.Metrics.SecureServing != tc.wantSecure {
				t.Errorf("Expected secure serving %v, got %v", tc.wantSecure, options.Metrics.SecureServing)
			}
		})
	}
}

func TestLoad_HealthConfiguration(t *testing.T) {
	scheme := setupScheme()

	content := `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
health:
  healthProbeBindAddress: :9090
  readinessEndpointName: ready
  livenessEndpointName: alive
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	options, cfg, err := Load(scheme, tmpFile.Name(), false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Health.HealthProbeBindAddress != ":9090" {
		t.Errorf("Expected health probe address :9090, got %s", cfg.Health.HealthProbeBindAddress)
	}

	if cfg.Health.ReadinessEndpointName != "ready" {
		t.Errorf("Expected readiness endpoint 'ready', got %s", cfg.Health.ReadinessEndpointName)
	}

	if cfg.Health.LivenessEndpointName != "alive" {
		t.Errorf("Expected liveness endpoint 'alive', got %s", cfg.Health.LivenessEndpointName)
	}

	if options.HealthProbeBindAddress != ":9090" {
		t.Errorf("Expected options health probe address :9090, got %s", options.HealthProbeBindAddress)
	}
}

func TestLoad_ControllerConcurrency(t *testing.T) {
	scheme := setupScheme()

	content := `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
controller:
  groupKindConcurrency:
    TrainJob.trainer.kubeflow.org: 10
    TrainingRuntime.trainer.kubeflow.org: 5
    ClusterTrainingRuntime.trainer.kubeflow.org: 3
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	options, cfg, err := Load(scheme, tmpFile.Name(), false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Controller == nil {
		t.Fatal("Expected Controller config to be set")
	}

	expectedConcurrency := map[string]int{
		"TrainJob.trainer.kubeflow.org":               10,
		"TrainingRuntime.trainer.kubeflow.org":        5,
		"ClusterTrainingRuntime.trainer.kubeflow.org": 3,
	}

	for gk, expected := range expectedConcurrency {
		if actual, ok := cfg.Controller.GroupKindConcurrency[gk]; !ok {
			t.Errorf("Expected concurrency for %s to be set", gk)
		} else if actual != expected {
			t.Errorf("Expected concurrency %d for %s, got %d", expected, gk, actual)
		}

		// Verify options are also set
		if actual, ok := options.Controller.GroupKindConcurrency[gk]; !ok {
			t.Errorf("Expected options concurrency for %s to be set", gk)
		} else if actual != expected {
			t.Errorf("Expected options concurrency %d for %s, got %d", expected, gk, actual)
		}
	}
}

func TestLoad_CertManagement(t *testing.T) {
	scheme := setupScheme()

	testCases := map[string]struct {
		yaml            string
		wantEnabled     bool
		wantServiceName string
		wantSecretName  string
	}{
		"default cert management": {
			yaml: `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
`,
			wantEnabled:     true,
			wantServiceName: "kubeflow-trainer-controller-manager",
			wantSecretName:  "kubeflow-trainer-webhook-cert",
		},
		"custom cert names": {
			yaml: `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
certManagement:
  enable: true
  webhookServiceName: custom-webhook-service
  webhookSecretName: custom-webhook-secret
`,
			wantEnabled:     true,
			wantServiceName: "custom-webhook-service",
			wantSecretName:  "custom-webhook-secret",
		},
		"disabled cert management": {
			yaml: `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
certManagement:
  enable: false
`,
			wantEnabled:     false,
			wantServiceName: "kubeflow-trainer-controller-manager",
			wantSecretName:  "kubeflow-trainer-webhook-cert",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "config-*.yaml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.Write([]byte(tc.yaml)); err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}
			tmpFile.Close()

			_, cfg, err := Load(scheme, tmpFile.Name(), false)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			enabled := IsCertManagementEnabled(&cfg)
			if enabled != tc.wantEnabled {
				t.Errorf("Expected cert management enabled=%v, got %v", tc.wantEnabled, enabled)
			}

			if cfg.CertManagement == nil {
				t.Fatal("Expected CertManagement to be initialized")
			}

			if cfg.CertManagement.WebhookServiceName != tc.wantServiceName {
				t.Errorf("Expected service name %s, got %s", tc.wantServiceName, cfg.CertManagement.WebhookServiceName)
			}

			if cfg.CertManagement.WebhookSecretName != tc.wantSecretName {
				t.Errorf("Expected secret name %s, got %s", tc.wantSecretName, cfg.CertManagement.WebhookSecretName)
			}
		})
	}
}

func TestLoad_WebhookHost(t *testing.T) {
	scheme := setupScheme()

	content := `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
webhook:
  port: 9443
  host: localhost
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	options, cfg, err := Load(scheme, tmpFile.Name(), false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Webhook.Host != "localhost" {
		t.Errorf("Expected webhook host 'localhost', got %s", cfg.Webhook.Host)
	}

	if options.WebhookServer == nil {
		t.Fatal("Expected webhook server to be configured")
	}
}

func TestLoad_EmptyYAML(t *testing.T) {
	scheme := setupScheme()

	// Empty YAML document is valid and should get defaults applied
	content := `---
apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	options, cfg, err := Load(scheme, tmpFile.Name(), false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should have defaults applied
	if cfg.Webhook.Port == nil || *cfg.Webhook.Port != 9443 {
		t.Errorf("Expected default webhook port 9443, got %v", cfg.Webhook.Port)
	}

	if options.Scheme == nil {
		t.Error("Expected scheme to be set")
	}
}

func TestLoad_OnlyWhitespace(t *testing.T) {
	scheme := setupScheme()

	content := `   
	
   `
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	_, _, err = Load(scheme, tmpFile.Name(), false)
	// File with only whitespace should fail
	if err == nil {
		t.Error("Expected error for file with only whitespace")
	}
}

func TestLoad_WrongAPIVersion(t *testing.T) {
	scheme := setupScheme()

	content := `apiVersion: config.wrong.group/v1
kind: Configuration
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	_, _, err = Load(scheme, tmpFile.Name(), false)
	if err == nil {
		t.Error("Expected error for wrong API version")
	}
}

func TestLoad_WrongKind(t *testing.T) {
	scheme := setupScheme()

	content := `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: WrongKind
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	_, _, err = Load(scheme, tmpFile.Name(), false)
	if err == nil {
		t.Error("Expected error for wrong Kind")
	}
}

func TestLoad_UnknownFields(t *testing.T) {
	scheme := setupScheme()

	content := `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
unknownField: value
webhook:
  port: 9443
  unknownWebhookField: value
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	_, _, err = Load(scheme, tmpFile.Name(), false)
	// Should fail due to strict decoding
	if err == nil {
		t.Error("Expected error for unknown fields")
	}
}

func TestValidate_NegativeBurst(t *testing.T) {
	scheme := setupScheme()

	content := `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
clientConnection:
  burst: -100
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	_, _, err = Load(scheme, tmpFile.Name(), false)
	if err == nil {
		t.Error("Expected validation error for negative burst")
	}
}

func TestValidate_ZeroPort(t *testing.T) {
	scheme := setupScheme()

	content := `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
webhook:
  port: 0
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	_, _, err = Load(scheme, tmpFile.Name(), false)
	if err == nil {
		t.Error("Expected validation error for port 0")
	}
}

func TestValidate_MaxPortBoundary(t *testing.T) {
	scheme := setupScheme()

	// Test exactly at the boundary
	content := `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
webhook:
  port: 65535
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	_, cfg, err := Load(scheme, tmpFile.Name(), false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Webhook.Port == nil || *cfg.Webhook.Port != 65535 {
		t.Errorf("Expected port 65535, got %v", cfg.Webhook.Port)
	}
}

func TestLoad_CompleteConfiguration(t *testing.T) {
	scheme := setupScheme()

	// Test with all possible configuration options
	content := `apiVersion: config.trainer.kubeflow.org/v1alpha1
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
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	options, cfg, err := Load(scheme, tmpFile.Name(), false)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify all configurations are properly loaded
	if cfg.Webhook.Port == nil || *cfg.Webhook.Port != 9443 {
		t.Error("Webhook port not set correctly")
	}
	if cfg.Webhook.Host != "0.0.0.0" {
		t.Error("Webhook host not set correctly")
	}
	if cfg.Metrics.BindAddress != ":8443" {
		t.Error("Metrics bind address not set correctly")
	}
	if cfg.Health.HealthProbeBindAddress != ":8081" {
		t.Error("Health probe address not set correctly")
	}
	if !options.LeaderElection {
		t.Error("Leader election not enabled")
	}
	if cfg.ClientConnection == nil || *cfg.ClientConnection.QPS != 50 {
		t.Error("QPS not set correctly")
	}
	if cfg.Controller == nil || len(cfg.Controller.GroupKindConcurrency) != 2 {
		t.Error("Controller concurrency not set correctly")
	}
}
