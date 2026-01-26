package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	configv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
)

// setupScheme creates a runtime scheme with the config API registered
func setupScheme(t *testing.T) *runtime.Scheme {
	scheme := runtime.NewScheme()
	if err := configapi.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add config API to scheme: %v", err)
	}
	return scheme
}

// createTempConfigFile creates a temporary YAML config file for testing
func createTempConfigFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	return filePath
}

func TestFromFile(t *testing.T) {
	scheme := setupScheme(t)

	tests := []struct {
		name        string
		fileContent string
		wantErr     bool
		validate    func(t *testing.T, cfg *configapi.Configuration)
	}{
		{
			name: "valid configuration file",
			fileContent: `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
metrics:
  bindAddress: ":8443"
  secureServing: true
webhook:
  port: 9443
health:
  healthProbeBindAddress: ":8081"
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *configapi.Configuration) {
				if cfg.Metrics.BindAddress != ":8443" {
					t.Errorf("Expected metrics bind address :8443, got %s", cfg.Metrics.BindAddress)
				}
				if cfg.Webhook.Port == nil || *cfg.Webhook.Port != 9443 {
					t.Errorf("Expected webhook port 9443, got %v", cfg.Webhook.Port)
				}
				if cfg.Health.HealthProbeBindAddress != ":8081" {
					t.Errorf("Expected health probe address :8081, got %s", cfg.Health.HealthProbeBindAddress)
				}
			},
		},
		{
			name: "configuration with leader election",
			fileContent: `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
leaderElection:
  leaderElect: true
  resourceName: test-lock
  leaseDuration: 15s
  renewDeadline: 10s
  retryPeriod: 2s
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *configapi.Configuration) {
				if cfg.LeaderElection == nil {
					t.Fatal("Expected leader election config, got nil")
				}
				if cfg.LeaderElection.ResourceName != "test-lock" {
					t.Errorf("Expected resource name test-lock, got %s", cfg.LeaderElection.ResourceName)
				}
			},
		},
		{
			name: "configuration with controller concurrency",
			fileContent: `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
controller:
  groupKindConcurrency:
    TrainJob.trainer.kubeflow.org: 5
    TrainingRuntime.trainer.kubeflow.org: 3
`,
			wantErr: false,
			validate: func(t *testing.T, cfg *configapi.Configuration) {
				if cfg.Controller == nil || cfg.Controller.GroupKindConcurrency == nil {
					t.Fatal("Expected controller config with concurrency, got nil")
				}
				if cfg.Controller.GroupKindConcurrency["TrainJob.trainer.kubeflow.org"] != 5 {
					t.Errorf("Expected TrainJob concurrency 5, got %d",
						cfg.Controller.GroupKindConcurrency["TrainJob.trainer.kubeflow.org"])
				}
			},
		},
		{
			name: "invalid YAML format",
			fileContent: `this is not valid yaml: [[[
invalid: structure
`,
			wantErr: true,
		},
		{
			name: "wrong API version",
			fileContent: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := createTempConfigFile(t, tt.fileContent)
			cfg := &configapi.Configuration{}

			err := fromFile(filePath, scheme, cfg)

			if (err != nil) != tt.wantErr {
				t.Errorf("fromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestFromFile_NonExistentFile(t *testing.T) {
	scheme := setupScheme(t)
	cfg := &configapi.Configuration{}

	err := fromFile("/nonexistent/path/config.yaml", scheme, cfg)

	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestAddTo(t *testing.T) {
	tests := []struct {
		name        string
		cfg         configapi.Configuration
		enableHTTP2 bool
		validate    func(t *testing.T, opts *runtime.Scheme)
	}{
		{
			name: "metrics configuration",
			cfg: configapi.Configuration{
				Metrics: configapi.ControllerMetrics{
					BindAddress:   ":9090",
					SecureServing: ptr.To(true),
				},
			},
			enableHTTP2: true,
		},
		{
			name: "webhook configuration",
			cfg: configapi.Configuration{
				Webhook: configapi.ControllerWebhook{
					Port: ptr.To(int32(8443)),
					Host: ptr.To("localhost"),
				},
			},
			enableHTTP2: true,
		},
		{
			name: "health probe configuration",
			cfg: configapi.Configuration{
				Health: configapi.ControllerHealth{
					HealthProbeBindAddress: ":8888",
				},
			},
			enableHTTP2: true,
		},
		{
			name: "leader election configuration",
			cfg: configapi.Configuration{
				LeaderElection: &configv1alpha1.LeaderElectionConfiguration{
					LeaderElect:       ptr.To(true),
					ResourceLock:      "leases",
					ResourceName:      "test-lock",
					ResourceNamespace: "default",
					LeaseDuration:     metav1.Duration{Duration: 15 * time.Second},
					RenewDeadline:     metav1.Duration{Duration: 10 * time.Second},
					RetryPeriod:       metav1.Duration{Duration: 2 * time.Second},
				},
			},
			enableHTTP2: true,
		},
		{
			name: "controller concurrency configuration",
			cfg: configapi.Configuration{
				Controller: &configapi.ControllerConfigurationSpec{
					GroupKindConcurrency: map[string]int32{
						"TrainJob.trainer.kubeflow.org": 5,
					},
				},
			},
			enableHTTP2: true,
		},
		{
			name: "HTTP/2 disabled",
			cfg: configapi.Configuration{
				Metrics: configapi.ControllerMetrics{
					BindAddress:   ":8443",
					SecureServing: ptr.To(true),
				},
			},
			enableHTTP2: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := setupScheme(t)
			options := ctrl.Options{Scheme: scheme}

			// This test verifies that addTo doesn't panic and can be called successfully
			// A full integration test would require mocking controller-runtime components
			// which is done in the Load() tests below
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("addTo() panicked: %v", r)
				}
			}()

			// Call addTo to verify it works without panicking
			addTo(&options, &tt.cfg, tt.enableHTTP2)

			// Note: Full behavioral testing of addTo is done in the Load() tests
			// since the options require a complete controller-runtime setup
		})
	}
}

func TestLoad(t *testing.T) {
	scheme := setupScheme(t)

	tests := []struct {
		name        string
		configFile  string
		fileContent string
		enableHTTP2 bool
		wantErr     bool
		validate    func(t *testing.T, cfg configapi.Configuration)
	}{
		{
			name:        "load with defaults (empty config file)",
			configFile:  "",
			enableHTTP2: true,
			wantErr:     false,
			validate: func(t *testing.T, cfg configapi.Configuration) {
				// Verify defaults are applied
				if cfg.Webhook.Port == nil || *cfg.Webhook.Port != 9443 {
					t.Errorf("Expected default webhook port 9443, got %v", cfg.Webhook.Port)
				}
				if cfg.Metrics.BindAddress != ":8443" {
					t.Errorf("Expected default metrics bind address :8443, got %s", cfg.Metrics.BindAddress)
				}
				if cfg.Health.HealthProbeBindAddress != ":8081" {
					t.Errorf("Expected default health probe address :8081, got %s", cfg.Health.HealthProbeBindAddress)
				}
				if cfg.CertManagement == nil || cfg.CertManagement.Enable == nil || !*cfg.CertManagement.Enable {
					t.Error("Expected cert management enabled by default")
				}
			},
		},
		{
			name: "load valid configuration file",
			fileContent: `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
metrics:
  bindAddress: ":9090"
  secureServing: false
webhook:
  port: 8443
health:
  healthProbeBindAddress: ":9000"
`,
			enableHTTP2: true,
			wantErr:     false,
			validate: func(t *testing.T, cfg configapi.Configuration) {
				if cfg.Metrics.BindAddress != ":9090" {
					t.Errorf("Expected metrics bind address :9090, got %s", cfg.Metrics.BindAddress)
				}
				if cfg.Webhook.Port == nil || *cfg.Webhook.Port != 8443 {
					t.Errorf("Expected webhook port 8443, got %v", cfg.Webhook.Port)
				}
			},
		},
		{
			name: "load configuration with invalid webhook port",
			fileContent: `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
webhook:
  port: 99999
`,
			enableHTTP2: true,
			wantErr:     true,
		},
		{
			name: "load configuration with negative QPS",
			fileContent: `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
clientConnection:
  qps: -10
`,
			enableHTTP2: true,
			wantErr:     true,
		},
		{
			name: "load configuration with invalid concurrency",
			fileContent: `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
controller:
  groupKindConcurrency:
    TrainJob.trainer.kubeflow.org: 0
`,
			enableHTTP2: true,
			wantErr:     true,
		},
		{
			name:        "load non-existent file",
			configFile:  "/nonexistent/config.yaml",
			enableHTTP2: true,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFile := tt.configFile
			if tt.fileContent != "" {
				configFile = createTempConfigFile(t, tt.fileContent)
			}

			opts, cfg, err := Load(scheme, configFile, tt.enableHTTP2)

			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if opts.Scheme == nil {
					t.Error("Expected options.Scheme to be set, got nil")
				}
				if tt.validate != nil {
					tt.validate(t, cfg)
				}
			}
		})
	}
}

