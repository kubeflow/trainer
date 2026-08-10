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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestTrainingRuntimeValidateObjects(t *testing.T) {
	resRequests := corev1.ResourceList{
		corev1.ResourceCPU: resource.MustParse("1"),
	}

	validRuntime := testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").
		RuntimeSpec(
			testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").Spec).
				Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
				Obj(),
		).Obj()

	cases := map[string]struct {
		trainJob        *trainer.TrainJob
		existingRuntime *trainer.TrainingRuntime
		wantFieldErrors int // Number of field errors expected
		wantErrorPath   string
		wantErrorSubstr string
	}{
		"succeeds when runtime exists and is valid": {
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				Obj(),
			existingRuntime: validRuntime,
			wantFieldErrors: 0,
		},
		"returns field error when runtime does not exist": {
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "nonexistent-runtime").
				Obj(),
			existingRuntime: nil,
			wantFieldErrors: 1,
			wantErrorPath:   "spec.runtimeRef",
			wantErrorSubstr: "specified trainingRuntime must be created before the TrainJob is created",
		},
		"returns field error when runtime is in different namespace": {
			trainJob: testingutil.MakeTrainJobWrapper("namespace-a", "test-job").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				Obj(),
			existingRuntime: testingutil.MakeTrainingRuntimeWrapper("namespace-b", "test-runtime").
				RuntimeSpec(
					testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper("namespace-b", "test-runtime").Spec).
						Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
						Obj(),
				).Obj(),
			wantFieldErrors: 1,
			wantErrorPath:   "spec.runtimeRef",
			wantErrorSubstr: "specified trainingRuntime must be created before the TrainJob is created",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			clientBuilder := testingutil.NewClientBuilder()
			if tc.existingRuntime != nil {
				clientBuilder = clientBuilder.WithObjects(tc.existingRuntime)
			}
			c := clientBuilder.Build()

			// Initialize the runtime factory
			runtimeFactory, err := NewTrainingRuntime(ctx, c, testingutil.AsIndex(clientBuilder), nil)
			if err != nil {
				t.Fatalf("Failed to initialize TrainingRuntime: %v", err)
			}

			trainingRuntime, ok := runtimeFactory.(*TrainingRuntime)
			if !ok {
				t.Fatal("Expected TrainingRuntime type")
			}

			// Call ValidateObjects
			warnings, fieldErrors := trainingRuntime.ValidateObjects(ctx, nil, tc.trainJob)

			// Verify field errors count
			if len(fieldErrors) != tc.wantFieldErrors {
				t.Errorf("Expected %d field errors, got %d: %v", tc.wantFieldErrors, len(fieldErrors), fieldErrors)
			}

			// Verify error path and message if errors are expected
			if tc.wantFieldErrors > 0 && len(fieldErrors) > 0 {
				firstError := fieldErrors[0]
				if firstError.Field != tc.wantErrorPath {
					t.Errorf("Expected error at field %q, got %q", tc.wantErrorPath, firstError.Field)
				}
				if !strings.Contains(firstError.Error(), tc.wantErrorSubstr) {
					t.Errorf("Expected error message to contain %q, got %q", tc.wantErrorSubstr, firstError.Error())
				}
			}

			// ValidateObjects for TrainingRuntime doesn't return warnings in the current implementation
			if len(warnings) != 0 {
				t.Errorf("Expected no warnings, got %v", warnings)
			}
		})
	}
}

func TestClusterTrainingRuntimeValidateObjects(t *testing.T) {
	resRequests := corev1.ResourceList{
		corev1.ResourceCPU: resource.MustParse("1"),
	}

	validRuntime := testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").
		RuntimeSpec(
			testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").Spec).
				Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
				Obj(),
		).Obj()

	deprecatedRuntime := testingutil.MakeClusterTrainingRuntimeWrapper("deprecated-runtime").
		RuntimeSpec(
			testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeClusterTrainingRuntimeWrapper("deprecated-runtime").Spec).
				Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
				Obj(),
		).Obj()
	deprecatedRuntime.Labels = map[string]string{
		constants.LabelSupport: constants.SupportDeprecated,
	}

	cases := map[string]struct {
		trainJob        *trainer.TrainJob
		existingRuntime *trainer.ClusterTrainingRuntime
		wantFieldErrors int
		wantWarnings    int
		wantErrorPath   string
		wantErrorSubstr string
		wantWarningSubstr string
	}{
		"succeeds when runtime exists and is valid": {
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				Obj(),
			existingRuntime: validRuntime,
			wantFieldErrors: 0,
			wantWarnings:    0,
		},
		"returns field error when runtime does not exist": {
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "nonexistent-runtime").
				Obj(),
			existingRuntime: nil,
			wantFieldErrors: 1,
			wantWarnings:    0,
			wantErrorPath:   "spec.RuntimeRef",
			wantErrorSubstr: "specified clusterTrainingRuntime must be created before the TrainJob is created",
		},
		"returns warning when runtime is deprecated": {
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "deprecated-runtime").
				Obj(),
			existingRuntime:   deprecatedRuntime,
			wantFieldErrors:   0,
			wantWarnings:      1,
			wantWarningSubstr: "deprecated and will be removed in a future release",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			clientBuilder := testingutil.NewClientBuilder()
			if tc.existingRuntime != nil {
				clientBuilder = clientBuilder.WithObjects(tc.existingRuntime)
			}
			c := clientBuilder.Build()

			// Initialize the runtime factory (ClusterTrainingRuntime requires TrainingRuntime to be initialized first)
			_, err := NewTrainingRuntime(ctx, c, testingutil.AsIndex(clientBuilder), nil)
			if err != nil {
				t.Fatalf("Failed to initialize TrainingRuntime: %v", err)
			}

			runtimeFactory, err := NewClusterTrainingRuntime(ctx, c, testingutil.AsIndex(clientBuilder), nil)
			if err != nil {
				t.Fatalf("Failed to initialize ClusterTrainingRuntime: %v", err)
			}

			clusterRuntime, ok := runtimeFactory.(*ClusterTrainingRuntime)
			if !ok {
				t.Fatal("Expected ClusterTrainingRuntime type")
			}

			// Call ValidateObjects
			warnings, fieldErrors := clusterRuntime.ValidateObjects(ctx, nil, tc.trainJob)

			// Verify field errors count
			if len(fieldErrors) != tc.wantFieldErrors {
				t.Errorf("Expected %d field errors, got %d: %v", tc.wantFieldErrors, len(fieldErrors), fieldErrors)
			}

			// Verify warnings count
			if len(warnings) != tc.wantWarnings {
				t.Errorf("Expected %d warnings, got %d: %v", tc.wantWarnings, len(warnings), warnings)
			}

			// Verify error path and message if errors are expected
			if tc.wantFieldErrors > 0 && len(fieldErrors) > 0 {
				firstError := fieldErrors[0]
				if firstError.Field != tc.wantErrorPath {
					t.Errorf("Expected error at field %q, got %q", tc.wantErrorPath, firstError.Field)
				}
				if !strings.Contains(firstError.Error(), tc.wantErrorSubstr) {
					t.Errorf("Expected error message to contain %q, got %q", tc.wantErrorSubstr, firstError.Error())
				}
			}

			// Verify warning message if warnings are expected
			if tc.wantWarnings > 0 && len(warnings) > 0 {
				if !strings.Contains(warnings[0], tc.wantWarningSubstr) {
					t.Errorf("Expected warning to contain %q, got %q", tc.wantWarningSubstr, warnings[0])
				}
			}
		})
	}
}
