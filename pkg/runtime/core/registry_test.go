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

import "testing"

func TestNewRuntimeRegistry(t *testing.T) {
	tests := map[string]struct {
		validate func(t *testing.T, registry Registry)
	}{
		"registry is initialized": {
			validate: func(t *testing.T, registry Registry) {
				if registry == nil {
					t.Fatalf("expected registry to be non-nil")
				}
			},
		},
		"TrainingRuntime is registered correctly": {
			validate: func(t *testing.T, registry Registry) {
				tr, ok := registry[TrainingRuntimeGroupKind]
				if !ok {
					t.Fatalf("expected TrainingRuntimeGroupKind to be registered")
				}
				if tr.factory == nil {
					t.Fatalf("expected TrainingRuntime factory to be non-nil")
				}
				if len(tr.dependencies) != 0 {
					t.Fatalf("expected TrainingRuntime to have no dependencies, got %v", tr.dependencies)
				}
			},
		},
		"ClusterTrainingRuntime is registered correctly": {
			validate: func(t *testing.T, registry Registry) {
				ctr, ok := registry[ClusterTrainingRuntimeGroupKind]
				if !ok {
					t.Fatalf("expected ClusterTrainingRuntimeGroupKind to be registered")
				}
				if ctr.factory == nil {
					t.Fatalf("expected ClusterTrainingRuntime factory to be non-nil")
				}
			},
		},
		"ClusterTrainingRuntime depends on TrainingRuntime": {
			validate: func(t *testing.T, registry Registry) {
				ctr, ok := registry[ClusterTrainingRuntimeGroupKind]

				if !ok {
					t.Fatalf("expected ClusterTrainingRuntimeGroupKind to be registered")
				}
				if ctr.dependencies[0] != TrainingRuntimeGroupKind {
					t.Fatalf(
						"expected dependency %q, got %v",
						TrainingRuntimeGroupKind,
						ctr.dependencies,
					)
				}
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			registry := NewRuntimeRegistry()
			tt.validate(t, registry)
		})
	}
}
