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

package tlsconfig

import (
	"crypto/tls"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
)

func TestApply(t *testing.T) {
	testcases := map[string]struct {
		opts             *configapi.TLSOptions
		wantNextProtos   []string
		wantMinVersion   uint16
		wantCipherSuites []uint16
	}{
		"nil options disable HTTP/2": {
			opts:           nil,
			wantNextProtos: []string{"http/1.1"},
		},
		"empty options disable HTTP/2": {
			opts:           &configapi.TLSOptions{},
			wantNextProtos: []string{"http/1.1"},
		},
		"nextProtos wins unconditionally when set": {
			opts: &configapi.TLSOptions{
				NextProtos: []string{"h2", "http/1.1"},
			},
			wantNextProtos: []string{"h2", "http/1.1"},
		},
		"nextProtos can pin HTTP/1.1 explicitly": {
			opts: &configapi.TLSOptions{
				NextProtos: []string{"http/1.1"},
			},
			wantNextProtos: []string{"http/1.1"},
		},
		"minVersion is applied": {
			opts: &configapi.TLSOptions{
				MinVersion: configapi.TLSVersion13,
			},
			wantNextProtos: []string{"http/1.1"},
			wantMinVersion: tls.VersionTLS13,
		},
		"every supported minVersion is recognized": {
			opts: &configapi.TLSOptions{
				MinVersion: configapi.TLSVersion10,
			},
			wantNextProtos: []string{"http/1.1"},
			wantMinVersion: tls.VersionTLS10,
		},
		"unrecognized minVersion falls back to the Go default": {
			opts: &configapi.TLSOptions{
				MinVersion: "1.5",
			},
			wantNextProtos: []string{"http/1.1"},
			wantMinVersion: 0,
		},
		"cipher suites are resolved to their IDs": {
			opts: &configapi.TLSOptions{
				CipherSuites: []string{
					"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
					"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
				},
			},
			wantNextProtos: []string{"http/1.1"},
			wantCipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			},
		},
		"insecure cipher suites are accepted with a warning": {
			opts: &configapi.TLSOptions{
				CipherSuites: []string{"TLS_RSA_WITH_RC4_128_SHA"},
			},
			wantNextProtos:   []string{"http/1.1"},
			wantCipherSuites: []uint16{tls.TLS_RSA_WITH_RC4_128_SHA},
		},
		"unrecognized cipher suites are dropped": {
			opts: &configapi.TLSOptions{
				CipherSuites: []string{
					"TLS_NOT_A_REAL_SUITE",
					"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				},
			},
			wantNextProtos:   []string{"http/1.1"},
			wantCipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
		},
		"all-unrecognized cipher suites leave the Go defaults in place": {
			opts: &configapi.TLSOptions{
				CipherSuites: []string{"TLS_NOT_A_REAL_SUITE"},
			},
			wantNextProtos:   []string{"http/1.1"},
			wantCipherSuites: nil,
		},
		"all options are applied together": {
			opts: &configapi.TLSOptions{
				MinVersion:   configapi.TLSVersion12,
				CipherSuites: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
				NextProtos:   []string{"h2", "http/1.1"},
			},
			wantNextProtos:   []string{"h2", "http/1.1"},
			wantMinVersion:   tls.VersionTLS12,
			wantCipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			got := &tls.Config{}
			Apply(got, tc.opts)

			if diff := cmp.Diff(tc.wantNextProtos, got.NextProtos); len(diff) != 0 {
				t.Errorf("Unexpected NextProtos (-want,+got):\n%s", diff)
			}
			if got.MinVersion != tc.wantMinVersion {
				t.Errorf("MinVersion = %v, want %v", got.MinVersion, tc.wantMinVersion)
			}
			if diff := cmp.Diff(tc.wantCipherSuites, got.CipherSuites, cmpopts.EquateEmpty()); len(diff) != 0 {
				t.Errorf("Unexpected CipherSuites (-want,+got):\n%s", diff)
			}
		})
	}
}

// Apply must not clobber fields it does not own, such as the certificate
// getter installed by the cert watcher.
func TestApplyPreservesExistingConfig(t *testing.T) {
	got := &tls.Config{
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return nil, nil
		},
	}

	Apply(got, &configapi.TLSOptions{MinVersion: configapi.TLSVersion13})

	if got.GetCertificate == nil {
		t.Error("Expected GetCertificate to be preserved")
	}
	if got.MinVersion != tls.VersionTLS13 {
		t.Errorf("MinVersion = %v, want %v", got.MinVersion, tls.VersionTLS13)
	}
}

func TestParseTLSVersion(t *testing.T) {
	testcases := map[string]struct {
		version string
		want    uint16
	}{
		"1.0":         {version: configapi.TLSVersion10, want: tls.VersionTLS10},
		"1.1":         {version: configapi.TLSVersion11, want: tls.VersionTLS11},
		"1.2":         {version: configapi.TLSVersion12, want: tls.VersionTLS12},
		"1.3":         {version: configapi.TLSVersion13, want: tls.VersionTLS13},
		"empty":       {version: "", want: 0},
		"unsupported": {version: "1.5", want: 0},
		"non-numeric": {version: "abc", want: 0},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			if got := parseTLSVersion(tc.version); got != tc.want {
				t.Errorf("parseTLSVersion(%q) = %v, want %v", tc.version, got, tc.want)
			}
		})
	}
}

func TestParseCipherSuiteIDs(t *testing.T) {
	testcases := map[string]struct {
		names []string
		want  []uint16
	}{
		"nil names": {
			names: nil,
			want:  nil,
		},
		"secure suites": {
			names: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
			want:  []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
		},
		"insecure suites are still resolved": {
			names: []string{"TLS_RSA_WITH_RC4_128_SHA"},
			want:  []uint16{tls.TLS_RSA_WITH_RC4_128_SHA},
		},
		"unrecognized names are skipped": {
			names: []string{"NOPE", "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "ALSO_NOPE"},
			want:  []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
		},
		"order is preserved": {
			names: []string{
				"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
				"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			},
			want: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			got := parseCipherSuiteIDs(tc.names)
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); len(diff) != 0 {
				t.Errorf("Unexpected cipher suite IDs (-want,+got):\n%s", diff)
			}
		})
	}
}
