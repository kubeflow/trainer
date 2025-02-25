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

package core

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubeflow/trainer/pkg/runtime"
)

// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=trainingruntimes,verbs=get;list;watch
// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=clustertrainingruntimes,verbs=get;list;watch

func New(ctx context.Context, client client.Client, indexer client.FieldIndexer) (map[string]runtime.Runtime, error) {
	registry := NewRuntimeRegistry()
	runtimes := make(map[string]runtime.Runtime, len(registry))
	for name, factory := range registry {
		r, err := factory(ctx, client, indexer)
		if err != nil {
			return nil, fmt.Errorf("initializing runtime %q: %w", name, err)
		}
		runtimes[name] = r
	}
	return runtimes, nil
}
