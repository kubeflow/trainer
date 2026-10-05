/*
Copyright 2026 The Kubeflow Authors.

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

package plugins

import (
	"context"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
)

const testPluginName = "test-plugin"

type testPlugin struct{}

func (testPlugin) Name() string {
	return testPluginName
}

func TestRegister(t *testing.T) {
	factory := func(context.Context, client.Client, client.FieldIndexer, *configapi.Configuration) (framework.Plugin, error) {
		return testPlugin{}, nil
	}

	if err := Register(testPluginName, factory); err != nil {
		t.Fatalf("register plugin: %v", err)
	}
	t.Cleanup(func() {
		registeredPluginsMu.Lock()
		defer registeredPluginsMu.Unlock()
		delete(registeredPlugins, testPluginName)
	})

	if _, found := NewRegistry()[testPluginName]; !found {
		t.Fatalf("expected registered plugin %q in a new registry", testPluginName)
	}
}

func TestRegisterRejectsInvalidFactories(t *testing.T) {
	validFactory := func(context.Context, client.Client, client.FieldIndexer, *configapi.Configuration) (framework.Plugin, error) {
		return testPlugin{}, nil
	}

	tests := map[string]struct {
		name    string
		factory func(context.Context, client.Client, client.FieldIndexer, *configapi.Configuration) (framework.Plugin, error)
	}{
		"empty name": {
			factory: validFactory,
		},
		"nil factory": {
			name: "nil-factory",
		},
		"built-in plugin": {
			name:    "JobSet",
			factory: validFactory,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if err := Register(tc.name, tc.factory); err == nil {
				t.Fatal("expected registration to fail")
			}
		})
	}
}
