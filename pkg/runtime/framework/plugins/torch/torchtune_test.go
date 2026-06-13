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

package torch

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/kubeflow/trainer/v2/pkg/constants"
)

func TestGetModelFromRuntimeRef(t *testing.T) {
	cases := map[string]struct {
		runtimeRefName string
		want           string
	}{
		"llama3.2 1B is normalized": {
			runtimeRefName: "torchtune-llama3.2-1b",
			want:           constants.TORCHTUNE_MODEL_LLAMA3_2_1B,
		},
		"qwen2.5 1.5B is normalized": {
			runtimeRefName: "torchtune-qwen2.5-1.5b",
			want:           constants.TORCHTUNE_MODEL_QWEN2_5_1_5B,
		},
		"fewer than three parts returns empty": {
			runtimeRefName: "torchtune-llama3.2",
			want:           "",
		},
		"more than three parts returns empty": {
			runtimeRefName: "torchtune-llama3.2-1b-extra",
			want:           "",
		},
		"empty name returns empty": {
			runtimeRefName: "",
			want:           "",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := getModelFromRuntimeRef(tc.runtimeRefName)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Unexpected model (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestIsLoraConfigEnabled(t *testing.T) {
	cases := map[string]struct {
		args []string
		want bool
	}{
		"lora attn modules present": {
			args: []string{"batch_size=32", constants.TorchTuneLoraAttnModules + "=['q_proj','v_proj']"},
			want: true,
		},
		"no lora args": {
			args: []string{"batch_size=32", "epochs=10"},
			want: false,
		},
		"empty args": {
			args: nil,
			want: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := isLoraConfigEnabled(tc.args); got != tc.want {
				t.Errorf("isLoraConfigEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsUseQLoraFinetune(t *testing.T) {
	cases := map[string]struct {
		args []string
		want bool
	}{
		"quantize base enabled": {
			args: []string{constants.TorchTuneQuantizeBase + "=True"},
			want: true,
		},
		"dora short-circuits even when quantize base is set": {
			args: []string{constants.TorchTuneQuantizeBase + "=True", constants.TorchTuneUseDora + "=True"},
			want: false,
		},
		"neither quantize base nor dora": {
			args: []string{"batch_size=32"},
			want: false,
		},
		"empty args": {
			args: nil,
			want: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := isUseQLoraFinetune(tc.args); got != tc.want {
				t.Errorf("isUseQLoraFinetune() = %v, want %v", got, tc.want)
			}
		})
	}
}
