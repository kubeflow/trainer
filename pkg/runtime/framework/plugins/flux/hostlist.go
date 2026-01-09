package flux

/*
Copyright 2025 The Kubeflow Authors.

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

import (
	"fmt"

	"github.com/kubeflow/trainer/v2/pkg/constants"
)

// generateHostlist for a specific size given a host prefix and a size
// This is a replicated job so format is different
// lammps-flux-interactive-node-0-0
func generateHostlist(prefix string, size int32) string {

	// Assume a setup without bursting / changing size.
	// We can extend this in the future to allow adding hosts
	// TODO where does the first index 0 come from?
	// TODO can we be guaranteed the pod (and network) will always be node?
	return fmt.Sprintf("%s-%s-0-[%s]", prefix, constants.Node, generateRange(size, 0))
}

// generateRange is a shared function to generate a range string
func generateRange(size int32, start int32) string {
	var rangeString string
	if size == 1 {
		rangeString = fmt.Sprintf("%d", start)
	} else {
		rangeString = fmt.Sprintf("%d-%d", start, (start+size)-1)
	}
	return rangeString
}
