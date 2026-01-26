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

package flux

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestGetOriginalCommand(t *testing.T) {
	cases := []struct {
		name     string
		trainJob *trainer.TrainJob
		want     string
	}{
		{
			name: "full command and args",
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().
					Container("image", []string{"python"}, []string{"train.py", "--epochs", "10"}, nil).
					Obj()).
				Obj(),
			want: "python train.py --epochs 10",
		},
		{
			name: "command and args with extra spaces",
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().
					Container("image", []string{"  python  "}, []string{" script.py "}, nil).
					Obj()).
				Obj(),
			want: "python    script.py",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := getOriginalCommand(tc.trainJob)
			if got != tc.want {
				t.Errorf("getOriginalCommand() = %q; want %q", got, tc.want)
			}
		})
	}
}
