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
	"crypto/tls"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/util/tlsconfig"
)

// fromFile loads configuration from a file.
func fromFile(path string, scheme *runtime.Scheme, cfg *configapi.Configuration) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	codecs := serializer.NewCodecFactory(scheme, serializer.EnableStrict)

	if err := runtime.DecodeInto(codecs.UniversalDecoder(), content, cfg); err != nil {
		return fmt.Errorf("failed to decode config file: %w", err)
	}

	return nil
}

// addTo applies the configuration to controller runtime Options.
func addTo(o *ctrl.Options, cfg *configapi.Configuration) {
	tlsOpts := []func(*tls.Config){
		func(c *tls.Config) {
			tlsconfig.Apply(c, cfg.TLS)
		},
	}

	o.Metrics = metricsserver.Options{
		BindAddress:   cfg.Metrics.BindAddress,
		SecureServing: cfg.Metrics.SecureServing != nil && *cfg.Metrics.SecureServing,
		TLSOpts:       tlsOpts,
	}

	if cfg.Webhook.Port != nil {
		webhookOpts := webhook.Options{
			Port:    int(*cfg.Webhook.Port),
			TLSOpts: tlsOpts,
		}
		if cfg.Webhook.Host != nil {
			webhookOpts.Host = *cfg.Webhook.Host
		}
		o.WebhookServer = webhook.NewServer(webhookOpts)
	}

	o.HealthProbeBindAddress = cfg.Health.HealthProbeBindAddress

	if cfg.LeaderElection != nil {
		if cfg.LeaderElection.LeaderElect != nil {
			o.LeaderElection = *cfg.LeaderElection.LeaderElect
		}
		o.LeaderElectionResourceLock = cfg.LeaderElection.ResourceLock
		o.LeaderElectionNamespace = cfg.LeaderElection.ResourceNamespace
		o.LeaderElectionID = cfg.LeaderElection.ResourceName
		o.LeaseDuration = &cfg.LeaderElection.LeaseDuration.Duration
		o.RenewDeadline = &cfg.LeaderElection.RenewDeadline.Duration
		o.RetryPeriod = &cfg.LeaderElection.RetryPeriod.Duration
	}

	if cfg.Controller != nil && len(cfg.Controller.GroupKindConcurrency) > 0 {
		if o.Controller.GroupKindConcurrency == nil {
			o.Controller.GroupKindConcurrency = make(map[string]int)
		}
		for gk, concurrency := range cfg.Controller.GroupKindConcurrency {
			o.Controller.GroupKindConcurrency[gk] = int(concurrency)
		}
	}
}

// Load loads configuration from file and returns controller Options and Configuration.
func Load(scheme *runtime.Scheme, configFile string) (ctrl.Options, configapi.Configuration, error) {
	options := ctrl.Options{
		Scheme: scheme,
	}

	cfg := configapi.Configuration{}

	if configFile == "" {
		scheme.Default(&cfg)
	} else {
		if err := fromFile(configFile, scheme, &cfg); err != nil {
			return options, cfg, err
		}
	}

	if errs := validate(&cfg); len(errs) > 0 {
		return options, cfg, fmt.Errorf("invalid configuration: %v", errs.ToAggregate())
	}

	addTo(&options, &cfg)

	return options, cfg, nil
}

// ApplyClientConnection copies QPS and burst from cfg.ClientConnection to restCfg.
// If ClientConnection is nil or individual fields are nil, existing restCfg values are preserved.
func ApplyClientConnection(restCfg *rest.Config, cfg *configapi.Configuration) {
	if cfg.ClientConnection != nil {
		if cfg.ClientConnection.QPS != nil {
			restCfg.QPS = *cfg.ClientConnection.QPS
		}
		if cfg.ClientConnection.Burst != nil {
			restCfg.Burst = int(*cfg.ClientConnection.Burst)
		}
	}
}

// IsCertManagementEnabled returns true if certificate management is enabled.
func IsCertManagementEnabled(cfg *configapi.Configuration) bool {
	if cfg.CertManagement == nil || cfg.CertManagement.Enable == nil {
		return true
	}
	return *cfg.CertManagement.Enable
}
