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

package statusserver

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/coreos/go-oidc/v3/oidc/oidctest"
	"github.com/google/go-cmp/cmp"
)

const (
	// testKeyID is the "kid" under which the test OIDC server publishes its RS256 signing key.
	testKeyID = "test-key"
	// testECKeyID is the "kid" of an ES256 key the test OIDC server publishes, although the
	// server only advertises RS256 as a supported signing algorithm.
	testECKeyID = "test-ec-key"
)

// newTestOIDCServer starts a local OIDC issuer that publishes keys in its JWKS, and returns it
// together with its issuer URL, which is the server's URL.
func newTestOIDCServer(t *testing.T, keys ...oidctest.PublicKey) (*oidctest.Server, string) {
	t.Helper()

	oidcServer := &oidctest.Server{PublicKeys: keys}
	srv := httptest.NewServer(oidcServer)
	t.Cleanup(srv.Close)
	oidcServer.SetIssuer(srv.URL)

	return oidcServer, srv.URL
}

// signToken returns a compact JWT carrying claims, signed with key under keyID using alg.
func signToken(t *testing.T, key crypto.PrivateKey, keyID, alg string, claims map[string]any) string {
	t.Helper()

	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("Failed to marshal JWT claims: %v", err)
	}
	return oidctest.SignIDToken(key, keyID, alg, string(payload))
}

// tokenClaims returns the claims of a projected service account token issued by issuer for
// the given audience to a pod in namespace, valid for the next hour.
func tokenClaims(issuer, audience, namespace string) map[string]any {
	now := time.Now()
	return map[string]any{
		"iss": issuer,
		"aud": []string{audience},
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
		"kubernetes.io": map[string]any{
			"namespace": namespace,
		},
	}
}

