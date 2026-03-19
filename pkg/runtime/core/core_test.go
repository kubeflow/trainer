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

package core

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestRuntimes(t *testing.T) {
	cases := map[string]struct {
	}{
		"returns a copy of the runtimes map": {},
	}
	for name := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			clientBuilder := testingutil.NewClientBuilder()
			c := clientBuilder.Build()

			newRuntimes, err := New(ctx, c, testingutil.AsIndex(clientBuilder), nil)
			if err != nil {
				t.Fatalf("Failed to initialize runtimes: %v", err)
			}

			gotRuntimes := Runtimes()
			if diff := cmp.Diff(newRuntimes, gotRuntimes, cmpopts.IgnoreUnexported(TrainingRuntime{}, ClusterTrainingRuntime{})); len(diff) != 0 {
				t.Errorf("Unexpected difference between new and got runtimes (-want,+got):\n%s", diff)
			}

			// Verify that modifying the returned map does not affect the persisted runtimes.
			gotRuntimes["mutated-key"] = nil
			gotRuntimes[TrainingRuntimeGroupKind] = nil
			delete(gotRuntimes, ClusterTrainingRuntimeGroupKind)

			if _, exists := runtimes["mutated-key"]; exists {
				t.Error("Adding a key to Runtimes() return value should not affect the persisted runtimes")
			}
			if runtimes[TrainingRuntimeGroupKind] == nil {
				t.Error("Modifying an existing key in Runtimes() return value should not affect the persisted runtimes")
			}
			if _, exists := runtimes[ClusterTrainingRuntimeGroupKind]; !exists {
				t.Error("Deleting a key from Runtimes() return value should not affect the persisted runtimes")
			}
		})
	}
}
