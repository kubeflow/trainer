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
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s.io/utils/ptr"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
)

func TestProbeChecker(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	checker := probeChecker(srv.Listener.Addr().String())

	if err := checker(nil); err != nil {
		t.Fatalf("Unexpected error from probe checker while the server is serving: %v", err)
	}

	srv.Close()

	if err := checker(nil); err == nil {
		t.Error("Expected an error from probe checker after the server stopped, got nil")
	}
}

func TestRegisterProbesValidatesConfig(t *testing.T) {
	cases := map[string]*configapi.StatusServer{
		"nil config":                      nil,
		"empty config":                    {},
		"config without an explicit port": {QPS: ptr.To[float32](5)},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			// The manager is never dereferenced, the port validation runs first.
			if err := RegisterProbes(nil, cfg); err == nil {
				t.Error("Expected an error when the status server port is unset, got nil")
			}
		})
	}
}
