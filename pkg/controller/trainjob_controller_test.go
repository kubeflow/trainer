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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2/ktesting"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	jobruntimes "github.com/kubeflow/trainer/v2/pkg/runtime"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestReconcile_TrainJobReconciler_UnsupportedRuntime(t *testing.T) {
	trainJob := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "trainjob").
		RuntimeRef(schema.GroupVersionKind{Group: "trainer.kubeflow.org", Kind: "UnsupportedRuntime"}, "unsupported").
		Obj()

	_, ctx := ktesting.NewTestContext(t)
	var cancel func()
	ctx, cancel = context.WithCancel(ctx)
	t.Cleanup(cancel)

	cli := utiltesting.NewClientBuilder().
		WithObjects(trainJob).
		WithStatusSubresource(trainJob).
		Build()

	r := NewTrainJobReconciler(cli, nil, map[string]jobruntimes.Runtime{})
	trainJobKey := client.ObjectKeyFromObject(trainJob)

	_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: trainJobKey})
	if err == nil {
		t.Fatalf("Expected error for unsupported runtime, got nil")
	}

	var gotTrainJob trainer.TrainJob
	if err := cli.Get(ctx, trainJobKey, &gotTrainJob); err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}

	failedCond := meta.FindStatusCondition(gotTrainJob.Status.Conditions, trainer.TrainJobFailed)
	if failedCond == nil {
		t.Fatalf("Expected Failed condition, got nil")
	}
	if failedCond.Reason != trainer.TrainJobRuntimeNotSupportedReason {
		t.Errorf("Unexpected condition reason: want %s, got %s", trainer.TrainJobRuntimeNotSupportedReason, failedCond.Reason)
	}
}
