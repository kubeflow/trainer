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
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/go-cmp/cmp"
)

// testKeyID is the "kid" under which the test OIDC server publishes its signing key.
const testKeyID = "test-key"

// newTestOIDCServer starts a local OIDC issuer that serves a discovery document and a JWKS
// publishing pub under testKeyID. The issuer URL is the server's URL.
func newTestOIDCServer(t *testing.T, pub *rsa.PublicKey) *httptest.Server {
	t.Helper()

	jwks, err := json.Marshal(map[string]any{
		"keys": []map[string]string{{
			"kty": "RSA",
			"use": "sig",
			"alg": "RS256",
			"kid": testKeyID,
			"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
		}},
	})
	if err != nil {
		t.Fatalf("Failed to marshal JWKS: %v", err)
	}

	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{
			"issuer":                                srv.URL,
			"jwks_uri":                              srv.URL + "/keys",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(jwks); err != nil {
			t.Errorf("Failed to write JWKS: %v", err)
		}
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Errorf("Failed to write JSON response: %v", err)
	}
}

// signToken returns a compact RS256 JWT carrying claims, signed with key under testKeyID.
func signToken(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()

	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": testKeyID})
	if err != nil {
		t.Fatalf("Failed to marshal JWT header: %v", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("Failed to marshal JWT claims: %v", err)
	}

	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("Failed to sign JWT: %v", err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
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
	srv := newTestOIDCServer(t, &key.PublicKey)

	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, srv.URL)
	if err != nil {
		t.Fatalf("Failed to create OIDC provider: %v", err)
	}

	audience := TokenAudience(namespace, trainJobName)
	validToken := signToken(t, key, tokenClaims(srv.URL, audience, namespace))

	missingKubernetesClaims := tokenClaims(srv.URL, audience, namespace)
	delete(missingKubernetesClaims, "kubernetes.io")

	expiredClaims := tokenClaims(srv.URL, audience, namespace)
	expiredClaims["iat"] = time.Now().Add(-2 * time.Hour).Unix()
	expiredClaims["exp"] = time.Now().Add(-time.Hour).Unix()

	testcases := map[string]struct {
		oidcProvider   *oidc.Provider
		authHeader     string
		wantAuthorized bool
		wantErr        bool
	}{
		// Proves Authorize fails loudly, rather than denying silently, when Init has not run.
		"uninitialized authorizer returns an error": {
			oidcProvider: nil,
			authHeader:   "Bearer " + validToken,
			wantErr:      true,
		},
		// Proves a token signed by the issuer, for this train job, from its namespace is accepted.
		"valid token for the train job is authorized": {
			oidcProvider:   provider,
			authHeader:     "Bearer " + validToken,
			wantAuthorized: true,
		},
		// Proves a request without credentials is denied.
		"empty authorization header is not authorized": {
			oidcProvider: provider,
			authHeader:   "",
		},
		// Proves a valid token is only accepted as a Bearer credential.
		"valid token under a non-Bearer scheme is not authorized": {
			oidcProvider: provider,
			authHeader:   "Basic " + validToken,
		},
		// Proves a token minted for one train job cannot update another train job.
		"token for a different train job is not authorized": {
			oidcProvider: provider,
			authHeader:   "Bearer " + signToken(t, key, tokenClaims(srv.URL, TokenAudience(namespace, "other-job"), namespace)),
		},
		// Proves a token minted for a same-named train job in another namespace is rejected by audience.
		"token for the same train job name in a different namespace is not authorized": {
			oidcProvider: provider,
			authHeader:   "Bearer " + signToken(t, key, tokenClaims(srv.URL, TokenAudience("other-namespace", trainJobName), namespace)),
		},
		// Proves a pod in another namespace cannot use a token carrying this train job's audience.
		"token from a pod in a different namespace is not authorized": {
			oidcProvider: provider,
			authHeader:   "Bearer " + signToken(t, key, tokenClaims(srv.URL, audience, "other-namespace")),
		},
		// Proves the namespace binding is required. A missing kubernetes.io claim decodes to an
		// empty namespace, which cannot match a non-empty one.
		"token without the kubernetes.io claim is not authorized": {
			oidcProvider: provider,
			authHeader:   "Bearer " + signToken(t, key, missingKubernetesClaims),
		},
		// Proves expiry is enforced; exp is an hour in the past, far outside any clock skew.
		"expired token is not authorized": {
			oidcProvider: provider,
			authHeader:   "Bearer " + signToken(t, key, expiredClaims),
		},
		// Proves the signature is checked against the issuer's published key, even when the kid matches.
		"token signed with an unpublished key is not authorized": {
			oidcProvider: provider,
			authHeader:   "Bearer " + signToken(t, otherKey, tokenClaims(srv.URL, audience, namespace)),
		},
		// Proves a correctly signed token claiming a different issuer is rejected.
		"token from a different issuer is not authorized": {
			oidcProvider: provider,
			authHeader:   "Bearer " + signToken(t, key, tokenClaims("https://other-issuer.example.com", audience, namespace)),
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			authorizer := &projectedServiceAccountTokenAuthorizer{oidcProvider: tc.oidcProvider}

			gotAuthorized, err := authorizer.Authorize(ctx, tc.authHeader, namespace, trainJobName)

			if gotErr := err != nil; gotErr != tc.wantErr {
				t.Errorf("Unexpected error, wantErr: %v, got: %v", tc.wantErr, err)
			}
			if diff := cmp.Diff(tc.wantAuthorized, gotAuthorized); len(diff) != 0 {
				t.Errorf("Unexpected authorization (-want,+got):\n%s", diff)
			}
		})
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
