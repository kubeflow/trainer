package config

import (
	"testing"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
)

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &configapi.Configuration{}

	errs := validate(cfg)

	if len(errs) != 0 {
		t.Errorf("expected no validation errors, got %d", len(errs))
	}
}