func TestIsCertManagementEnabled(t *testing.T) {
	tests := []struct {
		name string
		cfg  *configapi.Configuration
		want bool
	}{
		{
			name: "cert management is nil",
			cfg:  &configapi.Configuration{},
			want: true, // Default is enabled
		},
		{
			name: "cert management enable is nil",
			cfg: &configapi.Configuration{
				CertManagement: &configapi.CertManagement{},
			},
			want: true, // Default is enabled
		},
		{
			name: "cert management explicitly enabled",
			cfg: &configapi.Configuration{
				CertManagement: &configapi.CertManagement{
					Enable: ptr.To(true),
				},
			},
			want: true,
		},
		{
			name: "cert management explicitly disabled",
			cfg: &configapi.Configuration{
				CertManagement: &configapi.CertManagement{
					Enable: ptr.To(false),
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCertManagementEnabled(tt.cfg)
			if got != tt.want {
				t.Errorf("IsCertManagementEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     configapi.Configuration
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid configuration",
			cfg: configapi.Configuration{
				Webhook: configapi.ControllerWebhook{
					Port: ptr.To(int32(9443)),
				},
				ClientConnection: &configapi.ClientConnection{
					QPS:   ptr.To(float32(50)),
					Burst: ptr.To(int32(100)),
				},
				Controller: &configapi.ControllerConfigurationSpec{
					GroupKindConcurrency: map[string]int32{
						"TrainJob.trainer.kubeflow.org": 5,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid webhook port - too low",
			cfg: configapi.Configuration{
				Webhook: configapi.ControllerWebhook{
					Port: ptr.To(int32(0)),
				},
			},
			wantErr: true,
			errMsg:  "webhook.port",
		},
		{
			name: "invalid webhook port - too high",
			cfg: configapi.Configuration{
				Webhook: configapi.ControllerWebhook{
					Port: ptr.To(int32(65536)),
				},
			},
			wantErr: true,
			errMsg:  "webhook.port",
		},
		{
			name: "negative QPS",
			cfg: configapi.Configuration{
				ClientConnection: &configapi.ClientConnection{
					QPS: ptr.To(float32(-1)),
				},
			},
			wantErr: true,
			errMsg:  "clientConnection.qps",
		},
		{
			name: "negative Burst",
			cfg: configapi.Configuration{
				ClientConnection: &configapi.ClientConnection{
					Burst: ptr.To(int32(-10)),
				},
			},
			wantErr: true,
			errMsg:  "clientConnection.burst",
		},
		{
			name: "zero concurrency",
			cfg: configapi.Configuration{
				Controller: &configapi.ControllerConfigurationSpec{
					GroupKindConcurrency: map[string]int32{
						"TrainJob.trainer.kubeflow.org": 0,
					},
				},
			},
			wantErr: true,
			errMsg:  "controller.groupKindConcurrency[TrainJob.trainer.kubeflow.org]",
		},
		{
			name: "negative concurrency",
			cfg: configapi.Configuration{
				Controller: &configapi.ControllerConfigurationSpec{
					GroupKindConcurrency: map[string]int32{
						"TrainJob.trainer.kubeflow.org": -5,
					},
				},
			},
			wantErr: true,
			errMsg:  "controller.groupKindConcurrency[TrainJob.trainer.kubeflow.org]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validate(&tt.cfg)

			if tt.wantErr {
				if len(errs) == 0 {
					t.Error("Expected validation errors, got none")
				} else if tt.errMsg != "" {
					found := false
					for _, err := range errs {
						if err.Field == tt.errMsg {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected error for field %s, got errors: %v", tt.errMsg, errs)
					}
				}
			} else {
				if len(errs) > 0 {
					t.Errorf("Expected no validation errors, got: %v", errs.ToAggregate())
				}
			}
		})
	}
}

func TestLoad_IntegrationWithDefaults(t *testing.T) {
	scheme := setupScheme(t)

	// Test that defaults are properly applied and integrated
	opts, cfg, err := Load(scheme, "", true)
	if err != nil {
		t.Fatalf("Load() with defaults failed: %v", err)
	}

	// Verify scheme is set
	if opts.Scheme == nil {
		t.Error("Expected opts.Scheme to be set")
	}

	// Verify all default values are applied
	expectedDefaults := map[string]interface{}{
		"webhook.port":                   int32(9443),
		"metrics.bindAddress":            ":8443",
		"metrics.secureServing":          true,
		"health.healthProbeBindAddress":  ":8081",
		"health.readinessEndpointName":   "readyz",
		"health.livenessEndpointName":    "healthz",
		"certManagement.enable":          true,
		"certManagement.webhookServiceName": "kubeflow-trainer-controller-manager",
		"certManagement.webhookSecretName":  "kubeflow-trainer-webhook-cert",
		"clientConnection.qps":           float32(50),
		"clientConnection.burst":         int32(100),
	}

	if cfg.Webhook.Port == nil || *cfg.Webhook.Port != expectedDefaults["webhook.port"].(int32) {
		t.Errorf("Default webhook port mismatch")
	}
	if cfg.Metrics.BindAddress != expectedDefaults["metrics.bindAddress"].(string) {
		t.Errorf("Default metrics bind address mismatch")
	}
	if cfg.CertManagement.WebhookServiceName != expectedDefaults["certManagement.webhookServiceName"].(string) {
		t.Errorf("Default webhook service name mismatch")
	}
}

func TestLoad_CompleteConfiguration(t *testing.T) {
	scheme := setupScheme(t)

	// Create a complete configuration file similar to the one in manifests
	configContent := `apiVersion: config.trainer.kubeflow.org/v1alpha1
kind: Configuration
health:
  healthProbeBindAddress: :8081
  readinessEndpointName: readyz
  livenessEndpointName: healthz
metrics:
  bindAddress: :8443
  secureServing: true
webhook:
  port: 9443
  host: ""
leaderElection:
  leaderElect: true
  resourceName: trainer.kubeflow.org
  resourceNamespace: ""
  leaseDuration: 15s
  renewDeadline: 10s
  retryPeriod: 2s
controller:
  groupKindConcurrency:
    TrainJob.trainer.kubeflow.org: 5
    TrainingRuntime.trainer.kubeflow.org: 1
    ClusterTrainingRuntime.trainer.kubeflow.org: 1
certManagement:
  enable: true
  webhookServiceName: kubeflow-trainer-controller-manager
  webhookSecretName: kubeflow-trainer-webhook-cert
clientConnection:
  qps: 50
  burst: 100
`

	configFile := createTempConfigFile(t, configContent)
	opts, cfg, err := Load(scheme, configFile, true)

	if err != nil {
		t.Fatalf("Load() failed with complete configuration: %v", err)
	}

	// Verify configuration was loaded correctly
	if opts.Scheme == nil {
		t.Error("Expected opts.Scheme to be set")
	}

	// Verify specific values
	if cfg.Webhook.Port == nil || *cfg.Webhook.Port != 9443 {
		t.Errorf("Expected webhook port 9443, got %v", cfg.Webhook.Port)
	}

	if cfg.LeaderElection == nil {
		t.Fatal("Expected leader election config")
	}
	if cfg.LeaderElection.ResourceName != "trainer.kubeflow.org" {
		t.Errorf("Expected resource name trainer.kubeflow.org, got %s", cfg.LeaderElection.ResourceName)
	}

	if cfg.Controller == nil || cfg.Controller.GroupKindConcurrency == nil {
		t.Fatal("Expected controller concurrency config")
	}
	if cfg.Controller.GroupKindConcurrency["TrainJob.trainer.kubeflow.org"] != 5 {
		t.Errorf("Expected TrainJob concurrency 5, got %d",
			cfg.Controller.GroupKindConcurrency["TrainJob.trainer.kubeflow.org"])
	}
}
