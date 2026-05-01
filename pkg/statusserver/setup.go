/*
Copyright 2026 The Kubeflow Authors.

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

package statusserver

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/util/cert"
)

func SetupServer(mgr ctrl.Manager, cfg *configapi.StatusServer, enableHTTP2 bool) error {
	tlsConfig, err := cert.SetupTLSConfig(mgr, enableHTTP2)
	if err != nil {
		return err
	}

	// Create a separate client with its own QPS/Burst limits
	// to avoid impacting the main reconciler's rate limits
	cli, err := createClient(mgr, cfg)
	if err != nil {
		return err
	}

	// Initialize OIDC provider for token authentication
	// The provider will be used to create verifiers with TrainJob-specific audiences
	authorizer := NewProjectedServiceAccountTokenAuthorizer(mgr.GetConfig())

	server, err := NewServer(cli, cfg, tlsConfig, authorizer)
	if err != nil {
		return err
	}
	return mgr.Add(server)
}

// RegisterProbes registers the status server with the manager's healthz and readyz probes.
// Must be called before mgr.Start(). Uses a TLS dial to verify the server is reachable,
// following the same pattern as the webhook server probe.
func RegisterProbes(mgr ctrl.Manager, cfg *configapi.StatusServer) error {
	addr := fmt.Sprintf(":%d", *cfg.Port)
	tlsCfg := &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	checker := healthz.Checker(func(_ *http.Request) error {
		conn, err := tls.DialWithDialer(
			&net.Dialer{Timeout: 10 * time.Second},
			"tcp",
			addr,
			tlsCfg,
		)
		if err != nil {
			return fmt.Errorf("status server not reachable at %s: %w", addr, err)
		}
		return conn.Close()
	})
	if err := mgr.AddHealthzCheck("status-server", checker); err != nil {
		return fmt.Errorf("unable to set up status server health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("status-server", checker); err != nil {
		return fmt.Errorf("unable to set up status server ready check: %w", err)
	}
	return nil
}

func createClient(mgr ctrl.Manager, cfg *configapi.StatusServer) (client.Client, error) {
	// Copy the manager's rest config and override rate limits
	mgrCfg := rest.CopyConfig(mgr.GetConfig())
	if cfg.QPS != nil {
		mgrCfg.QPS = *cfg.QPS
	}
	if cfg.Burst != nil {
		mgrCfg.Burst = int(*cfg.Burst)
	}

	cli, err := client.New(mgrCfg, client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create status server client: %w", err)
	}

	return cli, nil
}
