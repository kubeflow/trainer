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

package progress

import "fmt"

const (
	// LabelTrainJobName is the label key used to identify which TrainJob a pod belongs to
	LabelTrainJobName = "trainer.kubeflow.org/trainjob-name"
)

// StatusUrl is the path of the endpoint for receiving status updates
func StatusUrl(namespace, name string) string {
	return fmt.Sprintf("/apis/trainer.kubeflow.org/v1alpha1/namespaces/%s/trainjobs/%s/status", namespace, name)
}
