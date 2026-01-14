package config

import (
	"testing"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
)

func TestIsCertManagementEnabled_Default(t *testing.T) {
	cfg := &configapi.Configuration{}

	// Cert management should be enabled by default
	if !IsCertManagementEnabled(cfg) {
		t.Errorf("expected cert management to be enabled by default")
	}
}
func TestIsCertManagementEnabled_Disabled(t *testing.T) {
	enabled := false

	cfg := &configapi.Configuration{
		CertManagement: &configapi.CertManagement{
			Enable: &enabled,
		},
	}

	if IsCertManagementEnabled(cfg) {
		t.Errorf("expected cert management to be disabled when explicitly set to false")
	}
}
