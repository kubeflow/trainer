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

package v1alpha1

import (
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

func TestOptimizationJobDeepCopy(t *testing.T) {
	metricName := "val_loss"
	direction := ObjectiveDirectionMinimize
	numTrials := int32(10)
	parallelTrials := int32(2)

	orig := &OptimizationJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: SchemeGroupVersion.String(),
			Kind:       OptimizationJobKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-optjob",
			Namespace: "default",
		},
		Spec: OptimizationJobSpec{
			Objectives: []Objective{
				{
					Metric:    &metricName,
					Direction: &direction,
				},
			},
			SearchAlgorithm: &SearchAlgorithm{
				Random: &RandomAlgorithm{
					Seed: ptr.To[int64](42),
				},
			},
			Parameters: []Parameter{
				{
					Name: "learning_rate",
					SearchSpace: SearchSpace{
						LogUniform: &LogUniformSpace{
							Min:  Double("0.0001"),
							Max:  Double("0.1"),
							Type: ParameterTypeFloat,
						},
					},
				},
				{
					Name: "batch_size",
					SearchSpace: SearchSpace{
						Categorical: &CategoricalSpace{
							Choices: []string{"16", "32", "64"},
						},
					},
				},
			},
			NumTrials:      &numTrials,
			ParallelTrials: &parallelTrials,
			TrainJobTemplate: TrainJobTemplateSpec{
				Spec: TrainJobSpec{
					Trainer: &Trainer{
						Image: ptr.To("docker.io/my-org/model:latest"),
					},
				},
			},
		},
		Status: OptimizationJobStatus{
			Conditions: []metav1.Condition{
				{
					Type:   "Complete",
					Status: metav1.ConditionTrue,
					Reason: "MaxTrialsReached",
				},
			},
			Result: &Result{
				TrainJobName: "test-optjob-trial-1",
				Parameters: []ParameterAssignment{
					{
						Name:  "learning_rate",
						Value: "0.001",
					},
				},
			},
		},
	}

	copied := orig.DeepCopy()

	if copied == nil {
		t.Fatalf("expected non-nil deepcopy")
	}
	if copied.Name != orig.Name {
		t.Errorf("expected Name %s, got %s", orig.Name, copied.Name)
	}

	// Verify DeepCopyObject runtime.Object interface implementation
	obj := orig.DeepCopyObject()
	if obj == nil {
		t.Fatalf("expected non-nil runtime.Object from DeepCopyObject")
	}

	list := &OptimizationJobList{
		Items: []OptimizationJob{*orig},
	}
	listCopied := list.DeepCopy()
	if len(listCopied.Items) != 1 {
		t.Errorf("expected 1 item in list deepcopy, got %d", len(listCopied.Items))
	}
	listObj := list.DeepCopyObject()
	if listObj == nil {
		t.Fatalf("expected non-nil runtime.Object from OptimizationJobList.DeepCopyObject")
	}
}

func TestOptimizationJobJSONSerialization(t *testing.T) {
	numTrials := int32(5)
	job := &OptimizationJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: SchemeGroupVersion.String(),
			Kind:       OptimizationJobKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "json-test-job",
		},
		Spec: OptimizationJobSpec{
			Objectives: []Objective{
				{
					Metric: ptr.To("accuracy"),
				},
			},
			NumTrials: &numTrials,
		},
	}

	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("failed to marshal OptimizationJob: %v", err)
	}

	var unmarshaled OptimizationJob
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal OptimizationJob: %v", err)
	}

	if unmarshaled.Name != job.Name {
		t.Errorf("expected name %s, got %s", job.Name, unmarshaled.Name)
	}
	if *unmarshaled.Spec.NumTrials != numTrials {
		t.Errorf("expected numTrials %d, got %d", numTrials, *unmarshaled.Spec.NumTrials)
	}
}

func TestOptimizationJobSchemeRegistration(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := addKnownTypes(scheme); err != nil {
		t.Fatalf("failed to add known types: %v", err)
	}

	knownTypes := scheme.KnownTypes(GroupVersion)
	if _, ok := knownTypes["OptimizationJob"]; !ok {
		t.Errorf("OptimizationJob not registered in scheme")
	}
	if _, ok := knownTypes["OptimizationJobList"]; !ok {
		t.Errorf("OptimizationJobList not registered in scheme")
	}
}
