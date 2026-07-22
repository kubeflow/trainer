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
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestTokenAudience(t *testing.T) {
	testcases := map[string]struct {
		namespace    string
		name         string
		wantAudience string
	}{
		"audience is scoped to the TrainJob status endpoint": {
			namespace:    "default",
			name:         "test-job",
			wantAudience: "trainer.kubeflow.org/v1alpha1/namespaces/default/trainjobs/test-job/status",
		},
		"audience is scoped to the TrainJob namespace": {
			namespace:    "kubeflow-system",
			name:         "test-job",
			wantAudience: "trainer.kubeflow.org/v1alpha1/namespaces/kubeflow-system/trainjobs/test-job/status",
		},
		"empty namespace and name are not rejected": {
			namespace:    "",
			name:         "",
			wantAudience: "trainer.kubeflow.org/v1alpha1/namespaces//trainjobs//status",
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			got := TokenAudience(tc.namespace, tc.name)

			if diff := cmp.Diff(tc.wantAudience, got); len(diff) != 0 {
				t.Errorf("Unexpected audience (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestStatusUrl(t *testing.T) {
	testcases := map[string]struct {
		namespace string
		name      string
		wantUrl   string
	}{
		"URL points at the TrainJob status endpoint": {
			namespace: "default",
			name:      "test-job",
			wantUrl:   "/apis/trainer.kubeflow.org/v1alpha1/namespaces/default/trainjobs/test-job/status",
		},
		"URL is scoped to the TrainJob namespace": {
			namespace: "kubeflow-system",
			name:      "test-job",
			wantUrl:   "/apis/trainer.kubeflow.org/v1alpha1/namespaces/kubeflow-system/trainjobs/test-job/status",
		},
		"empty namespace and name are not rejected": {
			namespace: "",
			name:      "",
			wantUrl:   "/apis/trainer.kubeflow.org/v1alpha1/namespaces//trainjobs//status",
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			got := StatusUrl(tc.namespace, tc.name)

			if diff := cmp.Diff(tc.wantUrl, got); len(diff) != 0 {
				t.Errorf("Unexpected URL (-want,+got):\n%s", diff)
			}
		})
	}
}
