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

package controller

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

func setupScheme(t *testing.T) *runtime.Scheme {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add clientgo scheme: %v", err)
	}
	if err := trainer.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add trainer scheme: %v", err)
	}
	return scheme
}

func TestOptimizationJobReconciler_Reconcile(t *testing.T) {
	scheme := setupScheme(t)

	optJob := &trainer.OptimizationJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-optjob",
			Namespace: "default",
		},
		Spec: trainer.OptimizationJobSpec{
			Objectives: []trainer.Objective{
				{
					Metric: "val_loss",
				},
			},
			Parameters: []trainer.Parameter{
				{
					Name: "learning_rate",
					SearchSpace: &trainer.SearchSpace{
						LogUniform: trainer.LogUniformSpace{
							Min:  trainer.Double("0.0001"),
							Max:  trainer.Double("0.1"),
							Type: trainer.ParameterTypeFloat,
						},
					},
				},
			},
			NumTrials:      2,
			ParallelTrials: 1,
			TrainJobTemplate: trainer.TrainJobTemplateSpec{
				Spec: trainer.TrainJobSpec{
					Trainer: &trainer.Trainer{
						Image: ptr.To("docker.io/my-org/model:latest"),
					},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(optJob).
		WithStatusSubresource(optJob).
		Build()

	reconciler := NewOptimizationJobReconciler(fakeClient, events.NewFakeRecorder(100))

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-optjob",
			Namespace: "default",
		},
	}

	// First reconcile: Initializes Created condition
	ctx := context.Background()
	res, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error during first reconcile: %v", err)
	}
	if res.Requeue {
		t.Errorf("expected no requeue")
	}

	// Verify status condition Created
	var updatedOptJob trainer.OptimizationJob
	if err := fakeClient.Get(ctx, req.NamespacedName, &updatedOptJob); err != nil {
		t.Fatalf("failed to get updated OptimizationJob: %v", err)
	}
	if updatedOptJob.Status == nil || len(updatedOptJob.Status.Conditions) == 0 {
		t.Fatalf("expected status conditions to be initialized")
	}
	if updatedOptJob.Status.Conditions[0].Type != OptimizationJobCreated {
		t.Errorf("expected condition type %s, got %s", OptimizationJobCreated, updatedOptJob.Status.Conditions[0].Type)
	}

	// Second reconcile: Provisions trial TrainJob 1
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("unexpected error during trial creation reconcile: %v", err)
	}

	var trainJobList trainer.TrainJobList
	if err := fakeClient.List(ctx, &trainJobList); err != nil {
		t.Fatalf("failed to list trainjobs: %v", err)
	}
	if len(trainJobList.Items) != 1 {
		t.Fatalf("expected 1 trial TrainJob, got %d", len(trainJobList.Items))
	}

	trialJob := trainJobList.Items[0]
	if trialJob.Name != "test-optjob-trial-1" {
		t.Errorf("expected trial name test-optjob-trial-1, got %s", trialJob.Name)
	}

	// Verify environment variable injection KUBEFLOW_TRAINER_OPT_LEARNING_RATE
	if trialJob.Spec.Trainer == nil || len(trialJob.Spec.Trainer.Env) == 0 {
		t.Fatalf("expected injected environment variables in trial TrainJob trainer spec")
	}
	env := trialJob.Spec.Trainer.Env[0]
	if env.Name != "KUBEFLOW_TRAINER_OPT_LEARNING_RATE" {
		t.Errorf("expected env name KUBEFLOW_TRAINER_OPT_LEARNING_RATE, got %s", env.Name)
	}

	// Mark trial 1 complete
	trialJob.Status.Conditions = []metav1.Condition{
		{
			Type:   trainer.TrainJobComplete,
			Status: metav1.ConditionTrue,
		},
	}
	if err := fakeClient.Update(ctx, &trialJob); err != nil {
		t.Fatalf("failed to update trial status: %v", err)
	}

	// Reconcile again: Provisions trial 2
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("unexpected error during trial 2 creation: %v", err)
	}

	if err := fakeClient.List(ctx, &trainJobList); err != nil {
		t.Fatalf("failed to list trainjobs: %v", err)
	}
	if len(trainJobList.Items) != 2 {
		t.Fatalf("expected 2 trial TrainJobs, got %d", len(trainJobList.Items))
	}

	// Mark trial 2 complete
	trialJob2 := trainJobList.Items[1]
	trialJob2.Status.Conditions = []metav1.Condition{
		{
			Type:   trainer.TrainJobComplete,
			Status: metav1.ConditionTrue,
		},
	}
	if err := fakeClient.Update(ctx, &trialJob2); err != nil {
		t.Fatalf("failed to update trial 2 status: %v", err)
	}

	// Final reconcile: Marks OptimizationJob Complete
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("unexpected error during final completion reconcile: %v", err)
	}

	if err := fakeClient.Get(ctx, req.NamespacedName, &updatedOptJob); err != nil {
		t.Fatalf("failed to get final OptimizationJob: %v", err)
	}

	hasComplete := false
	for _, c := range updatedOptJob.Status.Conditions {
		if c.Type == OptimizationJobComplete && c.Status == metav1.ConditionTrue {
			hasComplete = true
			break
		}
	}
	if !hasComplete {
		t.Errorf("expected OptimizationJob to have Complete status condition")
	}
}
