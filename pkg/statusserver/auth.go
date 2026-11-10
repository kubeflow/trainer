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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"k8s.io/client-go/rest"
)

type TokenAuthorizer interface {
	Init(ctx context.Context) error
	Authorize(ctx context.Context, rawIDToken, namespace, trainJobName string) (bool, error)
}

type projectedServiceAccountTokenAuthorizer struct {
	verifier *oidc.IDTokenVerifier
	keySet   oidc.KeySet
	issuer   string
	config   *rest.Config
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

// serviceAccountTokenPath is the default path where Kubernetes mounts the service account token
const serviceAccountTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"

// internalAPIServerHost is the internal DNS name for the Kubernetes API server
const internalAPIServerHost = "https://kubernetes.default.svc.cluster.local"

// NewProjectedServiceAccountTokenAuthorizer creates a validator for checking a bearer token has permission to
// update the requested train job. Tokens are validated locally using OIDC discovery from the Kubernetes
// API server's internal endpoint, avoiding TokenReview API calls that could cause DOS.
func NewProjectedServiceAccountTokenAuthorizer(config *rest.Config) TokenAuthorizer {
	return &projectedServiceAccountTokenAuthorizer{
		config: config,
	}
}

func (p *projectedServiceAccountTokenAuthorizer) Init(ctx context.Context) error {
	// Step 1: Read the issuer from the controller's own mounted service account token
	// This works on all clusters (KinD, EKS, GKE, AKS) regardless of their issuer configuration
	issuer, err := readIssuerFromServiceAccountToken()
	if err != nil {
		return fmt.Errorf("failed to read issuer from service account token: %w", err)
	}
	p.issuer = issuer

	// Create an authenticated HTTP client using the provided rest config
	httpClient, err := rest.HTTPClientFor(p.config)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Create context with the authenticated HTTP client
	ctx = oidc.ClientContext(ctx, httpClient)

	// Step 2: Verify the internal discovery endpoint is accessible and get supported algorithms
	// We fetch from internal endpoint but we DON'T pass internalAPIServerHost to NewProvider
	// because that would fail issuer validation. Instead, we fetch discovery manually.
	discoveryURL := internalAPIServerHost + "/.well-known/openid-configuration"
	resp, err := httpClient.Get(discoveryURL)
	if err != nil {
		return fmt.Errorf("failed to fetch discovery document from %s: %w", discoveryURL, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("discovery document request failed with status %d", resp.StatusCode)
	}

	var discoveryDoc struct {
		Issuer     string   `json:"issuer"`
		JWKSURI    string   `json:"jwks_uri"`
		Algorithms []string `json:"id_token_signing_alg_values_supported"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&discoveryDoc); err != nil {
		return fmt.Errorf("failed to parse discovery document: %w", err)
	}

	// Verify the discovery document's issuer matches what we read from the token
	if discoveryDoc.Issuer != issuer {
		return fmt.Errorf("issuer mismatch: token has %q but discovery document advertises %q", issuer, discoveryDoc.Issuer)
	}

	// Verify RS256 is supported
	supportsRS256 := false
	for _, alg := range discoveryDoc.Algorithms {
		if alg == "RS256" {
			supportsRS256 = true
			break
		}
	}
	if !supportsRS256 {
		return fmt.Errorf("OIDC provider does not support RS256 algorithm (required for service account tokens)")
	}

	// Step 3: Build the key set explicitly against the internal JWKS endpoint
	// This ignores the discovery document's jwks_uri (which points to external endpoints on EKS/GKE)
	// and instead uses the internal JWKS endpoint directly
	internalJWKSURL := internalAPIServerHost + "/openid/v1/jwks"
	keySet := oidc.NewRemoteKeySet(ctx, internalJWKSURL)
	p.keySet = keySet

	// Step 4: Construct the verifier with the real cluster issuer and internal KeySet
	// The verifier will check tokens against the actual cluster issuer (from Step 1)
	// while fetching keys from the internal endpoint (from Step 3)
	p.verifier = oidc.NewVerifier(issuer, keySet, &oidc.Config{
		SkipClientIDCheck: true, // We check audience separately in Authorize()
	})

	return nil
}

// readIssuerFromServiceAccountToken reads the controller's own service account token
// and extracts the issuer claim. This gives us the real cluster issuer regardless of
// whether it's kubernetes.default.svc.cluster.local (KinD), an EKS URL, or a GKE URL.
func readIssuerFromServiceAccountToken() (string, error) {
	// Read the token file
	tokenBytes, err := os.ReadFile(serviceAccountTokenPath)
	if err != nil {
		return "", fmt.Errorf("failed to read service account token: %w", err)
	}

	token := string(tokenBytes)

	// JWT tokens have three parts separated by dots: header.payload.signature
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("service account token is not a valid JWT (expected 3 parts, got %d)", len(parts))
	}

	// Decode the payload (second part)
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode token payload: %w", err)
	}

	// Parse the payload to extract the issuer
	var claims struct {
		Issuer string `json:"iss"`
	}
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return "", fmt.Errorf("failed to parse token claims: %w", err)
	}

	if claims.Issuer == "" {
		return "", fmt.Errorf("service account token does not contain an issuer claim")
	}

	return claims.Issuer, nil
}

func (p *projectedServiceAccountTokenAuthorizer) Authorize(ctx context.Context, authHeader, namespace, trainJobName string) (bool, error) {
	if p.verifier == nil {
		return false, fmt.Errorf("OIDC verifier has not been initialized")
	}

	rawToken := extractRawToken(authHeader)
	if rawToken == "" {
		return false, nil
	}

	// Create a verifier with TrainJob-specific audience check
	expectedAudience := TokenAudience(namespace, trainJobName)

	// Create a new verifier with audience check for this specific request
	// We reuse the stored keySet which points to the internal JWKS endpoint
	verifierWithAudience := oidc.NewVerifier(p.issuer, p.keySet, &oidc.Config{
		ClientID: expectedAudience,
	})

	// Verify token signature, expiry, issuer, and audience
	idToken, err := verifierWithAudience.Verify(ctx, rawToken)
	if err != nil {
		// Token verification failed (invalid signature, expired, wrong audience, etc.)
		return false, nil
	}

	// Check token is bound to a pod in the same namespace as the train job
	parsedToken := projectedToken{}
	if err := idToken.Claims(&parsedToken); err != nil {
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
