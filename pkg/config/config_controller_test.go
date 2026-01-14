package config

import (
	"testing"

	ctrl "sigs.k8s.io/controller-runtime"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
)

func TestAddTo_ControllerOptions(t *testing.T) {
	opts := ctrl.Options{}

	metricsAddr := ":9090"
	secure := true
	healthAddr := ":8081"

	cfg := &configapi.Configuration{
		Metrics: configapi.ControllerMetrics{
			BindAddress:   metricsAddr,
			SecureServing: &secure,
		},
		Health: configapi.ControllerHealth{
			HealthProbeBindAddress: healthAddr,
		},
	}

	addTo(&opts, cfg, false)

	if opts.Metrics.BindAddress != metricsAddr {
		t.Errorf("expected metrics bind address %s, got %s",
			metricsAddr, opts.Metrics.BindAddress)
	}

	if !opts.Metrics.SecureServing {
		t.Errorf("expected secure serving to be enabled")
	}

	if opts.HealthProbeBindAddress != healthAddr {
		t.Errorf("expected health probe bind address %s, got %s",
			healthAddr, opts.HealthProbeBindAddress)
	}
}
