/*
Copyright 2025 The Kubeflow Authors.

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

package flux

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	trainerapi "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

func TestEncodeZ85(t *testing.T) {
	cases := []struct {
		name     string
		input    []byte
		wantLen  int
		expected string
	}{
		{
			name:    "32 bytes produces 40 characters",
			input:   make([]byte, 32),
			wantLen: 40,
		},
		{
			name:    "invalid length returns empty string",
			input:   []byte{1, 2, 3}, // Not a multiple of 4
			wantLen: 0,
		},
		{
			name:     "all zeros produces zeros in Z85",
			input:    []byte{0, 0, 0, 0},
			wantLen:  5,
			expected: "00000",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := encodeZ85(tc.input)
			if len(got) != tc.wantLen {
				t.Errorf("encodeZ85() length = %d; want %d", len(got), tc.wantLen)
			}
			if tc.expected != "" && got != tc.expected {
				t.Errorf("encodeZ85() = %q; want %q", got, tc.expected)
			}
		})
	}
}

func TestBuildCurveSecret(t *testing.T) {
	f := &Flux{}
	trainJob := &trainerapi.TrainJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
			UID:       types.UID("12345-67890"),
		},
	}

	// Check generation
	secretApply, err := f.buildCurveSecret(trainJob)
	if err != nil {
		t.Fatalf("buildCurveSecret failed: %v", err)
	}

	if *secretApply.Name != "test-job-flux-curve" {
		t.Errorf("Expected secret name test-job-flux-curve, got %s", *secretApply.Name)
	}

	// Check format of the certificate content
	certBytes, ok := secretApply.Data["curve.cert"]
	if !ok {
		t.Fatal("curve.cert key missing from secret data")
	}
	certContent := string(certBytes)

	requiredHeaders := []string{"metadata", "curve", "public-key =", "secret-key =", "name = \"test-job\""}
	for _, header := range requiredHeaders {
		if !strings.Contains(certContent, header) {
			t.Errorf("certContent missing required header/field: %q", header)
		}
	}

	// Check Determinism: Same UID must produce same keys
	secretApply2, _ := f.buildCurveSecret(trainJob)
	if string(secretApply.Data["curve.cert"]) != string(secretApply2.Data["curve.cert"]) {
		t.Error("buildCurveSecret is not deterministic; generated different certs for the same UID")
	}

	// Check Uniqueness: Different UID must produce different keys
	trainJob.UID = types.UID("different-uid")
	secretApply3, _ := f.buildCurveSecret(trainJob)
	if string(secretApply.Data["curve.cert"]) == string(secretApply3.Data["curve.cert"]) {
		t.Error("buildCurveSecret produced the same cert for different UIDs")
	}

	// Verify indentation (Flux/CZMQ requires 4 spaces)
	if !strings.Contains(certContent, "    public-key =") {
		t.Error("certContent does not use the required 4-space indentation for key fields")
	}
}
