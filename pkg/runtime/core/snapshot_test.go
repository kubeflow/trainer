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

	"github.com/google/go-cmp/cmp"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	"sigs.k8s.io/yaml"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestGetRuntimeSnapshot(t *testing.T) {
	cases := map[string]struct {
		trainJob          *trainer.TrainJob
		configMap         *corev1.ConfigMap
		inputRuntimeObj   client.Object
		wantRuntimeObj    client.Object
		wantError         string
		wantNotFoundError bool
	}{
		"successfully retrieves ClusterTrainingRuntime snapshot from ConfigMap": {
			trainJob: testingutil.MakeTrainJobWrapper("test-namespace", "test-job").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				Obj(),
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job-runtime-snapshot",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					runtimeDataKey: `apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: test-runtime
spec:
  mlPolicy:
    numNodes: 2
    torch: {}
  template:
    spec:
      replicatedJobs:
      - name: node
        template:
          metadata:
            labels:
              trainer.kubeflow.org/trainjob-ancestor-step: trainer
          spec:
            template:
              spec:
                containers:
                - name: node
                  image: pytorch/pytorch:tag
`,
				},
			},
			inputRuntimeObj: &trainer.ClusterTrainingRuntime{},
			wantRuntimeObj: &trainer.ClusterTrainingRuntime{
				TypeMeta: metav1.TypeMeta{
					Kind:       trainer.ClusterTrainingRuntimeKind,
					APIVersion: trainer.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-runtime",
				},
				Spec: trainer.TrainingRuntimeSpec{
					MLPolicy: &trainer.MLPolicy{
						NumNodes: ptr.To[int32](2),
						MLPolicySource: trainer.MLPolicySource{
							Torch: &trainer.TorchMLPolicySource{},
						},
					},
					Template: trainer.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
								{
									Name: "node",
									Template: batchv1.JobTemplateSpec{
										ObjectMeta: metav1.ObjectMeta{
											Labels: map[string]string{
												"trainer.kubeflow.org/trainjob-ancestor-step": "trainer",
											},
										},
										Spec: batchv1.JobSpec{
											Template: corev1.PodTemplateSpec{
												Spec: corev1.PodSpec{
													Containers: []corev1.Container{
														{
															Name:  "node",
															Image: "pytorch/pytorch:tag",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"successfully retrieves TrainingRuntime snapshot from ConfigMap": {
			trainJob: testingutil.MakeTrainJobWrapper("test-namespace", "test-job").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				Obj(),
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job-runtime-snapshot",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					runtimeDataKey: `apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainingRuntime
metadata:
  name: test-runtime
  namespace: test-namespace
spec:
  mlPolicy:
    numNodes: 2
    torch: {}
  template:
    spec:
      replicatedJobs:
      - name: node
        template:
          metadata:
            labels:
              trainer.kubeflow.org/trainjob-ancestor-step: trainer
          spec:
            template:
              spec:
                containers:
                - name: node
                  image: pytorch/pytorch:tag
`,
				},
			},
			inputRuntimeObj: &trainer.TrainingRuntime{},
			wantRuntimeObj: &trainer.TrainingRuntime{
				TypeMeta: metav1.TypeMeta{
					Kind:       trainer.TrainingRuntimeKind,
					APIVersion: trainer.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runtime",
					Namespace: "test-namespace",
				},
				Spec: trainer.TrainingRuntimeSpec{
					MLPolicy: &trainer.MLPolicy{
						NumNodes: ptr.To[int32](2),
						MLPolicySource: trainer.MLPolicySource{
							Torch: &trainer.TorchMLPolicySource{},
						},
					},
					Template: trainer.JobSetTemplateSpec{
						Spec: jobsetv1alpha2.JobSetSpec{
							ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
								{
									Name: "node",
									Template: batchv1.JobTemplateSpec{
										ObjectMeta: metav1.ObjectMeta{
											Labels: map[string]string{
												"trainer.kubeflow.org/trainjob-ancestor-step": "trainer",
											},
										},
										Spec: batchv1.JobSpec{
											Template: corev1.PodTemplateSpec{
												Spec: corev1.PodSpec{
													Containers: []corev1.Container{
														{
															Name:  "node",
															Image: "pytorch/pytorch:tag",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"returns not found error when ConfigMap does not exist": {
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				Obj(),
			configMap:         nil,
			inputRuntimeObj:   &trainer.ClusterTrainingRuntime{},
			wantNotFoundError: true,
		},
		"returns error when ConfigMap is missing runtime data key": {
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				Obj(),
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job" + runtimeSnapshotSuffix,
					Namespace: metav1.NamespaceDefault,
				},
				Data: map[string]string{
					"wrong-key": "",
				},
			},
			inputRuntimeObj: &trainer.ClusterTrainingRuntime{},
			wantError:       "invalid runtime snapshot: snapshot ConfigMap missing \"runtime\" data key",
		},
		"returns error when ConfigMap contains invalid YAML": {
			trainJob: testingutil.MakeTrainJobWrapper("test-namespace", "test-job").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				Obj(),
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job" + runtimeSnapshotSuffix,
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					runtimeDataKey: "invalid: yaml: content:\n  - broken",
				},
			},
			inputRuntimeObj: &trainer.ClusterTrainingRuntime{},
			wantError:       "invalid runtime snapshot: unable to unmarshall the snapshot",
		},
		"returns error when snapshot runtime name does not match RuntimeRef": {
			trainJob: testingutil.MakeTrainJobWrapper("test-namespace", "test-job").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				Obj(),
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job" + runtimeSnapshotSuffix,
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					runtimeDataKey: `apiVersion: trainer.kubeflow.org/v1alpha1
kind: TrainingRuntime
metadata:
  name: other-runtime
  namespace: test-namespace
spec:
  mlPolicy:
    numNodes: 1
  template:
    spec:
      replicatedJobs: []
`,
				},
			},
			inputRuntimeObj: &trainer.TrainingRuntime{},
			wantError:       "invalid runtime snapshot: the snapshot refers to the wrong runtime: expecting a runtime with name, api group and kind of \"test-runtime\", \"trainer.kubeflow.org\", \"TrainingRuntime\" but found runtime with name, api group and kind of \"other-runtime\", \"trainer.kubeflow.org\", \"TrainingRuntime\"",
		},
		"returns error when snapshot runtime kind does not match RuntimeRef": {
			trainJob: testingutil.MakeTrainJobWrapper("test-namespace", "test-job").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-runtime").
				Obj(),
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job" + runtimeSnapshotSuffix,
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					runtimeDataKey: `apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: test-runtime
spec:
  mlPolicy:
    numNodes: 1
  template:
    spec:
      replicatedJobs: []
`,
				},
			},
			inputRuntimeObj: &trainer.TrainingRuntime{},
			wantError:       "invalid runtime snapshot: the snapshot refers to the wrong runtime: expecting a runtime with name, api group and kind of \"test-runtime\", \"trainer.kubeflow.org\", \"TrainingRuntime\" but found runtime with name, api group and kind of \"test-runtime\", \"trainer.kubeflow.org\", \"ClusterTrainingRuntime\"",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			clientBuilder := testingutil.NewClientBuilder()
			if tc.configMap != nil {
				clientBuilder = clientBuilder.WithObjects(tc.configMap)
			}
			c := clientBuilder.Build()

			runtimeObj := tc.inputRuntimeObj.DeepCopyObject().(client.Object)
			err := getRuntimeSnapshot(ctx, c, tc.trainJob, runtimeObj)

			if tc.wantRuntimeObj != nil {
				// check runtime object read from the configmap is correct
				if diff := cmp.Diff(tc.wantRuntimeObj, runtimeObj); diff != "" {
					t.Errorf("Unexpected retrieved runtime object (-want +got):\n%s", diff)
				}

				// check the runtime matches the TrainJob RuntimeRef
				gvk := runtimeObj.GetObjectKind().GroupVersionKind()
				if tc.trainJob.Spec.RuntimeRef.Kind != nil && gvk.Kind != *tc.trainJob.Spec.RuntimeRef.Kind {
					t.Errorf("Runtime kind mismatch: expected %s, got %s", *tc.trainJob.Spec.RuntimeRef.Kind, gvk.Kind)
				}
				if tc.trainJob.Spec.RuntimeRef.APIGroup != nil && gvk.Group != *tc.trainJob.Spec.RuntimeRef.APIGroup {
					t.Errorf("Runtime group mismatch: expected %s, got %s", *tc.trainJob.Spec.RuntimeRef.APIGroup, gvk.Group)
				}
			}

			if tc.wantError != "" {
				if err == nil {
					t.Fatalf("Expected error containing %q, got nil", tc.wantError)
				}

				if !strings.Contains(err.Error(), tc.wantError) {
					t.Errorf("Expected error containing %q, got %q", tc.wantError, err.Error())
				}
			}

			if tc.wantNotFoundError {
				if !apierrors.IsNotFound(err) {
					t.Errorf("Expected not found error but got %q", err.Error())
				}
			}

		})
	}
}

func TestCreateRuntimeSnapshot(t *testing.T) {
	resRequests := corev1.ResourceList{
		corev1.ResourceCPU: resource.MustParse("1"),
	}

	clusterRuntime := testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").
		RuntimeSpec(
			testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").Spec).
				Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
				Obj(),
		).Obj()

	namespacedRuntime := testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-ns-runtime").
		RuntimeSpec(
			testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-ns-runtime").Spec).
				Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
				Obj(),
		).Obj()

	cases := map[string]struct {
		trainJob   *trainer.TrainJob
		runtimeObj client.Object
		existingCM *corev1.ConfigMap // Pre-existing ConfigMap to test idempotency
	}{
		"successfully creates snapshot ConfigMap for ClusterTrainingRuntime": {
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("test-uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				Obj(),
			runtimeObj: clusterRuntime,
		},
		"successfully creates snapshot ConfigMap for TrainingRuntime": {
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("test-uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.TrainingRuntimeKind), "test-ns-runtime").
				Obj(),
			runtimeObj: namespacedRuntime,
		},
		"idempotently updates existing snapshot ConfigMap": {
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("test-uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				Obj(),
			runtimeObj: clusterRuntime,
			existingCM: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job" + runtimeSnapshotSuffix,
					Namespace: metav1.NamespaceDefault,
				},
				Data: map[string]string{
					runtimeDataKey: "old-data",
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			clientBuilder := testingutil.NewClientBuilder()
			if tc.existingCM != nil {
				clientBuilder = clientBuilder.WithObjects(tc.existingCM)
			}
			c := clientBuilder.Build()

			err := createRuntimeSnapshot(ctx, c, tc.trainJob, tc.runtimeObj)
			if err != nil {
				t.Fatalf("Unexpected error creating snapshot: %v", err)
			}

			// Verify ConfigMap was created/updated
			cm := &corev1.ConfigMap{}
			cmKey := client.ObjectKey{
				Name:      tc.trainJob.Name + runtimeSnapshotSuffix,
				Namespace: tc.trainJob.Namespace,
			}
			if err := c.Get(ctx, cmKey, cm); err != nil {
				t.Fatalf("Failed to get created ConfigMap: %v", err)
			}

			// Verify owner reference
			if len(cm.OwnerReferences) == 0 {
				t.Fatal("ConfigMap should have owner reference")
			}

			// Verify runtime data is stored correctly
			runtimeYAML, ok := cm.Data[runtimeDataKey]
			if !ok {
				t.Fatalf("ConfigMap missing %q data key", runtimeDataKey)
			}

			// Verify YAML can be unmarshaled back to runtime object
			var unmarshalledRuntime interface{}
			switch tc.runtimeObj.(type) {
			case *trainer.ClusterTrainingRuntime:
				unmarshalledRuntime = &trainer.ClusterTrainingRuntime{}
			case *trainer.TrainingRuntime:
				unmarshalledRuntime = &trainer.TrainingRuntime{}
			}

			if err := yaml.Unmarshal([]byte(runtimeYAML), unmarshalledRuntime); err != nil {
				t.Fatalf("Failed to unmarshal stored YAML: %v", err)
			}

			// Verify the unmarshalled runtime matches the original
			if diff := cmp.Diff(tc.runtimeObj, unmarshalledRuntime); diff != "" {
				t.Errorf("Runtime content mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRuntimeSnapshotRoundTrip(t *testing.T) {
	// Test that a runtime can be stored and retrieved without data loss
	resRequests := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("2"),
		corev1.ResourceMemory: resource.MustParse("4Gi"),
	}

	originalRuntime := testingutil.MakeClusterTrainingRuntimeWrapper("complex-runtime").
		RuntimeSpec(
			testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeClusterTrainingRuntimeWrapper("complex-runtime").Spec).
				Container(constants.Node, constants.Node, "test:runtime:v1.2.3", []string{"torchrun", "train.py"}, []string{"--epochs=10"}, resRequests).
				WithMLPolicy(
					testingutil.MakeMLPolicyWrapper().
						WithNumNodes(4).
						Obj(),
				).
				Obj(),
		).Obj()

	trainJob := testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
		UID("test-uid-12345").
		RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "complex-runtime").
		Obj()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	c := testingutil.NewClientBuilder().Build()

	// Store the runtime
	if err := createRuntimeSnapshot(ctx, c, trainJob, originalRuntime); err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	// Retrieve the runtime
	retrievedRuntime := &trainer.ClusterTrainingRuntime{}
	if err := getRuntimeSnapshot(ctx, c, trainJob, retrievedRuntime); err != nil {
		t.Fatalf("Failed to get snapshot: %v", err)
	}

	// Verify round-trip preserves data
	if diff := cmp.Diff(originalRuntime.Spec, retrievedRuntime.Spec); diff != "" {
		t.Errorf("Runtime spec mismatch after round-trip (-want +got):\n%s", diff)
	}

	// Verify metadata is preserved
	if originalRuntime.Name != retrievedRuntime.Name {
		t.Errorf("Runtime name mismatch: expected %s, got %s", originalRuntime.Name, retrievedRuntime.Name)
	}
}
