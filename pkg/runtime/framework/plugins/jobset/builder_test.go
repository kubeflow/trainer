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

package jobset

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	batchv1ac "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/utils/ptr"
	jobsetv1alpha2ac "sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestBuilderTrainerEnvPropagation(t *testing.T) {
	testCases := map[string]struct {
		trainJobEnv   []corev1.EnvVar
		initialPodEnv []corev1ac.EnvVarApplyConfiguration
		expectedEnv   []corev1.EnvVar
	}{
		"variables propagated": {
			trainJobEnv: []corev1.EnvVar{{Name: "CUSTOM_VAR", Value: "custom_value"}},
			expectedEnv: []corev1.EnvVar{{Name: "CUSTOM_VAR", Value: "custom_value"}},
		},
		"no variables propagated (empty case)": {
			trainJobEnv: []corev1.EnvVar{},
			expectedEnv: []corev1.EnvVar{},
		},
		"merge with existing variables": {
			trainJobEnv: []corev1.EnvVar{{Name: "CUSTOM_VAR", Value: "custom_value"}},
			initialPodEnv: []corev1ac.EnvVarApplyConfiguration{
				*corev1ac.EnvVar().WithName("EXISTING_VAR").WithValue("existing_value"),
			},
			expectedEnv: []corev1.EnvVar{
				{Name: "EXISTING_VAR", Value: "existing_value"},
				{Name: "CUSTOM_VAR", Value: "custom_value"},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup TrainJob
			trainJob := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().
					Env(tc.trainJobEnv...).
					Obj()).
				Obj()

			// Setup runtime info for MPI (launcher is NOT a node)
			info := &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicySource: utiltesting.MakeMLPolicySourceWrapper().
						MPIPolicy(nil, trainer.MPIImplementationOpenMPI, nil, ptr.To(false)).
						Obj(),
				},
			}

			// Create JobSet spec with initial environment
			container := corev1ac.Container().WithName(constants.Node)
			for i := range tc.initialPodEnv {
				container.WithEnv(&tc.initialPodEnv[i])
			}

			jobSetSpec := jobsetv1alpha2ac.JobSetSpec().WithReplicatedJobs(
				jobsetv1alpha2ac.ReplicatedJob().
					WithName(constants.Node).
					WithTemplate(batchv1ac.JobTemplateSpec().
						WithSpec(batchv1ac.JobSpec().
							WithTemplate(corev1ac.PodTemplateSpec().
								WithSpec(corev1ac.PodSpec().WithContainers(container)),
							),
						),
					),
			)

			builder := NewBuilder(jobsetv1alpha2ac.JobSet("test-job", metav1.NamespaceDefault).WithSpec(jobSetSpec))
			builder.Trainer(info, trainJob)

			// Verify results
			var actualEnv []corev1.EnvVar
			for _, rJob := range builder.Spec.ReplicatedJobs {
				if *rJob.Name == constants.Node {
					for _, c := range rJob.Template.Spec.Template.Spec.Containers {
						if *c.Name == constants.Node {
							for _, env := range c.Env {
								actualEnv = append(actualEnv, corev1.EnvVar{Name: *env.Name, Value: *env.Value})
							}
						}
					}
				}
			}

			if len(actualEnv) != len(tc.expectedEnv) {
				t.Fatalf("Expected %d environment variables, got %d", len(tc.expectedEnv), len(actualEnv))
			}

			for i, expected := range tc.expectedEnv {
				if actualEnv[i].Name != expected.Name || actualEnv[i].Value != expected.Value {
					t.Errorf("At index %d: expected %s=%s, got %s=%s", i, expected.Name, expected.Value, actualEnv[i].Name, actualEnv[i].Value)
				}
			}
		})
	}
}
