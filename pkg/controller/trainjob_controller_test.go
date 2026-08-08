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
	"errors"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2/ktesting"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestReconcileDeadline_RetryJobSetDeletion(t *testing.T) {
	_, ctx := ktesting.NewTestContext(t)
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	trainJob := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "train-job").
		ActiveDeadlineSeconds(1).
		Obj()
	trainJob.CreationTimestamp = metav1.NewTime(time.Now().Add(-time.Minute))
	jobSet := &jobsetv1alpha2.JobSet{
		ObjectMeta: metav1.ObjectMeta{Name: trainJob.Name, Namespace: trainJob.Namespace},
	}

	deleteErr := errors.New("error deleting JobSet")
	deleteCalls := 0
	clientBuilder := utiltesting.NewClientBuilder().WithObjects(jobSet)
	clientBuilder.WithInterceptorFuncs(interceptor.Funcs{
		Delete: func(ctx context.Context, cli client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			deleteCalls++
			if deleteCalls == 1 {
				return deleteErr
			}
			return cli.Delete(ctx, obj, opts...)
		},
	})
	cli := clientBuilder.Build()
	r := NewTrainJobReconciler(cli, nil, nil)

	if _, err := r.reconcileDeadline(ctx, trainJob); !errors.Is(err, deleteErr) {
		t.Fatalf("reconcileDeadline() error = %v, want %v", err, deleteErr)
	}
	deadlineCond := meta.FindStatusCondition(trainJob.Status.Conditions, trainer.TrainJobFailed)
	if deadlineCond == nil || deadlineCond.Reason != trainer.TrainJobDeadlineExceededReason {
		t.Fatalf("reconcileDeadline() condition = %v, want reason %q", deadlineCond, trainer.TrainJobDeadlineExceededReason)
	}

	if _, err := r.reconcileDeadline(ctx, trainJob); err != nil {
		t.Fatalf("reconcileDeadline() retry returned error: %v", err)
	}
	if deleteCalls != 2 {
		t.Errorf("reconcileDeadline() delete calls = %d, want 2", deleteCalls)
	}
	if err := cli.Get(ctx, client.ObjectKeyFromObject(jobSet), &jobsetv1alpha2.JobSet{}); !apierrors.IsNotFound(err) {
		t.Errorf("Get() error = %v, want NotFound", err)
	}
}
