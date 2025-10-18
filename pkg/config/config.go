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
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
)

// fromFile loads configuration from a file.
func fromFile(path string, scheme *runtime.Scheme, cfg *configapi.Configuration) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	codecs := serializer.NewCodecFactory(scheme, serializer.EnableStrict)

	// Decode the configuration file into the Configuration object
	if err := runtime.DecodeInto(codecs.UniversalDecoder(), content, cfg); err != nil {
		return fmt.Errorf("failed to decode config file: %w", err)
	}

	return nil
}

// addTo applies the configuration to controller runtime Options.
func addTo(o *ctrl.Options, cfg *configapi.Configuration, enableHTTP2 bool) {
	// Set metrics server options
	var tlsOpts []func(*tls.Config)
	if !enableHTTP2 {
		// Disable http/2 for security reasons (CVE-2023-44487, CVE-2023-39325)
		tlsOpts = append(tlsOpts, func(c *tls.Config) {
			c.NextProtos = []string{"http/1.1"}
		})
	}

	o.Metrics = metricsserver.Options{
		BindAddress:   cfg.Metrics.BindAddress,
		SecureServing: cfg.Metrics.SecureServing != nil && *cfg.Metrics.SecureServing,
		TLSOpts:       tlsOpts,
	}

	// Set webhook server options
	if cfg.Webhook.Port != nil {
		o.WebhookServer = webhook.NewServer(webhook.Options{
			Port:    int(*cfg.Webhook.Port),
			Host:    *cfg.Webhook.Host,
			TLSOpts: tlsOpts,
		})
	}

	// Set health probe bind address
	o.HealthProbeBindAddress = cfg.Health.HealthProbeBindAddress

	// Set leader election
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

	// Set controller concurrency if specified
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
// If configFile is empty, default configuration is used.
func Load(scheme *runtime.Scheme, configFile string, enableHTTP2 bool) (ctrl.Options, configapi.Configuration, error) {
	options := ctrl.Options{
		Scheme: scheme,
	}

	cfg := configapi.Configuration{}

	if configFile == "" {
		// Apply defaults
		scheme.Default(&cfg)
	} else {
		// Load from file
		if err := fromFile(configFile, scheme, &cfg); err != nil {
			return options, cfg, err
		}
	}

	// Validate configuration
	if errs := validate(&cfg); len(errs) > 0 {
		return options, cfg, fmt.Errorf("invalid configuration: %v", errs.ToAggregate())
	}

	// Apply configuration to options
	addTo(&options, &cfg, enableHTTP2)

	return options, cfg, nil
}

// IsCertManagementEnabled returns true if certificate management is enabled.
// Returns true by default if not explicitly disabled.
func IsCertManagementEnabled(cfg *configapi.Configuration) bool {
	if cfg.CertManagement == nil || cfg.CertManagement.Enable == nil {
		return true // Enabled by default
	}
	return *cfg.CertManagement.Enable
}
