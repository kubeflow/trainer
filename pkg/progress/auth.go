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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
)

const (
	audience = "trainer.kubeflow.org"
)

// TokenVerifier verifies OIDC tokens and returns decoded service account token claims.
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (*ProjectedServiceAccountToken, error)
}

// ProjectedServiceAccountToken is a decoded kubernetes projected service account token
type ProjectedServiceAccountToken struct {
	Issuer     string           `json:"iss"`           // e.g. https://kubernetes.default.svc
	Subject    string           `json:"sub"`           // system:serviceaccount:<namespace>:<name>
	Audience   []string         `json:"aud,omitempty"` // audiences the token is valid for
	Expiry     int64            `json:"exp"`           // expiration time (unix)
	NotBefore  int64            `json:"nbf,omitempty"` // not valid before (unix)
	IssuedAt   int64            `json:"iat"`           // issued at (unix)
	Kubernetes KubernetesClaims `json:"kubernetes.io"`
}

type KubernetesClaims struct {
	Namespace      string         `json:"namespace"`
	ServiceAccount ServiceAccount `json:"serviceaccount"`
	Pod            *Pod           `json:"pod,omitempty"`
}

// ServiceAccount identifies the service account.
type ServiceAccount struct {
	Name string    `json:"name"`
	UID  types.UID `json:"uid"`
}

// Pod is included when the token is bound to a pod.
type Pod struct {
	Name string    `json:"name"`
	UID  types.UID `json:"uid"`
}

// projectedServiceAccountTokenVerifier wraps an OIDC verifier and decodes tokens into ProjectedServiceAccountToken.
type projectedServiceAccountTokenVerifier struct {
	oidcVerifier *oidc.IDTokenVerifier
}

// Verify validates the token and decodes it into a ProjectedServiceAccountToken.
func (v *projectedServiceAccountTokenVerifier) Verify(ctx context.Context, token string) (*ProjectedServiceAccountToken, error) {
	idToken, err := v.oidcVerifier.Verify(ctx, token)
	if err != nil {
		return nil, err
	}

	// Decode the token into a struct
	var saToken ProjectedServiceAccountToken
	if err := idToken.Claims(&saToken); err != nil {
		return nil, fmt.Errorf("failed to decode token claims: %w", err)
	}

	return &saToken, nil
}

// NewProjectedServiceAccountTokenVerifier creates an OIDC token verifier for validating Kubernetes
// projected service account tokens against the in-cluster OIDC issuer.
func NewProjectedServiceAccountTokenVerifier(ctx context.Context, config *rest.Config) (TokenVerifier, error) {
	issuerURL, err := getClusterOIDCIssuerURL()
	if err != nil {
		return nil, fmt.Errorf("failed to discover issuer URL: %w", err)
	}

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

	return &projectedServiceAccountTokenVerifier{
		oidcVerifier: verifier,
	}, nil
}

type projectServiceAccountTokenContextKey struct{}

func withServiceAccountToken(ctx context.Context, token ProjectedServiceAccountToken) context.Context {
	return context.WithValue(ctx, projectServiceAccountTokenContextKey{}, &token)
}

func serviceAccountTokenFromContext(ctx context.Context) (*ProjectedServiceAccountToken, bool) {
	t, ok := ctx.Value(projectServiceAccountTokenContextKey{}).(*ProjectedServiceAccountToken)
	return t, ok
}

// getClusterOIDCIssuerURL tries to look up the cluster token issuer from the in-cluster service account token
// Different clusters may use different issuers. This is a reliable way of discovering the issuer.
func getClusterOIDCIssuerURL() (string, error) {
	tokenBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return "", err
	}

	parts := strings.Split(strings.TrimSpace(string(tokenBytes)), ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("serviceaccount token is not a jwt")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("serviceaccount token is not a jwt: %w", err)
	}

	var claims struct {
		Issuer string `json:"iss"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("serviceaccount token is not a jwt: %w", err)
	}

	if claims.Issuer == "" {
		return "", fmt.Errorf("serviceaccount token missing issuer claim")
	}

	return claims.Issuer, nil
}