func TestProjectedServiceAccountTokenAuthorizerAuthorize(t *testing.T) {
	const (
		namespace    = "default"
		trainJobName = "test-job"
	)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}
	// otherKey is not published by the test OIDC server.
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate ECDSA key: %v", err)
	}
	_, issuer := newTestOIDCServer(t,
		oidctest.PublicKey{PublicKey: key.Public(), KeyID: testKeyID, Algorithm: oidc.RS256},
		oidctest.PublicKey{PublicKey: ecKey.Public(), KeyID: testECKeyID, Algorithm: oidc.ES256},
	)

	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		t.Fatalf("Failed to create OIDC provider: %v", err)
	}

	audience := TokenAudience(namespace, trainJobName)
	validToken := signToken(t, key, testKeyID, oidc.RS256, tokenClaims(issuer, audience, namespace))
	validTokenParts := strings.Split(validToken, ".")
	otherTokenParts := strings.Split(signToken(t, key, testKeyID, oidc.RS256, tokenClaims(issuer, TokenAudience(namespace, "other-job"), namespace)), ".")

	missingKubernetesClaims := tokenClaims(issuer, audience, namespace)
	delete(missingKubernetesClaims, "kubernetes.io")

	expiredClaims := tokenClaims(issuer, audience, namespace)
	expiredClaims["iat"] = time.Now().Add(-2 * time.Hour).Unix()
	expiredClaims["exp"] = time.Now().Add(-time.Hour).Unix()

	notYetValidClaims := tokenClaims(issuer, audience, namespace)
	notYetValidClaims["nbf"] = time.Now().Add(time.Hour).Unix()

	testcases := map[string]struct {
		oidcProvider   *oidc.Provider
		authHeader     string
		wantAuthorized bool
		wantErrMsg     string
	}{
		"uninitialized authorizer returns an error": {
			oidcProvider: nil,
			authHeader:   "Bearer " + validToken,
			wantErrMsg:   "OIDC provider has not been initialized",
		},
		"valid token for the train job is authorized": {
			oidcProvider:   provider,
			authHeader:     "Bearer " + validToken,
			wantAuthorized: true,
		},
		"empty authorization header is not authorized": {
			oidcProvider: provider,
			authHeader:   "",
		},
		"valid token under a non-Bearer scheme is not authorized": {
			oidcProvider: provider,
			authHeader:   "Basic " + validToken,
		},
		"token for a different train job is not authorized": {
			oidcProvider: provider,
			authHeader:   "Bearer " + signToken(t, key, testKeyID, oidc.RS256, tokenClaims(issuer, TokenAudience(namespace, "other-job"), namespace)),
		},
		"token for the same train job name in a different namespace is not authorized": {
			oidcProvider: provider,
			authHeader:   "Bearer " + signToken(t, key, testKeyID, oidc.RS256, tokenClaims(issuer, TokenAudience("other-namespace", trainJobName), namespace)),
		},
		"token from a pod in a different namespace is not authorized": {
			oidcProvider: provider,
			authHeader:   "Bearer " + signToken(t, key, testKeyID, oidc.RS256, tokenClaims(issuer, audience, "other-namespace")),
		},
		"token without the kubernetes.io claim is not authorized": {
			oidcProvider: provider,
			authHeader:   "Bearer " + signToken(t, key, testKeyID, oidc.RS256, missingKubernetesClaims),
		},
		"expired token is not authorized": {
			oidcProvider: provider,
			authHeader:   "Bearer " + signToken(t, key, testKeyID, oidc.RS256, expiredClaims),
		},
		"token that is not yet valid is not authorized": {
			oidcProvider: provider,
			authHeader:   "Bearer " + signToken(t, key, testKeyID, oidc.RS256, notYetValidClaims),
		},
		"token signed with an unpublished key is not authorized": {
			oidcProvider: provider,
			authHeader:   "Bearer " + signToken(t, otherKey, testKeyID, oidc.RS256, tokenClaims(issuer, audience, namespace)),
		},
		"token with its signature stripped is not authorized": {
			oidcProvider: provider,
			authHeader:   "Bearer " + validTokenParts[0] + "." + validTokenParts[1] + ".",
		},
		"token with the signature of another token is not authorized": {
			oidcProvider: provider,
			authHeader:   "Bearer " + validTokenParts[0] + "." + validTokenParts[1] + "." + otherTokenParts[2],
		},
		"token that is not a JWT is not authorized": {
			oidcProvider: provider,
			authHeader:   "Bearer not-a-jwt",
		},
		"token signed with an unsupported algorithm is not authorized": {
			oidcProvider: provider,
			authHeader:   "Bearer " + signToken(t, ecKey, testECKeyID, oidc.ES256, tokenClaims(issuer, audience, namespace)),
		},
		"token from a different issuer is not authorized": {
			oidcProvider: provider,
			authHeader:   "Bearer " + signToken(t, key, testKeyID, oidc.RS256, tokenClaims("https://other-issuer.example.com", audience, namespace)),
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			authorizer := &projectedServiceAccountTokenAuthorizer{oidcProvider: tc.oidcProvider}

			gotAuthorized, err := authorizer.Authorize(ctx, tc.authHeader, namespace, trainJobName)

			var gotErrMsg string
			if err != nil {
				gotErrMsg = err.Error()
			}
			if diff := cmp.Diff(tc.wantErrMsg, gotErrMsg); len(diff) != 0 {
				t.Errorf("Unexpected error (-want,+got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantAuthorized, gotAuthorized); len(diff) != 0 {
				t.Errorf("Unexpected authorization (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestProjectedServiceAccountTokenAuthorizerAuthorizeKeyRotation(t *testing.T) {
	const (
		namespace    = "default"
		trainJobName = "test-job"
		rotatedKeyID = "rotated-key"
	)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}
	rotatedKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}
	oidcServer, issuer := newTestOIDCServer(t,
		oidctest.PublicKey{PublicKey: key.Public(), KeyID: testKeyID, Algorithm: oidc.RS256},
	)

	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		t.Fatalf("Failed to create OIDC provider: %v", err)
	}
	authorizer := &projectedServiceAccountTokenAuthorizer{oidcProvider: provider}

	claims := tokenClaims(issuer, TokenAudience(namespace, trainJobName), namespace)
	tokenBeforeRotation := "Bearer " + signToken(t, key, testKeyID, oidc.RS256, claims)
	tokenAfterRotation := "Bearer " + signToken(t, rotatedKey, rotatedKeyID, oidc.RS256, claims)

	steps := []struct {
		name           string
		authHeader     string
		rotateKeys     bool
		wantAuthorized bool
	}{
		{name: "token signed with the published key is authorized, caching the JWKS", authHeader: tokenBeforeRotation, wantAuthorized: true},
		{name: "token signed with a key not yet published is not authorized", authHeader: tokenAfterRotation},
		{name: "token signed with the rotated key is authorized once it is published", authHeader: tokenAfterRotation, rotateKeys: true, wantAuthorized: true},
	}
	for _, step := range steps {
		if step.rotateKeys {
			oidcServer.PublicKeys = append(oidcServer.PublicKeys,
				oidctest.PublicKey{PublicKey: rotatedKey.Public(), KeyID: rotatedKeyID, Algorithm: oidc.RS256})
		}

		gotAuthorized, err := authorizer.Authorize(ctx, step.authHeader, namespace, trainJobName)

		if err != nil {
			t.Fatalf("%s: unexpected error: %v", step.name, err)
		}
		if diff := cmp.Diff(step.wantAuthorized, gotAuthorized); len(diff) != 0 {
			t.Fatalf("%s: unexpected authorization (-want,+got):\n%s", step.name, diff)
		}
	}
}

func TestExtractRawToken(t *testing.T) {
	testcases := map[string]struct {
		authHeader string
		wantToken  string
	}{
		"bearer token is extracted": {
			authHeader: "Bearer token",
			wantToken:  "token",
		},
		"scheme is matched case-insensitively": {
			authHeader: "bearer token",
			wantToken:  "token",
		},
		"repeated whitespace between scheme and credentials is tolerated": {
			authHeader: "Bearer \t token",
			wantToken:  "token",
		},
		"surrounding whitespace is tolerated": {
			authHeader: "  Bearer token  ",
			wantToken:  "token",
		},
		"empty header yields no token": {
			authHeader: "",
			wantToken:  "",
		},
		"whitespace-only header yields no token": {
			authHeader: "   ",
			wantToken:  "",
		},
		"missing credentials yield no token": {
			authHeader: "Bearer",
			wantToken:  "",
		},
		"missing scheme yields no token": {
			authHeader: "token",
			wantToken:  "",
		},
		"other authorization schemes yield no token": {
			authHeader: "Basic dXNlcjpwYXNz",
			wantToken:  "",
		},
		"scheme substring is not accepted": {
			authHeader: "Bearerx token",
			wantToken:  "",
		},
		"multiple credentials yield no token": {
			authHeader: "Bearer token another",
			wantToken:  "",
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			got := extractRawToken(tc.authHeader)

			if diff := cmp.Diff(tc.wantToken, got); len(diff) != 0 {
				t.Errorf("Unexpected token (-want,+got):\n%s", diff)
			}
		})
	}
}
