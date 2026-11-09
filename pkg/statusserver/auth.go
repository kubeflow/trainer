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
	"context"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"k8s.io/client-go/rest"
)

type TokenAuthorizer interface {
	Init(ctx context.Context) error
	Authorize(ctx context.Context, rawIDToken, namespace, trainJobName string) (bool, error)
}

type projectedServiceAccountTokenAuthorizer struct {
	oidcProvider *oidc.Provider
	config       *rest.Config
}

var _ TokenAuthorizer = &projectedServiceAccountTokenAuthorizer{}

// projectedToken is the decoded JWT claims of a k8s projected service account token.
// Note: we only decode the subset of claims we actually need.
type projectedToken struct {
	Issuer     string `json:"iss"`
	Kubernetes struct {
		Namespace string `json:"namespace"`
	} `json:"kubernetes.io"`
}

// NewProjectedServiceAccountTokenAuthorizer creates a validator for checking a bearer token has permission to
// update the requested train job. Tokens are validated locally using OIDC discovery from the Kubernetes
// API server's internal endpoint, avoiding TokenReview API calls that could cause DOS.
func NewProjectedServiceAccountTokenAuthorizer(config *rest.Config) TokenAuthorizer {
	return &projectedServiceAccountTokenAuthorizer{
		config: config,
	}
}

func (p *projectedServiceAccountTokenAuthorizer) Init(ctx context.Context) error {
	// Use the Kubernetes API server's internal OIDC discovery endpoint instead
	// of the external issuer URL. This works on all clusters (EKS, GKE, AKS, KinD)
	// without requiring external network access or encountering TLS cert issues.
	// The API server exposes /.well-known/openid-configuration and /openid/v1/jwks
	// endpoints that can be accessed with the authenticated in-cluster client.
	apiServerHost := p.config.Host

	// Debug logging to help diagnose KinD issues
	fmt.Printf("DEBUG: Initializing OIDC provider with API server: %s\n", apiServerHost)
	fmt.Printf("DEBUG: rest.Config CAData length: %d\n", len(p.config.CAData))
	fmt.Printf("DEBUG: rest.Config CAFile: %s\n", p.config.CAFile)
	fmt.Printf("DEBUG: rest.Config TLS ServerName: %s\n", p.config.ServerName)

	// Create an authenticated HTTP client using the provided rest config
	httpClient, err := rest.HTTPClientFor(p.config)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Create context with the authenticated HTTP client
	ctx = oidc.ClientContext(ctx, httpClient)

	// Point OIDC provider to API server's internal OIDC endpoints
	fmt.Printf("DEBUG: Attempting to create OIDC provider...\n")
	provider, err := oidc.NewProvider(ctx, apiServerHost)
	if err != nil {
		fmt.Printf("DEBUG: OIDC provider creation failed: %v\n", err)
		return fmt.Errorf("failed to create OIDC provider from API server (%s): %w", apiServerHost, err)
	}
	p.oidcProvider = provider

	fmt.Printf("DEBUG: OIDC provider initialized successfully\n")
	return nil
}

func (p *projectedServiceAccountTokenAuthorizer) Authorize(ctx context.Context, authHeader, namespace, trainJobName string) (bool, error) {
	if p.oidcProvider == nil {
		return false, fmt.Errorf("OIDC provider has not been initialized")
	}

	rawToken := extractRawToken(authHeader)

	// Create authorizer with TrainJob-specific audience
	expectedAudience := TokenAudience(namespace, trainJobName)
	verifier := p.oidcProvider.Verifier(&oidc.Config{
		ClientID: expectedAudience,
	})

	// Check token signature, expiry, and audience
	idToken, err := verifier.Verify(ctx, rawToken)
	if err != nil {
		return false, nil
	}

	// Check token is bound to a pod in the same namespace as the train job
	parsedToken := projectedToken{}
	err = idToken.Claims(&parsedToken)
	if err != nil {
		return false, nil
	}
	if parsedToken.Kubernetes.Namespace != namespace {
		return false, nil
	}

	return true, nil
}

// extractRawToken returns the credentials of an RFC 7235 "Bearer" Authorization header.
// The scheme is matched case-insensitively, and any amount of surrounding or separating
// whitespace is tolerated. An empty string is returned when the header does not carry
// exactly one bearer credential.
func extractRawToken(authHeader string) string {
	parts := strings.Fields(authHeader)

	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}

	return parts[1]
}
