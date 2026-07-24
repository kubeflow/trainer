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
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	jobsetconsts "sigs.k8s.io/jobset/pkg/constants"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

func TestReconcileUnsupportedRuntime(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := trainer.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}
	trainJob := &trainer.TrainJob{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: trainer.TrainJobSpec{
			RuntimeRef: trainer.RuntimeRef{
				Name:     "unsupported",
				APIGroup: ptr.To(trainer.GroupVersion.Group),
				Kind:     ptr.To("UnsupportedRuntime"),
			},
		},
	}
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&trainer.TrainJob{}).
		WithObjects(trainJob).
		Build()
	reconciler := NewTrainJobReconciler(k8sClient, nil, nil)

	_, err := reconciler.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(trainJob)})
	if err == nil || !strings.Contains(err.Error(), "unsupported runtime") {
		t.Fatalf("Reconcile() error = %v, want unsupported runtime error", err)
	}
	if err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(trainJob), trainJob); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	failedCond := meta.FindStatusCondition(trainJob.Status.Conditions, trainer.TrainJobFailed)
	if failedCond == nil || failedCond.Status != metav1.ConditionTrue || failedCond.Reason != trainer.TrainJobRuntimeNotSupportedReason {
		t.Errorf("Failed condition = %#v, want True with reason %q", failedCond, trainer.TrainJobRuntimeNotSupportedReason)
	}
}

func TestRemoveTransientFailedCondition(t *testing.T) {
	cases := map[string]struct {
		failedCondition *metav1.Condition
		wantFailed      bool
	}{
		"no Failed condition": {},
		"TrainingRuntimeNotSupported is transient": {
			failedCondition: &metav1.Condition{
				Type:   trainer.TrainJobFailed,
				Status: metav1.ConditionTrue,
				Reason: trainer.TrainJobRuntimeNotSupportedReason,
			},
		},
		"DeadlineExceeded is terminal": {
			failedCondition: &metav1.Condition{
				Type:   trainer.TrainJobFailed,
				Status: metav1.ConditionTrue,
				Reason: trainer.TrainJobDeadlineExceededReason,
			},
			wantFailed: true,
		},
		"FailedJobs is terminal": {
			failedCondition: &metav1.Condition{
				Type:   trainer.TrainJobFailed,
				Status: metav1.ConditionTrue,
				Reason: jobsetconsts.FailedJobsReason,
			},
			wantFailed: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			trainJob := &trainer.TrainJob{}
			if tc.failedCondition != nil {
				trainJob.Status.Conditions = []metav1.Condition{*tc.failedCondition}
			}

			removeTransientFailedCondition(trainJob)

			gotFailed := len(trainJob.Status.Conditions) != 0
			if gotFailed != tc.wantFailed {
				t.Errorf("Failed condition presence = %t, want %t", gotFailed, tc.wantFailed)
			}
		})
	}
}

func TestSetRuntimeNotSupportedFailedCondition(t *testing.T) {
	cases := map[string]struct {
		failedCondition *metav1.Condition
		wantReason      string
	}{
		"no Failed condition": {
			wantReason: trainer.TrainJobRuntimeNotSupportedReason,
		},
		"TrainingRuntimeNotSupported remains transient": {
			failedCondition: &metav1.Condition{
				Type:   trainer.TrainJobFailed,
				Status: metav1.ConditionTrue,
				Reason: trainer.TrainJobRuntimeNotSupportedReason,
			},
			wantReason: trainer.TrainJobRuntimeNotSupportedReason,
		},
		"DeadlineExceeded is preserved": {
			failedCondition: &metav1.Condition{
				Type:   trainer.TrainJobFailed,
				Status: metav1.ConditionTrue,
				Reason: trainer.TrainJobDeadlineExceededReason,
			},
			wantReason: trainer.TrainJobDeadlineExceededReason,
		},
		"FailedJobs is preserved": {
			failedCondition: &metav1.Condition{
				Type:   trainer.TrainJobFailed,
				Status: metav1.ConditionTrue,
				Reason: jobsetconsts.FailedJobsReason,
			},
			wantReason: jobsetconsts.FailedJobsReason,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			trainJob := &trainer.TrainJob{}
			if tc.failedCondition != nil {
				trainJob.Status.Conditions = []metav1.Condition{*tc.failedCondition}
			}

			setRuntimeNotSupportedFailedCondition(trainJob, "unsupported runtime")

			if gotReason := trainJob.Status.Conditions[0].Reason; gotReason != tc.wantReason {
				t.Errorf("Failed condition reason = %q, want %q", gotReason, tc.wantReason)
			}
		})
	}
}
