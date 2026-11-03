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
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	authenticationv1 "k8s.io/api/authentication/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestTokenReviewAuthorizerAuthorize(t *testing.T) {
	const (
		namespace    = "default"
		trainJobName = "test-job"
		token        = "token"
	)
	audience := TokenAudience(namespace, trainJobName)

	testcases := map[string]struct {
		authHeader     string
		reviewStatus   authenticationv1.TokenReviewStatus
		reviewErr      error
		wantReview     *authenticationv1.TokenReviewSpec
		wantAuthorized bool
		wantErrMsg     string
	}{
		"token reviewed for the train job audience is authorized": {
			authHeader: "Bearer " + token,
			reviewStatus: authenticationv1.TokenReviewStatus{
				Authenticated: true,
				Audiences:     []string{audience},
				User:          authenticationv1.UserInfo{Username: "system:serviceaccount:default:trainer"},
			},
			wantReview:     &authenticationv1.TokenReviewSpec{Token: token, Audiences: []string{audience}},
			wantAuthorized: true,
		},
		"empty authorization header is not authorized without reviewing a token": {
			authHeader: "",
		},
		"token under a non-Bearer scheme is not authorized without reviewing a token": {
			authHeader: "Basic " + token,
		},
		"unauthenticated token is not authorized": {
			authHeader:   "Bearer " + token,
			reviewStatus: authenticationv1.TokenReviewStatus{Authenticated: false, Error: "token has expired"},
			wantReview:   &authenticationv1.TokenReviewSpec{Token: token, Audiences: []string{audience}},
		},
		"token authenticated without the train job audience is not authorized": {
			authHeader: "Bearer " + token,
			reviewStatus: authenticationv1.TokenReviewStatus{
				Authenticated: true,
				User:          authenticationv1.UserInfo{Username: "system:serviceaccount:default:trainer"},
			},
			wantReview: &authenticationv1.TokenReviewSpec{Token: token, Audiences: []string{audience}},
		},
		"token authenticated for a different audience is not authorized": {
			authHeader: "Bearer " + token,
			reviewStatus: authenticationv1.TokenReviewStatus{
				Authenticated: true,
				Audiences:     []string{TokenAudience(namespace, "other-job")},
				User:          authenticationv1.UserInfo{Username: "system:serviceaccount:default:trainer"},
			},
			wantReview: &authenticationv1.TokenReviewSpec{Token: token, Audiences: []string{audience}},
		},
		"token of a service account in a different namespace is not authorized": {
			authHeader: "Bearer " + token,
			reviewStatus: authenticationv1.TokenReviewStatus{
				Authenticated: true,
				Audiences:     []string{audience},
				User:          authenticationv1.UserInfo{Username: "system:serviceaccount:other-namespace:trainer"},
			},
			wantReview: &authenticationv1.TokenReviewSpec{Token: token, Audiences: []string{audience}},
		},
		"token of a user that is not a service account is not authorized": {
			authHeader: "Bearer " + token,
			reviewStatus: authenticationv1.TokenReviewStatus{
				Authenticated: true,
				Audiences:     []string{audience},
				User:          authenticationv1.UserInfo{Username: "default"},
			},
			wantReview: &authenticationv1.TokenReviewSpec{Token: token, Audiences: []string{audience}},
		},
		"token of a malformed service account username is not authorized": {
			authHeader: "Bearer " + token,
			reviewStatus: authenticationv1.TokenReviewStatus{
				Authenticated: true,
				Audiences:     []string{audience},
				User:          authenticationv1.UserInfo{Username: "system:serviceaccount:default"},
			},
			wantReview: &authenticationv1.TokenReviewSpec{Token: token, Audiences: []string{audience}},
		},
		"failure to review the token returns an error": {
			authHeader: "Bearer " + token,
			reviewErr:  errors.New("connection refused"),
			wantReview: &authenticationv1.TokenReviewSpec{Token: token, Audiences: []string{audience}},
			wantErrMsg: "failed to review token: connection refused",
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			var gotReview *authenticationv1.TokenReviewSpec
			cli := utiltesting.NewClientBuilder().
				WithInterceptorFuncs(interceptor.Funcs{
					Create: func(_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
						review, ok := obj.(*authenticationv1.TokenReview)
						if !ok {
							t.Fatalf("Unexpected object created: %T", obj)
						}
						gotReview = review.Spec.DeepCopy()
						if tc.reviewErr != nil {
							return tc.reviewErr
						}
						review.Status = tc.reviewStatus
						return nil
					},
				}).
				Build()
			authorizer := NewTokenReviewAuthorizer(cli)
			if err := authorizer.Init(context.Background()); err != nil {
				t.Fatalf("Failed to initialize authorizer: %v", err)
			}

			gotAuthorized, err := authorizer.Authorize(context.Background(), tc.authHeader, namespace, trainJobName)

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
			if diff := cmp.Diff(tc.wantReview, gotReview); len(diff) != 0 {
				t.Errorf("Unexpected token review (-want,+got):\n%s", diff)
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
