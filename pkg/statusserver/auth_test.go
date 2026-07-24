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
	"testing"

	"github.com/google/go-cmp/cmp"
)

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
