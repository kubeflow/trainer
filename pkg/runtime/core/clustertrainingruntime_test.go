/*
Copyright 2024 The Kubeflow Authors.

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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"
	volcanov1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	testingutil "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestClusterTrainingRuntimeNewObjects(t *testing.T) {

	resRequests := corev1.ResourceList{
		corev1.ResourceCPU: resource.MustParse("1"),
	}

	cases := map[string]struct {
		trainJob               *trainer.TrainJob
		clusterTrainingRuntime *trainer.ClusterTrainingRuntime
		wantObjs               []runtime.Object
		wantError              error
	}{
		"succeeded to build PodGroup and JobSet with NumNodes from the Runtime and container from the Trainer.": {
			clusterTrainingRuntime: testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").RuntimeSpec(
				testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").Spec).
					JobSetSpec(
						testingutil.MakeJobSetWrapper("", "").
							DependsOn(constants.Node,
								[]jobsetv1alpha2.DependsOn{
									{
										Name:   constants.DatasetInitializer,
										Status: jobsetv1alpha2.DependencyComplete,
									},
									{
										Name:   constants.ModelInitializer,
										Status: jobsetv1alpha2.DependencyComplete,
									},
								}...,
							).
							Obj().
							Spec,
					).
					Container(constants.DatasetInitializer, constants.DatasetInitializer, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Container(constants.ModelInitializer, constants.ModelInitializer, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					WithMLPolicy(
						testingutil.MakeMLPolicyWrapper().
							WithNumNodes(100).
							Obj(),
					).
					Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					PodGroupPolicyCoschedulingSchedulingTimeout(120).
					Obj(),
			).Obj(),
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				Suspend(true).
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						Container("test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
						Obj(),
				).
				Obj(),
			wantObjs: []runtime.Object{
				testingutil.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					Suspend(true).
					PodLabel(schedulerpluginsv1alpha1.PodGroupLabel, "test-job").
					Replicas(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Node, constants.Launcher).
					Parallelism(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Launcher).
					Completions(1, constants.DatasetInitializer, constants.ModelInitializer, constants.Launcher).
					NumNodes(100).
					Container(constants.DatasetInitializer, constants.DatasetInitializer, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Container(constants.ModelInitializer, constants.ModelInitializer, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Container(constants.Node, constants.Node, "test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
					DependsOn(constants.Node,
						[]jobsetv1alpha2.DependsOn{
							{
								Name:   constants.DatasetInitializer,
								Status: jobsetv1alpha2.DependencyComplete,
							},
							{
								Name:   constants.ModelInitializer,
								Status: jobsetv1alpha2.DependencyComplete,
							},
						}...,
					).
					Obj(),
				testingutil.MakeSchedulerPluginsPodGroup(metav1.NamespaceDefault, "test-job").
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "uid").
					MinMember(102). // 102 replicas = 100 Trainer nodes + 2 Initializers.
					MinResources(corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("102"), // Trainer node has 100 CPUs + 2 CPUs from 2 initializer containers.
					}).
					SchedulingTimeout(120).
					Obj(),
			},
		},
		"missing trainingRuntime resource": {
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				Trainer(
					testingutil.MakeTrainJobTrainerWrapper().
						Obj(),
				).
				Obj(),
			wantError: errorNotFoundSpecifiedClusterTrainingRuntime,
		},
	}
	cmpOpts := []cmp.Option{
		cmpopts.SortSlices(func(a, b runtime.Object) bool {
			return a.GetObjectKind().GroupVersionKind().String() < b.GetObjectKind().GroupVersionKind().String()
		}),
		cmpopts.EquateEmpty(),
		cmpopts.SortMaps(func(a, b string) bool { return a < b }),
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			clientBuilder := testingutil.NewClientBuilder()
			if tc.clusterTrainingRuntime != nil {
				clientBuilder.WithObjects(tc.clusterTrainingRuntime)
			}
			c := clientBuilder.Build()

			trainingRuntime, err := NewTrainingRuntime(ctx, c, testingutil.AsIndex(clientBuilder), nil)
			if err != nil {
				t.Fatal(err)
			}
			var ok bool
			trainingRuntimeFactory, ok = trainingRuntime.(*TrainingRuntime)
			if !ok {
				t.Fatal("Failed type assertion from Runtime interface to TrainingRuntime")
			}

			clTrainingRuntime, err := NewClusterTrainingRuntime(ctx, c, testingutil.AsIndex(clientBuilder), nil)
			if err != nil {
				t.Fatal(err)
			}

			objs, err := clTrainingRuntime.NewObjects(ctx, tc.trainJob)
			if diff := cmp.Diff(tc.wantError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error (-want,+got):\n%s", diff)
			}

			resultObjs, err := testingutil.ToObject(c.Scheme(), objs...)
			if err != nil {
				t.Errorf("Pipeline built unrecognizable objects: %v", err)
			}

			if diff := cmp.Diff(tc.wantObjs, resultObjs, cmpOpts...); len(diff) != 0 {
				t.Errorf("Unexpected objects (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestClusterTrainingRuntimeValidateObjects(t *testing.T) {
	validRuntime := testingutil.MakeClusterTrainingRuntimeWrapper("test-runtime").Obj()

	deprecatedRuntime := testingutil.MakeClusterTrainingRuntimeWrapper("deprecated-runtime").Obj()
	deprecatedRuntime.Labels = map[string]string{
		constants.LabelSupport: constants.SupportDeprecated,
	}

	cases := map[string]struct {
		trainJob        *trainer.TrainJob
		existingRuntime *trainer.ClusterTrainingRuntime
		wantErrs        field.ErrorList
		wantWarnings    admission.Warnings
	}{
		"succeeds when runtime exists and is valid": {
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "test-runtime").
				Obj(),
			existingRuntime: validRuntime,
			wantErrs:        nil,
			wantWarnings:    nil,
		},
		"returns field error when runtime does not exist": {
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "nonexistent-runtime").
				Obj(),
			existingRuntime: nil,
			wantErrs: field.ErrorList{
				field.Invalid(field.NewPath("spec", "runtimeRef"), trainer.RuntimeRef{
					Name:     "nonexistent-runtime",
					APIGroup: ptr.To(trainer.GroupVersion.Group),
					Kind:     ptr.To(trainer.ClusterTrainingRuntimeKind),
				}, ""),
			},
			wantWarnings: nil,
		},
		"returns warning when runtime is deprecated": {
			trainJob: testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), "deprecated-runtime").
				Obj(),
			existingRuntime: deprecatedRuntime,
			wantErrs:        nil,
			wantWarnings: admission.Warnings{
				`Referenced ClusterTrainingRuntime "deprecated-runtime" is deprecated and will be removed in a future release of Kubeflow Trainer. See runtime deprecation policy: ` + constants.RuntimeDeprecationPolicyURL,
			},
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

			clTrainingRuntime, err := NewClusterTrainingRuntime(ctx, c, testingutil.AsIndex(clientBuilder), nil)
			if err != nil {
				t.Fatal(err)
			}

			warnings, fieldErrors := clTrainingRuntime.ValidateObjects(ctx, nil, tc.trainJob)

			if diff := cmp.Diff(tc.wantErrs, fieldErrors, cmpopts.IgnoreFields(field.Error{}, "Detail")); diff != "" {
				t.Errorf("Unexpected field errors (-want,+got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantWarnings, warnings); diff != "" {
				t.Errorf("Unexpected warnings (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestClusterTrainingRuntimeDeleteObjects(t *testing.T) {
	errSnapshotGet := errors.New("failed to read snapshot ConfigMap")

	resRequests := corev1.ResourceList{
		corev1.ResourceCPU: resource.MustParse("1"),
	}

	runtimeWithVolcano := func(name string) *trainer.ClusterTrainingRuntime {
		return testingutil.MakeClusterTrainingRuntimeWrapper(name).
			RuntimeSpec(
				testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeClusterTrainingRuntimeWrapper(name).Spec).
					PodGroupPolicyVolcano(&trainer.VolcanoPodGroupPolicySource{}).
					Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Obj(),
			).Obj()
	}

	// runtimeWithBrokenTorch declares a Torch envInjection target that the template does not
	// have, so the Torch plugin rejects the runtime while its Info is being built.
	runtimeWithBrokenTorch := func(name string) *trainer.ClusterTrainingRuntime {
		return testingutil.MakeClusterTrainingRuntimeWrapper(name).
			RuntimeSpec(
				testingutil.MakeTrainingRuntimeSpecWrapper(testingutil.MakeClusterTrainingRuntimeWrapper(name).Spec).
					WithMLPolicy(testingutil.MakeMLPolicyWrapper().
						WithMLPolicySource(*testingutil.MakeMLPolicySourceWrapper().
							TorchPolicyWithEnvInjection(&trainer.EnvInjection{
								Targets: []trainer.EnvInjectionTarget{{JobName: constants.Node, ContainerNames: []string{"missing"}}},
							}).Obj()).
						Obj()).
					PodGroupPolicyVolcano(&trainer.VolcanoPodGroupPolicySource{}).
					Container(constants.Node, constants.Node, "test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Obj(),
			).Obj()
	}

	failedTrainJob := func(name, runtimeName string) *trainer.TrainJob {
		return testingutil.MakeTrainJobWrapper(metav1.NamespaceDefault, name).
			UID("uid").
			RuntimeRef(trainer.SchemeGroupVersion.WithKind(trainer.ClusterTrainingRuntimeKind), runtimeName).
			Conditions(metav1.Condition{Type: trainer.TrainJobFailed, Status: metav1.ConditionTrue}).
			Trainer(testingutil.MakeTrainJobTrainerWrapper().Obj()).
			Obj()
	}

	ownedPodGroup := func(name string) *volcanov1beta1.PodGroup {
		// No TypeMeta: Delete() returns whatever the fake client's Get() populates,
		// which (like a real client) leaves TypeMeta unset on a typed object.
		pg := testingutil.MakeVolcanoPodGroup(metav1.NamespaceDefault, name).
			ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), name, "uid").
			Obj()
		pg.TypeMeta = metav1.TypeMeta{}
		return pg
	}

	cases := map[string]struct {
		clusterTrainingRuntime *trainer.ClusterTrainingRuntime
		trainJob               *trainer.TrainJob
		podGroup               *volcanov1beta1.PodGroup
		withSnapshot           bool
		invalidSnapshot        bool
		// omitLiveRuntime seeds the snapshot but not the live runtime, as when the runtime is
		// deleted after the TrainJob finishes.
		omitLiveRuntime bool
		// snapshotGetErr makes reading the snapshot ConfigMap fail.
		snapshotGetErr error
		wantObjs       []client.Object
		wantErr        error
	}{
		"resolves from an existing snapshot": {
			clusterTrainingRuntime: runtimeWithVolcano("test-runtime"),
			trainJob:               failedTrainJob("test-job", "test-runtime"),
			podGroup:               ownedPodGroup("test-job"),
			withSnapshot:           true,
			wantObjs:               []client.Object{ownedPodGroup("test-job")},
		},
		"no snapshot falls back to the live ClusterTrainingRuntime": {
			clusterTrainingRuntime: runtimeWithVolcano("test-runtime"),
			trainJob:               failedTrainJob("test-job", "test-runtime"),
			podGroup:               ownedPodGroup("test-job"),
			wantObjs:               []client.Object{ownedPodGroup("test-job")},
		},
		"ClusterTrainingRuntime no longer exists returns no objects and no error": {
			trainJob: failedTrainJob("test-job", "test-runtime"),
			podGroup: ownedPodGroup("test-job"),
			wantObjs: nil,
		},
		"invalid snapshot returns no objects and no error": {
			clusterTrainingRuntime: runtimeWithVolcano("test-runtime"),
			trainJob:               failedTrainJob("test-job", "test-runtime"),
			podGroup:               ownedPodGroup("test-job"),
			withSnapshot:           true,
			invalidSnapshot:        true,
			wantObjs:               nil,
		},
		"runtime whose Info cannot be built returns no objects and no error": {
			clusterTrainingRuntime: runtimeWithBrokenTorch("test-runtime"),
			trainJob:               failedTrainJob("test-job", "test-runtime"),
			podGroup:               ownedPodGroup("test-job"),
			wantObjs:               nil,
		},
		"snapshot without the live runtime still returns the PodGroup": {
			clusterTrainingRuntime: runtimeWithVolcano("test-runtime"),
			trainJob:               failedTrainJob("test-job", "test-runtime"),
			podGroup:               ownedPodGroup("test-job"),
			withSnapshot:           true,
			omitLiveRuntime:        true,
			wantObjs:               []client.Object{ownedPodGroup("test-job")},
		},
		"failed read of the snapshot is returned so the reconcile is retried": {
			clusterTrainingRuntime: runtimeWithVolcano("test-runtime"),
			trainJob:               failedTrainJob("test-job", "test-runtime"),
			podGroup:               ownedPodGroup("test-job"),
			snapshotGetErr:         errSnapshotGet,
			wantErr:                errSnapshotGet,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			ctx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)

			clientBuilder := testingutil.NewClientBuilder()
			if tc.clusterTrainingRuntime != nil && !tc.omitLiveRuntime {
				clientBuilder.WithObjects(tc.clusterTrainingRuntime)
			}
			if tc.podGroup != nil {
				clientBuilder.WithObjects(tc.podGroup)
			}
			if tc.snapshotGetErr != nil {
				clientBuilder.WithInterceptorFuncs(interceptor.Funcs{
					Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						if _, ok := obj.(*corev1.ConfigMap); ok {
							return tc.snapshotGetErr
						}
						return c.Get(ctx, key, obj, opts...)
					},
				})
			}
			c := clientBuilder.Build()

			if tc.withSnapshot && tc.clusterTrainingRuntime != nil {
				snapshotRuntime := tc.clusterTrainingRuntime.DeepCopy()
				if tc.invalidSnapshot {
					otherTrainJob := tc.trainJob.DeepCopy()
					otherTrainJob.UID = "some-other-uid"
					if err := createRuntimeSnapshot(ctx, c, otherTrainJob, snapshotRuntime); err != nil {
						t.Fatalf("Failed to seed invalid runtime snapshot: %v", err)
					}
				} else {
					if err := createRuntimeSnapshot(ctx, c, tc.trainJob, snapshotRuntime); err != nil {
						t.Fatalf("Failed to seed runtime snapshot: %v", err)
					}
				}
			}

			trainingRuntime, err := NewTrainingRuntime(ctx, c, testingutil.AsIndex(clientBuilder), nil)
			if err != nil {
				t.Fatal(err)
			}
			var ok bool
			trainingRuntimeFactory, ok = trainingRuntime.(*TrainingRuntime)
			if !ok {
				t.Fatal("Failed type assertion from Runtime interface to TrainingRuntime")
			}

			clTrainingRuntime, err := NewClusterTrainingRuntime(ctx, c, testingutil.AsIndex(clientBuilder), nil)
			if err != nil {
				t.Fatal(err)
			}

			gotObjs, err := clTrainingRuntime.DeleteObjects(ctx, tc.trainJob)
			if diff := cmp.Diff(tc.wantErr, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error (-want,+got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantObjs, gotObjs, cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion")); len(diff) != 0 {
				t.Errorf("Unexpected objects (-want,+got):\n%s", diff)
			}

			// Unlike NewObjects, DeleteObjects must never create a runtime snapshot.
			if !tc.withSnapshot && tc.snapshotGetErr == nil {
				err := c.Get(ctx, client.ObjectKey{
					Name:      tc.trainJob.Name + runtimeSnapshotSuffix,
					Namespace: tc.trainJob.Namespace,
				}, &corev1.ConfigMap{})
				if !apierrors.IsNotFound(err) {
					t.Errorf("Expected no runtime snapshot to be created, got error: %v", err)
				}
			}
		})
	}
}
