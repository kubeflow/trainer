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
	"strings"
	"testing"

	"github.com/go-logr/logr/funcr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestReconcile_ClusterTrainingRuntimeReconciler(t *testing.T) {
	cases := map[string]struct {
		clTrainingRuntime     *trainer.ClusterTrainingRuntime
		wantClTrainingRuntime *trainer.ClusterTrainingRuntime
		wantRuntimeLogValue   string
	}{
		"remove existing finalizer during reconciliation": {
			clTrainingRuntime: utiltesting.MakeClusterTrainingRuntimeWrapper("runtime").
				Finalizers(constants.ResourceInUseFinalizer).
				Obj(),
			wantClTrainingRuntime: utiltesting.MakeClusterTrainingRuntimeWrapper("runtime").
				Obj(),
			wantRuntimeLogValue: `"clusterTrainingRuntime"={"name"="runtime"}`,
		},
		"no action when runtime has no finalizer": {
			clTrainingRuntime: utiltesting.MakeClusterTrainingRuntimeWrapper("runtime").
				Obj(),

			wantClTrainingRuntime: utiltesting.MakeClusterTrainingRuntimeWrapper("runtime").
				Obj(),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var logBuffer strings.Builder
			logger := funcr.New(func(_, args string) { logBuffer.WriteString(args) }, funcr.Options{})
			ctx := ctrl.LoggerInto(context.Background(), logger)
			clientBuilder := utiltesting.NewClientBuilder().WithObjects(tc.clTrainingRuntime)
			clientBuilder.WithInterceptorFuncs(interceptor.Funcs{
				Patch: func(ctx context.Context, cli client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					ctrl.LoggerFrom(ctx).Info("Patching ClusterTrainingRuntime")
					return cli.Patch(ctx, obj, patch, opts...)
				},
			})
			cli := clientBuilder.Build()
			r := NewClusterTrainingRuntimeReconciler(cli, nil)
			clRuntimeKey := client.ObjectKeyFromObject(tc.clTrainingRuntime)
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: clRuntimeKey})
			if err != nil {
				t.Fatalf("Reconcile() returned error: %v", err)
			}
			var gotClRuntime trainer.ClusterTrainingRuntime
			if err := cli.Get(ctx, clRuntimeKey, &gotClRuntime); err != nil {
				t.Fatalf("Get() returned error: %v", err)
			}
			if diff := cmp.Diff(tc.wantClTrainingRuntime, &gotClRuntime,
				cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion"),
				cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion"),
			); len(diff) != 0 {
				t.Errorf("Unexpected ClusterTrainingRuntime: (-want, +got): \n%s", diff)
			}
			if tc.wantRuntimeLogValue != "" && !strings.Contains(logBuffer.String(), tc.wantRuntimeLogValue) {
				t.Errorf("Patch context log = %q, want it to contain %q", logBuffer.String(), tc.wantRuntimeLogValue)
			}
		})
	}
}
