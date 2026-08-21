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

package controller

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	jobruntimes "github.com/kubeflow/trainer/v2/pkg/runtime"
)

type mockRuntime struct {
	status *trainer.TrainJobStatus
	err    error
}

func (m *mockRuntime) NewObjects(ctx context.Context, trainJob *trainer.TrainJob) ([]runtime.ApplyConfiguration, error) {
	return nil, nil
}

func (m *mockRuntime) RuntimeInfo(trainJob *trainer.TrainJob, runtimeTemplateSpec any, mlPolicy *trainer.MLPolicy, podGroupPolicy *trainer.PodGroupPolicy) (*jobruntimes.Info, error) {
	return nil, nil
}

func (m *mockRuntime) TrainJobStatus(ctx context.Context, trainJob *trainer.TrainJob) (*trainer.TrainJobStatus, error) {
	return m.status, m.err
}

func (m *mockRuntime) EventHandlerRegistrars() []jobruntimes.ReconcilerBuilder {
	return nil
}

func (m *mockRuntime) ValidateObjects(ctx context.Context, old, new *trainer.TrainJob) (admission.Warnings, field.ErrorList) {
	return nil, nil
}

func TestSetTrainJobStatusPreservesFailedCondition(t *testing.T) {
	testCases := map[string]struct {
		reason        string
		runtimeStatus *trainer.TrainJobStatus
	}{
		"preserves ResourcesCreationFailed condition with nil runtime status": {
			reason:        trainer.TrainJobResourcesCreationFailedReason,
			runtimeStatus: nil,
		},
		"preserves ResourcesCreationFailed condition with non-nil runtime status": {
			reason:        trainer.TrainJobResourcesCreationFailedReason,
			runtimeStatus: &trainer.TrainJobStatus{},
		},
		"preserves DeadlineExceeded condition": {
			reason:        trainer.TrainJobDeadlineExceededReason,
			runtimeStatus: &trainer.TrainJobStatus{},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			trainJob := &trainer.TrainJob{}
			setFailedCondition(trainJob, "test failure", tc.reason)

			runtime := &mockRuntime{status: tc.runtimeStatus}

			if err := setTrainJobStatus(context.Background(), runtime, trainJob); err != nil {
				t.Fatalf("setTrainJobStatus returned error: %v", err)
			}

			cond := meta.FindStatusCondition(trainJob.Status.Conditions, trainer.TrainJobFailed)
			if cond == nil {
				t.Fatal("Expected Failed condition to be preserved, but it was overwritten or removed")
			}
			if cond.Reason != tc.reason {
				t.Fatalf("Expected condition reason %s, got %s", tc.reason, cond.Reason)
			}
			if cond.Message != "test failure" {
				t.Fatalf("Expected message 'test failure', got %q", cond.Message)
			}
		})
	}
}
