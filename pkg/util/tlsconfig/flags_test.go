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
)

func TestFromFlags(t *testing.T) {
	tests := []struct {
		name       string
		configMin  string
		version    string
		ciphers    string
		curves     string
		wantErr    bool
		wantMin    uint16
		wantCipher int
		wantCurves int
	}{
		{name: "empty flags", wantMin: 0},
		{name: "TLS 1.2", version: "VersionTLS12", wantMin: tls.VersionTLS12},
		{name: "short TLS 1.3", version: "TLS1.3", wantMin: tls.VersionTLS13},
		{name: "config TLS 1.3", configMin: "1.3", wantMin: 0},
		{
			name:       "all options",
			version:    "VersionTLS12",
			ciphers:    "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			curves:     "23,29",
			wantMin:    tls.VersionTLS12,
			wantCipher: 1,
			wantCurves: 2,
		},
		{name: "TLS 1.1 is rejected", version: "VersionTLS11", wantErr: true},
		{name: "unknown version", version: "VersionTLS09", wantErr: true},
		{name: "unknown cipher", ciphers: "BOGUS", wantErr: true},
		{name: "insecure cipher is rejected", ciphers: "TLS_RSA_WITH_RC4_128_SHA", wantErr: true},
		{name: "unknown curve", curves: "not-a-number", wantErr: true},
		{name: "unsupported curve", curves: "9999", wantErr: true},
		{name: "TLS 1.3 cipher restriction", version: "VersionTLS13", ciphers: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", wantErr: true},
		{name: "config TLS 1.3 cipher restriction", configMin: "1.3", ciphers: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			option, err := FromFlags(tt.configMin, tt.version, tt.ciphers, tt.curves)
			if tt.wantErr {
				if err == nil {
					t.Fatal("FromFlags() expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("FromFlags() unexpected error: %v", err)
			}
			config := &tls.Config{}
			if option != nil {
				option(config)
			}
			if config.MinVersion != tt.wantMin {
				t.Fatalf("MinVersion = %d, want %d", config.MinVersion, tt.wantMin)
			}
			if len(config.CipherSuites) != tt.wantCipher {
				t.Fatalf("CipherSuites = %d, want %d", len(config.CipherSuites), tt.wantCipher)
			}
			if len(config.CurvePreferences) != tt.wantCurves {
				t.Fatalf("CurvePreferences = %d, want %d", len(config.CurvePreferences), tt.wantCurves)
			}
		})
	}
}
