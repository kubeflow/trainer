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
	"strings"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/features"
	"github.com/kubeflow/trainer/v2/pkg/util/cert"
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

// healthEndpointPath returns the probe endpoint name as a rooted path.
func healthEndpointPath(name string) string {
	// controller-runtime registers the name as an http.ServeMux pattern, which panics on
	// a pattern that has no leading slash, and the API and the shipped manifests spell
	// the names without one.
	if name == "" || strings.HasPrefix(name, "/") {
		return name
	}
	return "/" + name
}

// addTo applies the configuration to controller runtime Options.
func addTo(o *ctrl.Options, cfg *configapi.Configuration) {
	tlsOpts := []func(*tls.Config){
		func(c *tls.Config) {
			tlsconfig.Apply(c, cfg.TLS)
		},
	}

	o.Metrics = metricsserver.Options{
		// The metrics server is always started manually in setupManagerComponents
		// after certificates are guaranteed to be present. Disable the manager's
		// built-in metrics server to prevent it from racing for the same port.
		BindAddress: "0",
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
	o.ReadinessEndpointName = healthEndpointPath(cfg.Health.ReadinessEndpointName)
	o.LivenessEndpointName = healthEndpointPath(cfg.Health.LivenessEndpointName)

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

// ApplyNetworkPolicyCache configures infrastructure watches after feature gates are resolved.
func ApplyNetworkPolicyCache(o *ctrl.Options, cfg *configapi.Configuration) {
	if cfg.NetworkPolicy == nil || !ptr.Deref(cfg.NetworkPolicy.Enabled, false) {
		return
	}
	namespaces := map[string]cache.Config{corev1.NamespaceDefault: {}, cert.GetOperatorNamespace(): {}}
	o.Cache.ByObject = map[client.Object]cache.ByObject{
		&discoveryv1.EndpointSlice{}: {Namespaces: namespaces},
	}
	// OptimizationJob owns Services in user namespaces and needs its existing cluster-wide watch.
	if !features.Enabled(features.OptimizationJob) {
		o.Cache.ByObject[&corev1.Service{}] = cache.ByObject{Namespaces: namespaces}
	}
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
