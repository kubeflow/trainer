/*
Copyright The Kubeflow Authors.

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

package metrics

import (
	"crypto/tls"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/util/tlsconfig"
)

// +kubebuilder:rbac:groups=authentication.k8s.io,resources=tokenreviews,verbs=create
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create

// SetupServer creates and registers a metrics server with the manager.
// It must be called only after the serving certificates are available on disk —
// typically inside setupManagerComponents after certsReady fires.
func SetupServer(mgr ctrl.Manager, cfg *configapi.ControllerMetrics, tlsOpts *configapi.TLSOptions) error {
	secureServing := cfg.SecureServing != nil && *cfg.SecureServing

	opts := metricsserver.Options{
		SecureServing: secureServing,
		BindAddress:   cfg.BindAddress,
	}

	if secureServing {
		opts.TLSOpts = []func(*tls.Config){
			func(c *tls.Config) {
				tlsconfig.Apply(c, tlsOpts)
			},
		}
	}

	if cfg.Auth != nil && cfg.Auth.Enabled != nil && *cfg.Auth.Enabled {
		opts.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	server, err := metricsserver.NewServer(opts, mgr.GetConfig(), mgr.GetHTTPClient())
	if err != nil {
		return fmt.Errorf("failed to create metrics server: %w", err)
	}

	if server == nil {
		// metrics server is disabled: opts.BindAddress="0"
		return nil
	}

	return mgr.Add(server)
}
