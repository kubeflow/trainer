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

package torch

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/util/intstr"
	batchv1ac "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	jobsetv1alpha2ac "sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
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

func TestGetRecipeAndConfig(t *testing.T) {
	const model = constants.TORCHTUNE_MODEL_LLAMA3_2_1B
	runtimeRef := "torchtune-llama3.2-1b"
	loraArgs := []string{constants.TorchTuneLoraAttnModules + "=['q_proj']"}
	qloraArgs := []string{constants.TorchTuneQuantizeBase + "=True"}

	makeTrainJob := func(args []string) *trainer.TrainJob {
		return utiltesting.MakeTrainJobWrapper("default", "torchtune-job").
			Trainer(utiltesting.MakeTrainJobTrainerWrapper().
				Container("", []string{"tune", "run"}, args, nil).
				Obj(),
			).
			RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), runtimeRef).
			Obj()
	}

	cases := map[string]struct {
		numNodes       int32
		numProcPerNode intstr.IntOrString
		gpuQ           int
		args           []string
		wantRecipe     string
		wantConfig     string
	}{
		"single device full": {
			numNodes: 1, numProcPerNode: intstr.FromString("auto"), gpuQ: 1,
			wantRecipe: constants.TorchTuneFullFinetuneSingleDevice,
			wantConfig: model + constants.TorchTuneFullFinetuneSingleDeviceConfigSuffix,
		},
		"single device lora": {
			numNodes: 1, numProcPerNode: intstr.FromString("auto"), gpuQ: 1, args: loraArgs,
			wantRecipe: constants.TorchTuneLoRAFinetuneSingleDevice,
			wantConfig: model + constants.TorchTuneLoRAFinetuneSingleDeviceConfigSuffix,
		},
		"single device qlora": {
			numNodes: 1, numProcPerNode: intstr.FromString("auto"), gpuQ: 1, args: qloraArgs,
			wantRecipe: constants.TorchTuneLoRAFinetuneSingleDevice,
			wantConfig: model + constants.TorchTuneQLoRAFinetuneSingleDeviceConfigSuffix,
		},
		"single node multi-gpu full": {
			numNodes: 1, numProcPerNode: intstr.FromString("auto"), gpuQ: 4,
			wantRecipe: constants.TorchTuneFullFinetuneDistributed,
			wantConfig: model + constants.TorchTuneFullFinetuneMultiDevicesConfigSuffix,
		},
		"single node multi-gpu lora": {
			numNodes: 1, numProcPerNode: intstr.FromString("auto"), gpuQ: 4, args: loraArgs,
			wantRecipe: constants.TorchTuneLoRAFinetuneDistributed,
			wantConfig: model + constants.TorchTuneLoRAFinetuneDistributedConfigSuffix,
		},
		"single node multi-gpu qlora": {
			numNodes: 1, numProcPerNode: intstr.FromString("auto"), gpuQ: 4, args: qloraArgs,
			wantRecipe: constants.TorchTuneLoRAFinetuneDistributed,
			wantConfig: model + constants.TorchTuneQLoRAFinetuneDistributedConfigSuffix,
		},
		"multi-node full": {
			numNodes: 2, numProcPerNode: intstr.FromString("auto"), gpuQ: 8,
			wantRecipe: constants.TorchTuneFullFinetuneDistributed,
			wantConfig: model + constants.TorchTuneFullFinetuneMultiNodesConfigSuffix,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			recipe, config := getRecipeAndConfig(tc.numNodes, tc.numProcPerNode, tc.gpuQ, makeTrainJob(tc.args))
			if recipe != tc.wantRecipe {
				t.Errorf("recipe = %q, want %q", recipe, tc.wantRecipe)
			}
			if config != tc.wantConfig {
				t.Errorf("config = %q, want %q", config, tc.wantConfig)
			}
		})
	}
}

func TestExtractOverridesFromRuntime(t *testing.T) {
	// makeInfo builds a runtime.Info whose template carries a single replicated job
	// with the given ancestor label and a container with the given name and commands.
	makeInfo := func(ancestor, containerName string, commands []string) *runtime.Info {
		return runtime.NewInfo(runtime.WithTemplateSpecObjApply(
			jobsetv1alpha2ac.JobSetSpec().WithReplicatedJobs(
				jobsetv1alpha2ac.ReplicatedJob().WithTemplate(
					batchv1ac.JobTemplateSpec().
						WithLabels(map[string]string{constants.LabelTrainJobAncestor: ancestor}).
						WithSpec(batchv1ac.JobSpec().WithTemplate(
							corev1ac.PodTemplateSpec().WithSpec(
								corev1ac.PodSpec().WithContainers(
									corev1ac.Container().WithName(containerName).WithCommand(commands...),
								),
							),
						)),
				),
			),
		))
	}

	immutableCommands := []string{
		constants.TorchTuneModelOutputDir + "=/out",
		constants.TorchTuneTokenizerPath + "=/tok",
		"batch_size=32",
	}
	wantOverrides := []string{
		constants.TorchTuneModelOutputDir + "=/out",
		constants.TorchTuneTokenizerPath + "=/tok",
	}

	cases := map[string]struct {
		info *runtime.Info
		want []string
	}{
		"no jobset template": {
			info: runtime.NewInfo(),
			want: []string{},
		},
		"trainer node container returns only immutable configs": {
			info: makeInfo(constants.AncestorTrainer, constants.Node, immutableCommands),
			want: wantOverrides,
		},
		"non-trainer ancestor is ignored": {
			info: makeInfo("other", constants.Node, immutableCommands),
			want: []string{},
		},
		"non-node container is ignored": {
			info: makeInfo(constants.AncestorTrainer, "sidecar", immutableCommands),
			want: []string{},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := extractOverridesFromRuntime(tc.info)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Unexpected overrides (-want,+got):\n%s", diff)
			}
		})
	}
}
