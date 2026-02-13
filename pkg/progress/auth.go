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

	"github.com/coreos/go-oidc/v3/oidc"
	"k8s.io/client-go/rest"
)

const (
	// issuerUrl is the in-cluster issuer
	issuerURL = "https://kubernetes.default.svc"
	audience  = "trainer.kubeflow.org"
)

// TokenVerifier verifies OIDC tokens.
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (*oidc.IDToken, error)
}

// NewProjectedServiceAccountTokenVerifier creates an OIDC token verifier for validating Kubernetes
// projected service account tokens against the in-cluster OIDC issuer.
func NewProjectedServiceAccountTokenVerifier(ctx context.Context, config *rest.Config) (TokenVerifier, error) {

	// Create an authenticated HTTP client using the provided rest config
	httpClient, err := rest.HTTPClientFor(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Create context with the authenticated HTTP client
	ctx = oidc.ClientContext(ctx, httpClient)

	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: audience,
	})

	return verifier, nil
}
