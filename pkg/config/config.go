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
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	tlspkg "github.com/openshift/controller-runtime-common/pkg/tls"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

var (
	tlsScheme = runtime.NewScheme()
	configLog = ctrl.Log.WithName("config")
)

func init() {
	utilruntime.Must(configv1.Install(tlsScheme))
}

// TLSProfileResult holds the result of fetching the cluster TLS profile.
type TLSProfileResult struct {
	TLSOpts               []func(*tls.Config)
	Profile               configv1.TLSProfileSpec
	HasOpenShiftConfigAPI bool
}

// FetchTLSProfile fetches the cluster TLS profile and returns TLS options.
// When restCfg is nil (e.g. in unit tests), hardened defaults are returned without
// attempting API access.
func FetchTLSProfile(restCfg *rest.Config) TLSProfileResult {
	var result TLSProfileResult
	if restCfg == nil {
		result.TLSOpts = append(result.TLSOpts, func(c *tls.Config) {
			c.MinVersion = tls.VersionTLS12
		})
		return result
	}
	bootstrapCtx, bootstrapCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer bootstrapCancel()
	bootstrapClient, err := client.New(restCfg, client.Options{Scheme: tlsScheme})
	if err != nil {
		configLog.Error(err, "Failed to create bootstrap client for TLS profile, using hardened defaults")
		result.TLSOpts = append(result.TLSOpts, func(c *tls.Config) {
			c.MinVersion = tls.VersionTLS12
		})
		return result
	}
	result.Profile, err = tlspkg.FetchAPIServerTLSProfile(bootstrapCtx, bootstrapClient)
	if err != nil {
		if apimeta.IsNoMatchError(err) {
			configLog.Info("TLS profile not available, using hardened defaults (non-OpenShift cluster)")
		} else {
			configLog.Error(err, "Failed to read APIServer TLS profile, using hardened defaults")
		}
		result.TLSOpts = append(result.TLSOpts, func(c *tls.Config) {
			c.MinVersion = tls.VersionTLS12
		})
	} else {
		result.HasOpenShiftConfigAPI = true
		tlsConfigFn, unsupportedCiphers := tlspkg.NewTLSConfigFromProfile(result.Profile)
		if len(unsupportedCiphers) > 0 {
			configLog.Info("Some ciphers from TLS profile are not supported by Go", "unsupported", unsupportedCiphers)
		}
		result.TLSOpts = append(result.TLSOpts, tlsConfigFn)
	}
	return result
}

// addTo applies the configuration to controller runtime Options.
func addTo(o *ctrl.Options, cfg *configapi.Configuration, enableHTTP2 bool, tlsResult TLSProfileResult) {
	var tlsOpts []func(*tls.Config)
	tlsOpts = append(tlsOpts, tlsResult.TLSOpts...)
	// ALPN must always be set for HTTP/2 support
	if enableHTTP2 {
		tlsOpts = append(tlsOpts, func(c *tls.Config) {
			c.NextProtos = []string{"h2", "http/1.1"}
		})
	} else {
		tlsOpts = append(tlsOpts, func(c *tls.Config) {
			c.NextProtos = []string{"h2", "http/1.1"}
		})
	}

	o.Metrics = metricsserver.Options{
		BindAddress:   cfg.Metrics.BindAddress,
		SecureServing: cfg.Metrics.SecureServing != nil && *cfg.Metrics.SecureServing,
		TLSOpts:       tlsOpts,
	}

	// Set webhook server options
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

// Load loads configuration from file and returns controller Options, Configuration, and TLS profile result.
// If configFile is empty, default configuration is used.
func Load(scheme *runtime.Scheme, configFile string, enableHTTP2 bool, restCfg *rest.Config) (ctrl.Options, configapi.Configuration, TLSProfileResult, error) {
	var tlsResult TLSProfileResult
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
			return options, cfg, tlsResult, err
		}
	}

	// Validate configuration
	if errs := validate(&cfg); len(errs) > 0 {
		return options, cfg, tlsResult, fmt.Errorf("invalid configuration: %v", errs.ToAggregate())
	}

	// Fetch cluster TLS profile
	tlsResult = FetchTLSProfile(restCfg)

	// Apply configuration to options
	addTo(&options, &cfg, enableHTTP2, tlsResult)

	return options, cfg, tlsResult, nil
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
// Returns true by default if not explicitly disabled.
func IsCertManagementEnabled(cfg *configapi.Configuration) bool {
	if cfg.CertManagement == nil || cfg.CertManagement.Enable == nil {
		return true // Enabled by default
	}
	return *cfg.CertManagement.Enable
}
