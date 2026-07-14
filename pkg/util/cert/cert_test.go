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

package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
)

// fakeManager records the runnables added to it. SetupTLSConfig only ever calls
// Add, so the embedded nil Manager is never dereferenced.
type fakeManager struct {
	ctrl.Manager
	added  []manager.Runnable
	addErr error
}

func (f *fakeManager) Add(r manager.Runnable) error {
	if f.addErr != nil {
		return f.addErr
	}
	f.added = append(f.added, r)
	return nil
}

// writeSelfSignedCert writes a valid tls.crt/tls.key pair into dir, so that the
// cert watcher has something real to load.
func writeSelfSignedCert(t *testing.T, dir string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "kubeflow-trainer-webhook"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		DNSNames:     []string{"kubeflow-trainer-webhook.kubeflow-system.svc"},
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(filepath.Join(dir, "tls.crt"), certPEM, os.FileMode(0600)); err != nil {
		t.Fatal(err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(filepath.Join(dir, "tls.key"), keyPEM, os.FileMode(0600)); err != nil {
		t.Fatal(err)
	}
}

func TestSetupTLSConfig(t *testing.T) {
	testcases := map[string]struct {
		tlsOpts          *configapi.TLSOptions
		wantNextProtos   []string
		wantMinVersion   uint16
		wantCipherSuites []uint16
	}{
		"nil options disable HTTP/2": {
			tlsOpts:        nil,
			wantNextProtos: []string{"http/1.1"},
		},
		"nextProtos enables HTTP/2": {
			tlsOpts: &configapi.TLSOptions{
				NextProtos: []string{"h2", "http/1.1"},
			},
			wantNextProtos: []string{"h2", "http/1.1"},
		},
		"minVersion and cipherSuites are applied": {
			tlsOpts: &configapi.TLSOptions{
				MinVersion:   configapi.TLSVersion13,
				CipherSuites: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
			},
			wantNextProtos:   []string{"http/1.1"},
			wantMinVersion:   tls.VersionTLS13,
			wantCipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeSelfSignedCert(t, dir)

			originalCertDir := certDir
			certDir = dir
			t.Cleanup(func() { certDir = originalCertDir })

			mgr := &fakeManager{}
			got, err := SetupTLSConfig(mgr, tc.tlsOpts)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// The cert watcher must be registered with the manager, otherwise
			// rotated certificates would never be picked up.
			if len(mgr.added) != 1 {
				t.Errorf("Expected the cert watcher to be added to the manager, got %d runnables", len(mgr.added))
			}
			if got.GetCertificate == nil {
				t.Error("Expected GetCertificate to be wired to the cert watcher")
			}

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

func TestSetupTLSConfigMissingCerts(t *testing.T) {
	originalCertDir := certDir
	certDir = filepath.Join(t.TempDir(), "does-not-exist")
	t.Cleanup(func() { certDir = originalCertDir })

	if _, err := SetupTLSConfig(&fakeManager{}, nil); err == nil {
		t.Error("Expected an error when the serving certificates are missing")
	}
}

func TestGetOperatorNamespace(t *testing.T) {
	// Outside of a pod the service account namespace file does not exist, so we
	// must fall back to the default namespace rather than returning an empty one.
	if got := GetOperatorNamespace(); got != defaultNamespace {
		t.Errorf("GetOperatorNamespace() = %q, want %q", got, defaultNamespace)
	}
}
