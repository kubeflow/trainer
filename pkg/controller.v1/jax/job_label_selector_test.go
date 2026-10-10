// Copyright 2021 The Kubeflow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jax

import (
	"context"
	kubeflowv1 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v1"
	"github.com/kubeflow/training-operator/pkg/controller.v1/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"testing"
)

// Only Get is allowed: any other client operation panics through the nil embedded client.
// In particular, skipped jobs must not list/create children or write status.
type selectorGetOnlyClient struct {
	client.Client
	reader client.Reader
}

func (c selectorGetOnlyClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return c.reader.Get(ctx, key, obj, opts...)
}

func TestJobLabelSelectorSkipsReconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := kubeflowv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	selector, err := labels.Parse("routing.example.org/controller=new")
	if err != nil {
		t.Fatal(err)
	}
	for _, jobLabels := range []map[string]string{nil, {"routing.example.org/controller": "old"}} {
		job := &kubeflowv1.JAXJob{ObjectMeta: metav1.ObjectMeta{Name: "existing", Namespace: "test", Labels: jobLabels}}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(job).Build()
		r := &JAXJobReconciler{client: selectorGetOnlyClient{reader: c}, JobController: common.JobController{JobLabelSelector: selector}}
		result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(job)})
		if err != nil || result != (ctrl.Result{}) {
			t.Fatalf("skipped job: result=%v err=%v", result, err)
		}
		before := job.DeepCopy()
		if r.onOwnerCreateFunc()(event.TypedCreateEvent[*kubeflowv1.JAXJob]{Object: job}) {
			t.Fatal("nonmatching create was accepted")
		}
		if !reflect.DeepEqual(before, job) {
			t.Fatal("nonmatching create mutated the job")
		}
	}
}

// Matching and default configurations must still execute the existing create handler.
func TestJobLabelSelectorAcceptsCreate(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := kubeflowv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	selector, err := labels.Parse("routing.example.org/controller=new")
	if err != nil {
		t.Fatal(err)
	}
	for _, selected := range []labels.Selector{nil, labels.Everything(), selector} {
		job := &kubeflowv1.JAXJob{ObjectMeta: metav1.ObjectMeta{Name: "new-job", Namespace: "test", Labels: map[string]string{"routing.example.org/controller": "new"}}}
		r := &JAXJobReconciler{scheme: scheme, JobController: common.JobController{JobLabelSelector: selected}}
		if !r.onOwnerCreateFunc()(event.TypedCreateEvent[*kubeflowv1.JAXJob]{Object: job}) {
			t.Fatal("matching create was rejected")
		}
		if len(job.Status.Conditions) == 0 {
			t.Fatal("matching create did not initialize job status")
		}
	}
}
