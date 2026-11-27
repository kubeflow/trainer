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
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	"sigs.k8s.io/yaml"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

// TestNewObjectsRuntimeSnapshotCreateIfAbsent verifies that an existing runtime snapshot stays authoritative
// when the cached ConfigMap read is stale and reports NotFound, and that the objects are built from it.
func TestNewObjectsRuntimeSnapshotCreateIfAbsent(t *testing.T) {
	const (
		oldImage = "test:v1"
		newImage = "test:v2"
		jobName  = "test-job"
	)
	errCreate := errors.New("create failed")
	errGet := errors.New("get failed")
	resRequests := corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")}

	buildSpec := func(image string) trainer.TrainingRuntimeSpec {
		return testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").Spec).
			Container(constants.Node, constants.Node, image, []string{"runtime"}, []string{"runtime"}, resRequests).
			Obj()
	}
	trainingRuntime := func(image string) *trainer.TrainingRuntime {
		return testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").
			RuntimeSpec(buildSpec(image)).Obj()
	}
	clusterRuntime := func(image string) *trainer.ClusterTrainingRuntime {
		return testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").
			RuntimeSpec(buildSpec(image)).Obj()
	}
	snapshotKey := client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: jobName + runtimeSnapshotSuffix}
	isConfigMap := func(obj client.Object) bool {
		_, ok := obj.(*corev1.ConfigMap)
		return ok
	}

	cases := map[string]struct {
		runtimeKind string
		// existingImage, when set, seeds a snapshot built from a runtime with this image.
		existingImage string
		// staleCache makes cached ConfigMap reads report NotFound although the API server has the object.
		staleCache bool
		// apiReaderGetErr makes uncached ConfigMap reads fail.
		apiReaderGetErr error
		// createErr makes ConfigMap creation fail.
		createErr error
		wantImage string
		wantErr   error
		// wantSnapshotImage is the image expected in the stored snapshot, if one is expected.
		wantSnapshotImage string
	}{
		"TrainingRuntime: first creation snapshots the live runtime": {
			runtimeKind:       trainer.TrainingRuntimeKind,
			wantImage:         newImage,
			wantSnapshotImage: newImage,
		},
		"TrainingRuntime: stale cache does not overwrite the existing snapshot": {
			runtimeKind:       trainer.TrainingRuntimeKind,
			existingImage:     oldImage,
			staleCache:        true,
			wantImage:         oldImage,
			wantSnapshotImage: oldImage,
		},
		"TrainingRuntime: existing snapshot found by the cache is used": {
			runtimeKind:       trainer.TrainingRuntimeKind,
			existingImage:     oldImage,
			wantImage:         oldImage,
			wantSnapshotImage: oldImage,
		},
		"TrainingRuntime: errors other than AlreadyExists are propagated": {
			runtimeKind: trainer.TrainingRuntimeKind,
			createErr:   errCreate,
			wantErr:     errCreate,
		},
		"TrainingRuntime: failure to read the existing snapshot is propagated": {
			runtimeKind:       trainer.TrainingRuntimeKind,
			existingImage:     oldImage,
			staleCache:        true,
			apiReaderGetErr:   errGet,
			wantErr:           errGet,
			wantSnapshotImage: oldImage,
		},
		"ClusterTrainingRuntime: first creation snapshots the live runtime": {
			runtimeKind:       trainer.ClusterTrainingRuntimeKind,
			wantImage:         newImage,
			wantSnapshotImage: newImage,
		},
		"ClusterTrainingRuntime: stale cache does not overwrite the existing snapshot": {
			runtimeKind:       trainer.ClusterTrainingRuntimeKind,
			existingImage:     oldImage,
			staleCache:        true,
			wantImage:         oldImage,
			wantSnapshotImage: oldImage,
		},
		"ClusterTrainingRuntime: errors other than AlreadyExists are propagated": {
			runtimeKind: trainer.ClusterTrainingRuntimeKind,
			createErr:   errCreate,
			wantErr:     errCreate,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			trainJob := testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, jobName).
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(tc.runtimeKind), "test-runtime").
				Obj()

			// The live runtime always carries the new image.
			clientBuilder := testingutil.NewClientBuilder()
			if tc.runtimeKind == trainer.ClusterTrainingRuntimeKind {
				clientBuilder.WithObjects(clusterRuntime(newImage))
			} else {
				clientBuilder.WithObjects(trainingRuntime(newImage))
			}
			api := clientBuilder.Build()

			if tc.existingImage != "" {
				var seed client.Object = trainingRuntime(tc.existingImage)
				if tc.runtimeKind == trainer.ClusterTrainingRuntimeKind {
					seed = clusterRuntime(tc.existingImage)
				}
				if err := createRuntimeSnapshot(ctx, api, trainJob, seed); err != nil {
					t.Fatalf("Failed to seed the runtime snapshot: %v", err)
				}
			}

			// cached simulates the manager's informer-backed client.
			cached := interceptor.NewClient(api, interceptor.Funcs{
				Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if tc.staleCache && isConfigMap(obj) {
						return apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, key.Name)
					}
					return c.Get(ctx, key, obj, opts...)
				},
				Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					if tc.createErr != nil && isConfigMap(obj) {
						return tc.createErr
					}
					return c.Create(ctx, obj, opts...)
				},
			})
			// apiReader simulates the manager's uncached API reader.
			apiReader := interceptor.NewClient(api, interceptor.Funcs{
				Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if tc.apiReaderGetErr != nil && isConfigMap(obj) {
						return tc.apiReaderGetErr
					}
					return c.Get(ctx, key, obj, opts...)
				},
			})

			trainingRuntimeFactory, err := NewTrainingRuntime(ctx, cached, apiReader, testingutil.AsIndex(clientBuilder), nil)
			if err != nil {
				t.Fatal(err)
			}
			rt := trainingRuntimeFactory
			if tc.runtimeKind == trainer.ClusterTrainingRuntimeKind {
				rt, err = NewClusterTrainingRuntime(ctx, cached, apiReader, testingutil.AsIndex(clientBuilder), nil)
				if err != nil {
					t.Fatal(err)
				}
			}

			objs, err := rt.NewObjects(ctx, trainJob)
			if diff := cmp.Diff(tc.wantErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("Unexpected error (-want,+got):\n%s", diff)
			}

			if tc.wantSnapshotImage != "" {
				cm := &corev1.ConfigMap{}
				if err := api.Get(ctx, snapshotKey, cm); err != nil {
					t.Fatalf("Failed to get the runtime snapshot: %v", err)
				}
				gotImage := snapshotNodeImage(t, tc.runtimeKind, cm.Data[runtimeDataKey])
				if gotImage != tc.wantSnapshotImage {
					t.Errorf("Unexpected image in the runtime snapshot: want %q, got %q", tc.wantSnapshotImage, gotImage)
				}
			} else if err := api.Get(ctx, snapshotKey, &corev1.ConfigMap{}); !apierrors.IsNotFound(err) {
				t.Errorf("Expected no runtime snapshot to exist, got error: %v", err)
			}

			if tc.wantErr != nil {
				return
			}
			resultObjs, err := testingutil.ToObject(api.Scheme(), objs...)
			if err != nil {
				t.Fatalf("Pipeline built unrecognizable objects: %v", err)
			}
			var gotImage string
			for _, o := range resultObjs {
				if jobSet, ok := o.(*jobsetv1alpha2.JobSet); ok {
					gotImage = nodeContainerImage(jobSet.Spec.ReplicatedJobs)
				}
			}
			if gotImage != tc.wantImage {
				t.Errorf("Unexpected node image in the JobSet: want %q, got %q", tc.wantImage, gotImage)
			}
		})
	}
}

func nodeContainerImage(replicatedJobs []jobsetv1alpha2.ReplicatedJob) string {
	for _, rj := range replicatedJobs {
		if rj.Name != constants.Node {
			continue
		}
		for _, c := range rj.Template.Spec.Template.Spec.Containers {
			if c.Name == constants.Node {
				return c.Image
			}
		}
	}
	return ""
}

func snapshotNodeImage(t *testing.T, kind, snapshotYAML string) string {
	t.Helper()
	var template trainer.JobSetTemplateSpec
	if kind == trainer.ClusterTrainingRuntimeKind {
		rt := &trainer.ClusterTrainingRuntime{}
		if err := yaml.Unmarshal([]byte(snapshotYAML), rt); err != nil {
			t.Fatalf("Failed to unmarshal the snapshot: %v", err)
		}
		template = rt.Spec.Template
	} else {
		rt := &trainer.TrainingRuntime{}
		if err := yaml.Unmarshal([]byte(snapshotYAML), rt); err != nil {
			t.Fatalf("Failed to unmarshal the snapshot: %v", err)
		}
		template = rt.Spec.Template
	}
	return nodeContainerImage(template.Spec.ReplicatedJobs)
}
