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

package progress

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/util/cert"
)

func SetupServer(mgr ctrl.Manager, cfg *configapi.ProgressServer, enableHTTP2 bool) error {
	tlsConfig, err := cert.SetupTLSConfig(mgr, enableHTTP2)
	if err != nil {
		return err
	}

	// Create a separate client with its own QPS/Burst limits
	// to avoid impacting the main reconciler's rate limits
	progressClient, err := createClient(mgr, cfg)
	if err != nil {
		return err
	}

	// Initialize OIDC verifier for token authentication
	verifier, err := NewProjectedServiceAccountTokenVerifier(context.Background(), mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("failed to create projected service account token verifier: %w", err)
	}

	server, err := NewServer(progressClient, cfg, tlsConfig, verifier)
	if err != nil {
		return err
	}
	return mgr.Add(server)
}

func createClient(mgr ctrl.Manager, cfg *configapi.ProgressServer) (client.Client, error) {
	// Clone the manager's rest config and override rate limits
	progressConfig := *mgr.GetConfig()
	if cfg.QPS != nil {
		progressConfig.QPS = *cfg.QPS
	}
	if cfg.Burst != nil {
		progressConfig.Burst = int(*cfg.Burst)
	}

	progressClient, err := client.New(&progressConfig, client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create progress server client: %w", err)
	}

	return progressClient, nil
}
