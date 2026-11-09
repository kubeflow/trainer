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

package metrics

import (
	"errors"
	"net/http"
	"testing"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type fakeControlManager struct {
	ctrl.Manager

	initialisedConfig *rest.Config
	httpClient        *http.Client
	addError          error
	serverAdded       bool
}

func (mgr *fakeControlManager) GetConfig() *rest.Config {
	return mgr.initialisedConfig
}

func (mgr *fakeControlManager) GetHTTPClient() *http.Client {
	return mgr.httpClient
}

func (mgr *fakeControlManager) Add(_ manager.Runnable) error {
	mgr.serverAdded = true
	return mgr.addError
}

func TestSetupServer(t *testing.T) {
	tests := map[string]struct {
		cfg             *configapi.ControllerMetrics
		tlsOpts         *configapi.TLSOptions
		wantServerAdded bool
		addError        error
		wantError       bool
	}{

		"insecure metrics server is added": {
			cfg: &configapi.ControllerMetrics{
				BindAddress:   ":8443",
				SecureServing: ptr.To(false),
			},
			wantServerAdded: true,
		},

		"secure metrics server is added": {
			cfg: &configapi.ControllerMetrics{
				BindAddress:   ":8443",
				SecureServing: ptr.To(true),
			},
			tlsOpts:         &configapi.TLSOptions{},
			wantServerAdded: true,
		},

		"metrics server is added when SecureServing is not specified": {
			cfg: &configapi.ControllerMetrics{
				BindAddress: ":8443",
			},
			wantServerAdded: true,
		},

		"metrics server is disabled when BindAddress is zero": {
			cfg: &configapi.ControllerMetrics{
				BindAddress: "0",
			},
			wantServerAdded: false,
		},

		"metrics server is added when authentication is enabled": {
			cfg: &configapi.ControllerMetrics{
				BindAddress:   ":8443",
				SecureServing: ptr.To(true),
				Auth: &configapi.MetricsAuthConfig{
					Enabled: ptr.To(true),
				},
			},
			tlsOpts:         &configapi.TLSOptions{},
			wantServerAdded: true,
		},

		"returns error when metrics server addition fails": {
			cfg: &configapi.ControllerMetrics{
				BindAddress:   ":8443",
				SecureServing: ptr.To(false),
			},
			addError:        errors.New("failed to add metrics server"),
			wantError:       true,
			wantServerAdded: true,
		},
	}

	for name, testCase := range tests {

		t.Run(name, func(t *testing.T) {
			fakeManager := &fakeControlManager{
				initialisedConfig: &rest.Config{
					Host: "https://127.0.0.1",
				},
				httpClient: &http.Client{},
				addError:   testCase.addError,
			}
			err := SetupServer(fakeManager, testCase.cfg, testCase.tlsOpts)

			if testCase.wantError {
				if err == nil {
					t.Fatalf("SetupServer() returned nil error, want error")
				}
			} else if err != nil {
				t.Fatalf("SetupServer() returned unexpected error: %v", err)
			}

			if fakeManager.serverAdded != testCase.wantServerAdded {
				t.Errorf(
					"server addition: %v, want %v",
					fakeManager.serverAdded,
					testCase.wantServerAdded,
				)
			}
		})
	}
}
