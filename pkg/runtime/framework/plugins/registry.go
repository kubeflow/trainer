/*
Copyright 2024 The Kubeflow Authors.

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
	"fmt"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/client"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/features"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/coscheduling"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/flux"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/jax"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/jobset"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/mpi"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/plainml"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/torch"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/trainjobstatus"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/volcano"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins/xgboost"
)

type Registry map[string]func(ctx context.Context, client client.Client, indexer client.FieldIndexer, cfg *configapi.Configuration) (framework.Plugin, error)

var (
	registeredPlugins   = make(Registry)
	registeredPluginsMu sync.RWMutex
)

// Register adds a plugin factory to every subsequently created Registry.
//
// Register must be called before the controller manager initializes its runtimes.
// It is intended for custom controller-manager builds to register plugins during
// package initialization.
func Register(name string, factory func(ctx context.Context, client client.Client, indexer client.FieldIndexer, cfg *configapi.Configuration) (framework.Plugin, error)) error {
	if name == "" {
		return fmt.Errorf("plugin name must not be empty")
	}
	if factory == nil {
		return fmt.Errorf("plugin factory for %q must not be nil", name)
	}
	if isBuiltInPlugin(name) {
		return fmt.Errorf("plugin %q is built in and cannot be replaced", name)
	}

	registeredPluginsMu.Lock()
	defer registeredPluginsMu.Unlock()
	if _, found := registeredPlugins[name]; found {
		return fmt.Errorf("plugin %q is already registered", name)
	}
	registeredPlugins[name] = factory
	return nil
}

func NewRegistry() Registry {
	registry := Registry{
		coscheduling.Name: coscheduling.New,
		flux.Name:         flux.New,
		volcano.Name:      volcano.New,
		mpi.Name:          mpi.New,
		plainml.Name:      plainml.New,
		torch.Name:        torch.New,
		jobset.Name:       jobset.New,
		jax.Name:          jax.New,
		xgboost.Name:      xgboost.New,
	}

	if features.Enabled(features.TrainJobStatus) {
		registry[trainjobstatus.Name] = trainjobstatus.New
	}

	registeredPluginsMu.RLock()
	defer registeredPluginsMu.RUnlock()
	for name, factory := range registeredPlugins {
		if _, found := registry[name]; found {
			continue
		}
		registry[name] = factory
	}

	return registry
}

func isBuiltInPlugin(name string) bool {
	switch name {
	case coscheduling.Name, flux.Name, volcano.Name, mpi.Name, plainml.Name,
		torch.Name, jobset.Name, jax.Name, xgboost.Name, trainjobstatus.Name:
		return true
	default:
		return false
	}
}
