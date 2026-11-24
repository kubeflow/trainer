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

package workloadawarescheduling

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	schedulingv1alpha3 "k8s.io/api/scheduling/v1alpha3"
	schedulingv1beta1 "k8s.io/api/scheduling/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	batchv1ac "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	jobsetv1alpha2ac "sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestNew(t *testing.T) {
	// newRESTMapper returns a RESTMapper that serves only the given scheduling.k8s.io/v1beta1 kinds.
	newRESTMapper := func(kinds ...string) meta.RESTMapper {
		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{schedulingv1beta1.SchemeGroupVersion})
		for _, kind := range kinds {
			mapper.Add(schedulingv1beta1.SchemeGroupVersion.WithKind(kind), meta.RESTScopeNamespace)
		}
		return mapper
	}
	cases := map[string]struct {
		restMapper meta.RESTMapper
		wantError  bool
	}{
		"Workload and PodGroup APIs are served": {
			restMapper: newRESTMapper("Workload", "PodGroup"),
		},
		"Workload API is not served": {
			restMapper: newRESTMapper("PodGroup"),
			wantError:  true,
		},
		"PodGroup API is not served": {
			restMapper: newRESTMapper("Workload"),
			wantError:  true,
		},
		"scheduling.k8s.io/v1beta1 API is not served": {
			restMapper: newRESTMapper(),
			wantError:  true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			cli := utiltesting.NewClientBuilder().WithRESTMapper(tc.restMapper).Build()
			_, err := New(ctx, cli, nil, nil)
			if (err != nil) != tc.wantError {
				t.Errorf("New() error = %v, wantError %v", err, tc.wantError)
			}
		})
	}
}

func TestWorkload(t *testing.T) {
	cases := map[string]struct {
		trainJob         *trainer.TrainJob
		info             *runtime.Info
		objs             []client.Object
		expectInfo       *runtime.Info
		expectObjs       []apiruntime.Object
		expectBuildError error
	}{
		"no action when info is nil": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").UID("uid").Obj(),
		},
		"no action when trainJob is nil": {
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{Scheduling: &trainer.TrainingRuntimeScheduling{}},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{Name: constants.Node, Count: ptr.To[int32](1)}},
				},
			},
			expectInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{Scheduling: &trainer.TrainingRuntimeScheduling{}},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{Name: constants.Node, Count: ptr.To[int32](1)}},
				},
			},
		},
		"no action when the runtime does not set scheduling": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").UID("uid").Obj(),
			info: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{Name: constants.Node, Count: ptr.To[int32](1)}},
				},
			},
			expectInfo: &runtime.Info{
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{Name: constants.Node, Count: ptr.To[int32](1)}},
				},
			},
		},
		"no Workload is built when the runtime has no ReplicatedJobs": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").UID("uid").Obj(),
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{Scheduling: &trainer.TrainingRuntimeScheduling{}},
			},
			expectInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{Scheduling: &trainer.TrainingRuntimeScheduling{}},
			},
		},
		"no Workload and PodGroups are built when the TrainJob is suspended": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").UID("uid").Suspend(true).Obj(),
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{Scheduling: &trainer.TrainingRuntimeScheduling{}},
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: jobsetv1alpha2ac.JobSetSpec().WithReplicatedJobs(
						jobsetv1alpha2ac.ReplicatedJob().WithName(constants.Node).WithReplicas(1).
							WithTemplate(batchv1ac.JobTemplateSpec().WithSpec(batchv1ac.JobSpec().WithTemplate(corev1ac.PodTemplateSpec().WithSpec(corev1ac.PodSpec())))),
					),
					PodSets: []runtime.PodSet{{Name: constants.Node, Count: ptr.To[int32](2)}},
				},
			},
			expectInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{Scheduling: &trainer.TrainingRuntimeScheduling{}},
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: jobsetv1alpha2ac.JobSetSpec().WithReplicatedJobs(
						jobsetv1alpha2ac.ReplicatedJob().WithName(constants.Node).WithReplicas(1).
							WithTemplate(batchv1ac.JobTemplateSpec().WithSpec(batchv1ac.JobSpec().WithTemplate(corev1ac.PodTemplateSpec().WithSpec(corev1ac.PodSpec())))),
					),
					PodSets: []runtime.PodSet{{
						Name:            constants.Node,
						Count:           ptr.To[int32](2),
						SchedulingGroup: corev1ac.PodSchedulingGroup().WithPodGroupName("test-job-e2577dd78b-b6ff97ef4e"),
					}},
				},
			},
		},
		"TrainJob-level gang scheduling builds one Workload and PodGroup for every Pod": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").UID("uid").Obj(),
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{Scheduling: &trainer.TrainingRuntimeScheduling{}},
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: jobsetv1alpha2ac.JobSetSpec().WithReplicatedJobs(
						jobsetv1alpha2ac.ReplicatedJob().WithName("sidecar").WithReplicas(2).
							WithTemplate(batchv1ac.JobTemplateSpec().WithSpec(batchv1ac.JobSpec().WithTemplate(corev1ac.PodTemplateSpec().WithSpec(corev1ac.PodSpec())))),
						jobsetv1alpha2ac.ReplicatedJob().WithName(constants.Node).WithReplicas(1).
							WithTemplate(batchv1ac.JobTemplateSpec().WithSpec(batchv1ac.JobSpec().WithTemplate(corev1ac.PodTemplateSpec().WithSpec(corev1ac.PodSpec())))),
					),
					PodSets: []runtime.PodSet{
						{Name: "sidecar", Count: ptr.To[int32](1)},
						{Name: constants.Node, Count: ptr.To[int32](2)},
					},
				},
			},
			expectInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{Scheduling: &trainer.TrainingRuntimeScheduling{}},
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: jobsetv1alpha2ac.JobSetSpec().WithReplicatedJobs(
						jobsetv1alpha2ac.ReplicatedJob().WithName("sidecar").WithReplicas(2).
							WithTemplate(batchv1ac.JobTemplateSpec().WithSpec(batchv1ac.JobSpec().WithTemplate(corev1ac.PodTemplateSpec().WithSpec(corev1ac.PodSpec())))),
						jobsetv1alpha2ac.ReplicatedJob().WithName(constants.Node).WithReplicas(1).
							WithTemplate(batchv1ac.JobTemplateSpec().WithSpec(batchv1ac.JobSpec().WithTemplate(corev1ac.PodTemplateSpec().WithSpec(corev1ac.PodSpec())))),
					),
					PodSets: []runtime.PodSet{
						{
							Name:            "sidecar",
							Count:           ptr.To[int32](1),
							SchedulingGroup: corev1ac.PodSchedulingGroup().WithPodGroupName("test-job-e2577dd78b-b6ff97ef4e"),
						},
						{
							Name:            constants.Node,
							Count:           ptr.To[int32](2),
							SchedulingGroup: corev1ac.PodSchedulingGroup().WithPodGroupName("test-job-e2577dd78b-b6ff97ef4e"),
						},
					},
				},
			},
			expectObjs: []apiruntime.Object{
				&schedulingv1beta1.Workload{
					TypeMeta: metav1.TypeMeta{APIVersion: schedulingv1beta1.SchemeGroupVersion.String(), Kind: "Workload"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
					Spec: schedulingv1beta1.WorkloadSpec{
						ControllerRef: &schedulingv1beta1.TypedLocalObjectReference{
							APIGroup: trainer.GroupVersion.Group,
							Kind:     trainer.TrainJobKind,
							Name:     "test-job",
						},
						PodGroupTemplates: []schedulingv1beta1.PodGroupTemplate{
							{Name: "test-job", SchedulingPolicy: schedulingv1beta1.PodGroupSchedulingPolicy{Gang: &schedulingv1beta1.GangSchedulingPolicy{MinCount: 4}}},
						},
					},
				},
				&schedulingv1beta1.PodGroup{
					TypeMeta: metav1.TypeMeta{APIVersion: schedulingv1beta1.SchemeGroupVersion.String(), Kind: "PodGroup"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b-b6ff97ef4e",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
					Spec: schedulingv1beta1.PodGroupSpec{
						WorkloadRef:      &schedulingv1beta1.WorkloadReference{WorkloadName: "test-job-e2577dd78b", TemplateName: "test-job"},
						SchedulingPolicy: schedulingv1beta1.PodGroupSchedulingPolicy{Gang: &schedulingv1beta1.GangSchedulingPolicy{MinCount: 4}},
					},
				},
			},
		},
		"per-ReplicatedJob scheduling builds one PodGroup for every entry": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").UID("uid").Obj(),
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					Scheduling: &trainer.TrainingRuntimeScheduling{
						ReplicatedJobs: []trainer.ReplicatedJobScheduling{
							{
								TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer},
								SchedulingPolicy: &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingPolicy{
									Basic: &schedulingv1alpha3.WorkloadCompositePodGroupBasicSchedulingPolicy{},
								},
							},
							{
								TargetReplicatedJobs: []string{constants.Node},
								SchedulingConstraints: &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingConstraints{
									Topology: []schedulingv1alpha3.TopologyConstraint{{Key: "topology.kubernetes.io/rack"}},
								},
							},
						},
					},
				},
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: jobsetv1alpha2ac.JobSetSpec().WithReplicatedJobs(
						jobsetv1alpha2ac.ReplicatedJob().WithName(constants.DatasetInitializer).WithReplicas(1).
							WithTemplate(batchv1ac.JobTemplateSpec().WithSpec(batchv1ac.JobSpec().WithTemplate(corev1ac.PodTemplateSpec().WithSpec(corev1ac.PodSpec())))),
						jobsetv1alpha2ac.ReplicatedJob().WithName(constants.ModelInitializer).WithReplicas(1).
							WithTemplate(batchv1ac.JobTemplateSpec().WithSpec(batchv1ac.JobSpec().WithTemplate(corev1ac.PodTemplateSpec().WithSpec(corev1ac.PodSpec())))),
						jobsetv1alpha2ac.ReplicatedJob().WithName(constants.Node).WithReplicas(1).
							WithDependsOn(jobsetv1alpha2ac.DependsOn().WithName(constants.DatasetInitializer).WithStatus(jobsetv1alpha2.DependencyComplete)).
							WithTemplate(batchv1ac.JobTemplateSpec().WithSpec(batchv1ac.JobSpec().
								WithTemplate(corev1ac.PodTemplateSpec().WithSpec(corev1ac.PodSpec().WithPriorityClassName("high-priority"))))),
					),
					PodSets: []runtime.PodSet{
						{Name: constants.DatasetInitializer, Count: ptr.To[int32](1)},
						{Name: constants.ModelInitializer, Count: ptr.To[int32](1)},
						{Name: constants.Node, Count: ptr.To[int32](4)},
					},
				},
			},
			expectInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					Scheduling: &trainer.TrainingRuntimeScheduling{
						ReplicatedJobs: []trainer.ReplicatedJobScheduling{
							{
								TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer},
								SchedulingPolicy: &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingPolicy{
									Basic: &schedulingv1alpha3.WorkloadCompositePodGroupBasicSchedulingPolicy{},
								},
							},
							{
								TargetReplicatedJobs: []string{constants.Node},
								SchedulingConstraints: &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingConstraints{
									Topology: []schedulingv1alpha3.TopologyConstraint{{Key: "topology.kubernetes.io/rack"}},
								},
							},
						},
					},
				},
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: jobsetv1alpha2ac.JobSetSpec().WithReplicatedJobs(
						jobsetv1alpha2ac.ReplicatedJob().WithName(constants.DatasetInitializer).WithReplicas(1).
							WithTemplate(batchv1ac.JobTemplateSpec().WithSpec(batchv1ac.JobSpec().WithTemplate(corev1ac.PodTemplateSpec().WithSpec(corev1ac.PodSpec())))),
						jobsetv1alpha2ac.ReplicatedJob().WithName(constants.ModelInitializer).WithReplicas(1).
							WithTemplate(batchv1ac.JobTemplateSpec().WithSpec(batchv1ac.JobSpec().WithTemplate(corev1ac.PodTemplateSpec().WithSpec(corev1ac.PodSpec())))),
						jobsetv1alpha2ac.ReplicatedJob().WithName(constants.Node).WithReplicas(1).
							WithDependsOn(jobsetv1alpha2ac.DependsOn().WithName(constants.DatasetInitializer).WithStatus(jobsetv1alpha2.DependencyComplete)).
							WithTemplate(batchv1ac.JobTemplateSpec().WithSpec(batchv1ac.JobSpec().
								WithTemplate(corev1ac.PodTemplateSpec().WithSpec(corev1ac.PodSpec().WithPriorityClassName("high-priority"))))),
					),
					PodSets: []runtime.PodSet{
						{
							Name:            constants.DatasetInitializer,
							Count:           ptr.To[int32](1),
							SchedulingGroup: corev1ac.PodSchedulingGroup().WithPodGroupName("test-job-e2577dd78b-0-dataset-initializer-model-init-8cb2fda4f5"),
						},
						{
							Name:            constants.ModelInitializer,
							Count:           ptr.To[int32](1),
							SchedulingGroup: corev1ac.PodSchedulingGroup().WithPodGroupName("test-job-e2577dd78b-0-dataset-initializer-model-init-8cb2fda4f5"),
						},
						{
							Name:            constants.Node,
							Count:           ptr.To[int32](4),
							SchedulingGroup: corev1ac.PodSchedulingGroup().WithPodGroupName("test-job-e2577dd78b-1-node-cc90ebe740"),
						},
					},
				},
			},
			expectObjs: []apiruntime.Object{
				&schedulingv1beta1.Workload{
					TypeMeta: metav1.TypeMeta{APIVersion: schedulingv1beta1.SchemeGroupVersion.String(), Kind: "Workload"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
					Spec: schedulingv1beta1.WorkloadSpec{
						ControllerRef: &schedulingv1beta1.TypedLocalObjectReference{
							APIGroup: trainer.GroupVersion.Group,
							Kind:     trainer.TrainJobKind,
							Name:     "test-job",
						},
						PodGroupTemplates: []schedulingv1beta1.PodGroupTemplate{
							{Name: "0-dataset-initializer-model-initializer", SchedulingPolicy: schedulingv1beta1.PodGroupSchedulingPolicy{Basic: &schedulingv1beta1.BasicSchedulingPolicy{}}},
							{
								Name:             "1-node",
								SchedulingPolicy: schedulingv1beta1.PodGroupSchedulingPolicy{Gang: &schedulingv1beta1.GangSchedulingPolicy{MinCount: 4}},
								SchedulingConstraints: &schedulingv1beta1.PodGroupSchedulingConstraints{
									Topology: []schedulingv1beta1.TopologyConstraint{{Key: "topology.kubernetes.io/rack"}},
								},
								PriorityClassName: "high-priority",
							},
						},
					},
				},
				&schedulingv1beta1.PodGroup{
					TypeMeta: metav1.TypeMeta{APIVersion: schedulingv1beta1.SchemeGroupVersion.String(), Kind: "PodGroup"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b-0-dataset-initializer-model-init-8cb2fda4f5",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
					Spec: schedulingv1beta1.PodGroupSpec{
						WorkloadRef:      &schedulingv1beta1.WorkloadReference{WorkloadName: "test-job-e2577dd78b", TemplateName: "0-dataset-initializer-model-initializer"},
						SchedulingPolicy: schedulingv1beta1.PodGroupSchedulingPolicy{Basic: &schedulingv1beta1.BasicSchedulingPolicy{}},
					},
				},
				&schedulingv1beta1.PodGroup{
					TypeMeta: metav1.TypeMeta{APIVersion: schedulingv1beta1.SchemeGroupVersion.String(), Kind: "PodGroup"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b-1-node-cc90ebe740",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
					Spec: schedulingv1beta1.PodGroupSpec{
						WorkloadRef:      &schedulingv1beta1.WorkloadReference{WorkloadName: "test-job-e2577dd78b", TemplateName: "1-node"},
						SchedulingPolicy: schedulingv1beta1.PodGroupSchedulingPolicy{Gang: &schedulingv1beta1.GangSchedulingPolicy{MinCount: 4}},
						SchedulingConstraints: &schedulingv1beta1.PodGroupSchedulingConstraints{
							Topology: []schedulingv1beta1.TopologyConstraint{{Key: "topology.kubernetes.io/rack"}},
						},
						PriorityClassName: "high-priority",
					},
				},
			},
		},
		"ReplicatedJob depending on another target of the same entry is excluded from minCount": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").UID("uid").Obj(),
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					Scheduling: &trainer.TrainingRuntimeScheduling{
						ReplicatedJobs: []trainer.ReplicatedJobScheduling{
							{TargetReplicatedJobs: []string{constants.Launcher, constants.Node}},
						},
					},
				},
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: jobsetv1alpha2ac.JobSetSpec().WithReplicatedJobs(
						jobsetv1alpha2ac.ReplicatedJob().WithName(constants.Node).WithReplicas(1).
							WithTemplate(batchv1ac.JobTemplateSpec().WithSpec(batchv1ac.JobSpec().WithTemplate(corev1ac.PodTemplateSpec().WithSpec(corev1ac.PodSpec())))),
						jobsetv1alpha2ac.ReplicatedJob().WithName(constants.Launcher).WithReplicas(1).
							WithDependsOn(jobsetv1alpha2ac.DependsOn().WithName(constants.Node).WithStatus(jobsetv1alpha2.DependencyReady)).
							WithTemplate(batchv1ac.JobTemplateSpec().WithSpec(batchv1ac.JobSpec().WithTemplate(corev1ac.PodTemplateSpec().WithSpec(corev1ac.PodSpec())))),
					),
					PodSets: []runtime.PodSet{
						{Name: constants.Node, Count: ptr.To[int32](3)},
						{Name: constants.Launcher, Count: ptr.To[int32](1)},
					},
				},
			},
			expectInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					Scheduling: &trainer.TrainingRuntimeScheduling{
						ReplicatedJobs: []trainer.ReplicatedJobScheduling{
							{TargetReplicatedJobs: []string{constants.Launcher, constants.Node}},
						},
					},
				},
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: jobsetv1alpha2ac.JobSetSpec().WithReplicatedJobs(
						jobsetv1alpha2ac.ReplicatedJob().WithName(constants.Node).WithReplicas(1).
							WithTemplate(batchv1ac.JobTemplateSpec().WithSpec(batchv1ac.JobSpec().WithTemplate(corev1ac.PodTemplateSpec().WithSpec(corev1ac.PodSpec())))),
						jobsetv1alpha2ac.ReplicatedJob().WithName(constants.Launcher).WithReplicas(1).
							WithDependsOn(jobsetv1alpha2ac.DependsOn().WithName(constants.Node).WithStatus(jobsetv1alpha2.DependencyReady)).
							WithTemplate(batchv1ac.JobTemplateSpec().WithSpec(batchv1ac.JobSpec().WithTemplate(corev1ac.PodTemplateSpec().WithSpec(corev1ac.PodSpec())))),
					),
					PodSets: []runtime.PodSet{
						{
							Name:            constants.Node,
							Count:           ptr.To[int32](3),
							SchedulingGroup: corev1ac.PodSchedulingGroup().WithPodGroupName("test-job-e2577dd78b-0-launcher-node-e4613efea6"),
						},
						{
							Name:            constants.Launcher,
							Count:           ptr.To[int32](1),
							SchedulingGroup: corev1ac.PodSchedulingGroup().WithPodGroupName("test-job-e2577dd78b-0-launcher-node-e4613efea6"),
						},
					},
				},
			},
			expectObjs: []apiruntime.Object{
				&schedulingv1beta1.Workload{
					TypeMeta: metav1.TypeMeta{APIVersion: schedulingv1beta1.SchemeGroupVersion.String(), Kind: "Workload"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
					Spec: schedulingv1beta1.WorkloadSpec{
						ControllerRef: &schedulingv1beta1.TypedLocalObjectReference{
							APIGroup: trainer.GroupVersion.Group,
							Kind:     trainer.TrainJobKind,
							Name:     "test-job",
						},
						PodGroupTemplates: []schedulingv1beta1.PodGroupTemplate{
							{Name: "0-launcher-node", SchedulingPolicy: schedulingv1beta1.PodGroupSchedulingPolicy{Gang: &schedulingv1beta1.GangSchedulingPolicy{MinCount: 3}}},
						},
					},
				},
				&schedulingv1beta1.PodGroup{
					TypeMeta: metav1.TypeMeta{APIVersion: schedulingv1beta1.SchemeGroupVersion.String(), Kind: "PodGroup"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b-0-launcher-node-e4613efea6",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
					Spec: schedulingv1beta1.PodGroupSpec{
						WorkloadRef:      &schedulingv1beta1.WorkloadReference{WorkloadName: "test-job-e2577dd78b", TemplateName: "0-launcher-node"},
						SchedulingPolicy: schedulingv1beta1.PodGroupSchedulingPolicy{Gang: &schedulingv1beta1.GangSchedulingPolicy{MinCount: 3}},
					},
				},
			},
		},
		"Job-level scheduling builds the PodGroup of the Job": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").UID("uid").Obj(),
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					Scheduling: &trainer.TrainingRuntimeScheduling{
						ReplicatedJobs: []trainer.ReplicatedJobScheduling{
							{
								TargetReplicatedJobs: []string{constants.Node},
								Job: &trainer.JobScheduling{
									DisruptionMode: &schedulingv1alpha3.WorkloadPodGroupDisruptionMode{
										All: &schedulingv1alpha3.WorkloadPodGroupAllDisruptionMode{},
									},
									ResourceClaims: []schedulingv1alpha3.WorkloadPodGroupResourceClaim{{
										Name:                      "tpu-slice",
										ResourceClaimTemplateName: ptr.To("tpu-slice-template"),
									}},
								},
							},
						},
					},
				},
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: jobsetv1alpha2ac.JobSetSpec().WithReplicatedJobs(
						jobsetv1alpha2ac.ReplicatedJob().WithName(constants.Node).WithReplicas(1).
							WithTemplate(batchv1ac.JobTemplateSpec().WithSpec(batchv1ac.JobSpec().WithTemplate(corev1ac.PodTemplateSpec().WithSpec(corev1ac.PodSpec())))),
					),
					PodSets: []runtime.PodSet{{Name: constants.Node, Count: ptr.To[int32](4)}},
				},
			},
			expectInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					Scheduling: &trainer.TrainingRuntimeScheduling{
						ReplicatedJobs: []trainer.ReplicatedJobScheduling{
							{
								TargetReplicatedJobs: []string{constants.Node},
								Job: &trainer.JobScheduling{
									DisruptionMode: &schedulingv1alpha3.WorkloadPodGroupDisruptionMode{
										All: &schedulingv1alpha3.WorkloadPodGroupAllDisruptionMode{},
									},
									ResourceClaims: []schedulingv1alpha3.WorkloadPodGroupResourceClaim{{
										Name:                      "tpu-slice",
										ResourceClaimTemplateName: ptr.To("tpu-slice-template"),
									}},
								},
							},
						},
					},
				},
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: jobsetv1alpha2ac.JobSetSpec().WithReplicatedJobs(
						jobsetv1alpha2ac.ReplicatedJob().WithName(constants.Node).WithReplicas(1).
							WithTemplate(batchv1ac.JobTemplateSpec().WithSpec(batchv1ac.JobSpec().WithTemplate(corev1ac.PodTemplateSpec().WithSpec(corev1ac.PodSpec())))),
					),
					PodSets: []runtime.PodSet{
						{
							Name:            constants.Node,
							Count:           ptr.To[int32](4),
							SchedulingGroup: corev1ac.PodSchedulingGroup().WithPodGroupName("test-job-e2577dd78b-node-0-baa3c64c89"),
						},
					},
				},
			},
			expectObjs: []apiruntime.Object{
				&schedulingv1beta1.Workload{
					TypeMeta: metav1.TypeMeta{APIVersion: schedulingv1beta1.SchemeGroupVersion.String(), Kind: "Workload"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
					Spec: schedulingv1beta1.WorkloadSpec{
						ControllerRef: &schedulingv1beta1.TypedLocalObjectReference{
							APIGroup: trainer.GroupVersion.Group,
							Kind:     trainer.TrainJobKind,
							Name:     "test-job",
						},
						PodGroupTemplates: []schedulingv1beta1.PodGroupTemplate{
							{
								Name:             "node-0",
								SchedulingPolicy: schedulingv1beta1.PodGroupSchedulingPolicy{Gang: &schedulingv1beta1.GangSchedulingPolicy{MinCount: 4}},
								DisruptionMode:   &schedulingv1beta1.DisruptionMode{All: &schedulingv1beta1.AllDisruptionMode{}},
								ResourceClaims: []schedulingv1beta1.PodGroupResourceClaim{{
									Name:                      "tpu-slice",
									ResourceClaimTemplateName: ptr.To("tpu-slice-template"),
								}},
							},
						},
					},
				},
				&schedulingv1beta1.PodGroup{
					TypeMeta: metav1.TypeMeta{APIVersion: schedulingv1beta1.SchemeGroupVersion.String(), Kind: "PodGroup"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b-node-0-baa3c64c89",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
					Spec: schedulingv1beta1.PodGroupSpec{
						WorkloadRef:      &schedulingv1beta1.WorkloadReference{WorkloadName: "test-job-e2577dd78b", TemplateName: "node-0"},
						SchedulingPolicy: schedulingv1beta1.PodGroupSchedulingPolicy{Gang: &schedulingv1beta1.GangSchedulingPolicy{MinCount: 4}},
						DisruptionMode:   &schedulingv1beta1.DisruptionMode{All: &schedulingv1beta1.AllDisruptionMode{}},
						ResourceClaims: []schedulingv1beta1.PodGroupResourceClaim{{
							Name:                      "tpu-slice",
							ResourceClaimTemplateName: ptr.To("tpu-slice-template"),
						}},
					},
				},
			},
		},
		"existing Workload and PodGroup are not updated": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").UID("uid").Obj(),
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{Scheduling: &trainer.TrainingRuntimeScheduling{}},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{Name: constants.Node, Count: ptr.To[int32](2)}},
				},
			},
			objs: []client.Object{
				&schedulingv1beta1.Workload{ObjectMeta: metav1.ObjectMeta{Name: "test-job-e2577dd78b", Namespace: metav1.NamespaceDefault}},
				&schedulingv1beta1.PodGroup{ObjectMeta: metav1.ObjectMeta{Name: "test-job-e2577dd78b-b6ff97ef4e", Namespace: metav1.NamespaceDefault}},
			},
			expectInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{Scheduling: &trainer.TrainingRuntimeScheduling{}},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:            constants.Node,
							Count:           ptr.To[int32](2),
							SchedulingGroup: corev1ac.PodSchedulingGroup().WithPodGroupName("test-job-e2577dd78b-b6ff97ef4e"),
						},
					},
				},
			},
		},
		"missing PodGroup is built when the Workload exists": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").UID("uid").Obj(),
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{Scheduling: &trainer.TrainingRuntimeScheduling{}},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{Name: constants.Node, Count: ptr.To[int32](2)}},
				},
			},
			objs: []client.Object{
				&schedulingv1beta1.Workload{ObjectMeta: metav1.ObjectMeta{Name: "test-job-e2577dd78b", Namespace: metav1.NamespaceDefault}},
			},
			expectInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{Scheduling: &trainer.TrainingRuntimeScheduling{}},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:            constants.Node,
							Count:           ptr.To[int32](2),
							SchedulingGroup: corev1ac.PodSchedulingGroup().WithPodGroupName("test-job-e2577dd78b-b6ff97ef4e"),
						},
					},
				},
			},
			expectObjs: []apiruntime.Object{
				&schedulingv1beta1.PodGroup{
					TypeMeta: metav1.TypeMeta{APIVersion: schedulingv1beta1.SchemeGroupVersion.String(), Kind: "PodGroup"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b-b6ff97ef4e",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
					Spec: schedulingv1beta1.PodGroupSpec{
						WorkloadRef:      &schedulingv1beta1.WorkloadReference{WorkloadName: "test-job-e2577dd78b", TemplateName: "test-job"},
						SchedulingPolicy: schedulingv1beta1.PodGroupSchedulingPolicy{Gang: &schedulingv1beta1.GangSchedulingPolicy{MinCount: 2}},
					},
				},
			},
		},
		"error when getting existing Workload": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").UID("uid").Obj(),
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{Scheduling: &trainer.TrainingRuntimeScheduling{}},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{{Name: constants.Node, Count: ptr.To[int32](2)}},
				},
			},
			expectInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{Scheduling: &trainer.TrainingRuntimeScheduling{}},
				TemplateSpec: runtime.TemplateSpec{
					PodSets: []runtime.PodSet{
						{
							Name:            constants.Node,
							Count:           ptr.To[int32](2),
							SchedulingGroup: corev1ac.PodSchedulingGroup().WithPodGroupName("test-job-e2577dd78b-b6ff97ef4e"),
						},
					},
				},
			},
			expectBuildError: errors.New("error when getting existing Workload"),
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			ctx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)

			clientBuilder := utiltesting.WithWorkloadRESTMapping(utiltesting.NewClientBuilder()).WithObjects(c.objs...)
			if c.expectBuildError != nil {
				clientBuilder.WithInterceptorFuncs(interceptor.Funcs{
					Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
						return c.expectBuildError
					},
				})
			}
			cli := clientBuilder.Build()
			plugin, err := New(ctx, cli, utiltesting.AsIndex(clientBuilder), nil)
			if err != nil {
				t.Fatalf("Failed to initialize WorkloadAwareScheduling plugin: %v", err)
			}

			// Test EnforceScheduling
			if err := plugin.(framework.EnforceSchedulingPlugin).EnforceScheduling(c.info, c.trainJob); err != nil {
				t.Fatalf("Unexpected error from EnforceScheduling: %v", err)
			}
			if diff := cmp.Diff(c.expectInfo, c.info); len(diff) != 0 {
				t.Errorf("Unexpected info from EnforceScheduling (-want, +got): %s", diff)
			}

			// Test Build
			objs, err := plugin.(framework.ComponentBuilderPlugin).Build(ctx, c.info, c.trainJob)
			if diff := cmp.Diff(c.expectBuildError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error from Build (-want, +got): %s", diff)
			}
			typedObjs, err := utiltesting.ToObject(cli.Scheme(), objs...)
			if err != nil {
				t.Fatalf("Failed to convert objects: %v", err)
			}
			if diff := cmp.Diff(c.expectObjs, typedObjs); len(diff) != 0 {
				t.Errorf("Unexpected objects from Build (-want, +got): %s", diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	errorListPodGroups := errors.New("error when listing PodGroups")
	cases := map[string]struct {
		trainJob  *trainer.TrainJob
		objs      []client.Object
		listError error
		wantObjs  []client.Object
		wantError error
	}{
		"no objects are deleted when trainJob is nil": {
			objs: []client.Object{
				&schedulingv1beta1.Workload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
				},
			},
		},
		"no objects are deleted when the TrainJob is running": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").UID("uid").Obj(),
			objs: []client.Object{
				&schedulingv1beta1.Workload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
				},
			},
		},
		"Workload and PodGroups are deleted when the TrainJob is complete": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				Conditions(metav1.Condition{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue}).
				Obj(),
			objs: []client.Object{
				&schedulingv1beta1.Workload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
				},
				&schedulingv1beta1.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b-b6ff97ef4e",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
				},
			},
			wantObjs: []client.Object{
				&schedulingv1beta1.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b-b6ff97ef4e",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
				},
				&schedulingv1beta1.Workload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
				},
			},
		},
		"Workload and PodGroups are deleted when the TrainJob is failed": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				Conditions(metav1.Condition{Type: trainer.TrainJobFailed, Status: metav1.ConditionTrue}).
				Obj(),
			objs: []client.Object{
				&schedulingv1beta1.Workload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
				},
				&schedulingv1beta1.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b-0-node-b408b826df",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
				},
				&schedulingv1beta1.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b-1-launcher-78bf98d2d9",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
				},
			},
			wantObjs: []client.Object{
				&schedulingv1beta1.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b-0-node-b408b826df",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
				},
				&schedulingv1beta1.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b-1-launcher-78bf98d2d9",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
				},
				&schedulingv1beta1.Workload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
				},
			},
		},
		"Workload is deleted when the TrainJob is suspended": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").UID("uid").Suspend(true).Obj(),
			objs: []client.Object{
				&schedulingv1beta1.Workload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
				},
			},
			wantObjs: []client.Object{
				&schedulingv1beta1.Workload{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job-e2577dd78b",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "test-job",
							UID:                "uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
				},
			},
		},
		"no objects are deleted when they are already deleted": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").UID("uid").Suspend(true).Obj(),
		},
		"Workload and PodGroups not controlled by the TrainJob are not deleted": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").UID("uid").Suspend(true).Obj(),
			objs: []client.Object{
				&schedulingv1beta1.Workload{
					ObjectMeta: metav1.ObjectMeta{Name: "test-job-e2577dd78b", Namespace: metav1.NamespaceDefault},
				},
				&schedulingv1beta1.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-job-a1b2c3d4e5-b6ff97ef4e",
						Namespace: metav1.NamespaceDefault,
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion:         trainer.GroupVersion.String(),
							Kind:               trainer.TrainJobKind,
							Name:               "other-job",
							UID:                "other-uid",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						}},
					},
				},
			},
		},
		"error from listing PodGroups is propagated": {
			trainJob:  utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").UID("uid").Suspend(true).Obj(),
			listError: errorListPodGroups,
			wantError: errorListPodGroups,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			ctx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)

			clientBuilder := utiltesting.WithWorkloadRESTMapping(utiltesting.NewClientBuilder()).WithObjects(c.objs...)
			if c.listError != nil {
				clientBuilder.WithInterceptorFuncs(interceptor.Funcs{
					List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
						return c.listError
					},
				})
			}
			cli := clientBuilder.Build()
			plugin, err := New(ctx, cli, utiltesting.AsIndex(clientBuilder), nil)
			if err != nil {
				t.Fatalf("Failed to initialize WorkloadAwareScheduling plugin: %v", err)
			}

			objs, err := plugin.(framework.ComponentDeleterPlugin).Delete(ctx, c.trainJob)
			if diff := cmp.Diff(c.wantError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error from Delete (-want, +got): %s", diff)
			}
			if diff := cmp.Diff(c.wantObjs, objs, cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion")); len(diff) != 0 {
				t.Errorf("Unexpected objects from Delete (-want, +got): %s", diff)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	jobClaimsScheduling := &trainer.TrainingRuntimeScheduling{
		ReplicatedJobs: []trainer.ReplicatedJobScheduling{{
			TargetReplicatedJobs: []string{constants.Node},
			Job: &trainer.JobScheduling{
				ResourceClaims: []schedulingv1alpha3.WorkloadPodGroupResourceClaim{{
					Name:                      "tpu-slice",
					ResourceClaimTemplateName: ptr.To("tpu-slice-template"),
				}},
			},
		}},
	}
	jobScheduling := &trainer.TrainingRuntimeScheduling{
		ReplicatedJobs: []trainer.ReplicatedJobScheduling{{
			TargetReplicatedJobs: []string{constants.Node},
			Job:                  &trainer.JobScheduling{},
		}},
	}
	trainerWithClaims := utiltesting.MakeTrainJobTrainerWrapper().
		ResourceClaimsPerNode(trainer.TrainerResourceClaim{Name: "gpu", ResourceClaimTemplateName: "gpu-template"}).
		Obj()
	podClaimsPatch := trainer.RuntimePatch{
		Manager: "test.io/manager",
		TrainingRuntimeSpec: &trainer.TrainingRuntimeSpecPatch{
			Template: &trainer.JobSetTemplatePatch{
				Spec: &trainer.JobSetSpecPatch{
					ReplicatedJobs: []trainer.ReplicatedJobPatch{{
						Name: constants.Node,
						Template: &trainer.JobTemplatePatch{
							Spec: &trainer.JobSpecPatch{
								Template: &trainer.PodTemplatePatch{
									Spec: &trainer.PodSpecPatch{
										ResourceClaims: []corev1.PodResourceClaim{{
											Name:                      "gpu",
											ResourceClaimTemplateName: ptr.To("gpu-template"),
										}},
									},
								},
							},
						},
					}},
				},
			},
		},
	}
	nodeSelectorPatch := trainer.RuntimePatch{
		Manager: "test.io/manager",
		TrainingRuntimeSpec: &trainer.TrainingRuntimeSpecPatch{
			Template: &trainer.JobSetTemplatePatch{
				Spec: &trainer.JobSetSpecPatch{
					ReplicatedJobs: []trainer.ReplicatedJobPatch{{
						Name: constants.Node,
						Template: &trainer.JobTemplatePatch{
							Spec: &trainer.JobSpecPatch{
								Template: &trainer.PodTemplatePatch{
									Spec: &trainer.PodSpecPatch{
										NodeSelector: map[string]string{"accelerator": "tpu"},
									},
								},
							},
						},
					}},
				},
			},
		},
	}
	// newClaimsPatch returns a RuntimePatch from the given manager which sets Pod resourceClaims
	// on the given ReplicatedJob.
	newClaimsPatch := func(manager, rJobName string) trainer.RuntimePatch {
		patch := *podClaimsPatch.DeepCopy()
		patch.Manager = manager
		patch.TrainingRuntimeSpec.Template.Spec.ReplicatedJobs[0].Name = rJobName
		return patch
	}
	podClaimsPathAt := func(patchIdx, rJobIdx int) *field.Path {
		return field.NewPath("spec", "runtimePatches").Index(patchIdx).
			Child("trainingRuntimeSpec", "template", "spec", "replicatedJobs").Index(rJobIdx).
			Child("template", "spec", "template", "spec", "resourceClaims")
	}
	trainerClaimsPath := field.NewPath("spec", "trainer", "resourceClaimsPerNode")
	patchClaimsPath := field.NewPath("spec", "runtimePatches").Index(0).
		Child("trainingRuntimeSpec", "template", "spec", "replicatedJobs").Index(0).
		Child("template", "spec", "template", "spec", "resourceClaims")

	cases := map[string]struct {
		info      *runtime.Info
		oldObj    *trainer.TrainJob
		newObj    *trainer.TrainJob
		wantError field.ErrorList
	}{
		"no action when info is nil": {
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").Obj(),
		},
		"no action when the runtime does not set scheduling": {
			info: runtime.NewInfo(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(trainerWithClaims).
				RuntimePatches([]trainer.RuntimePatch{podClaimsPatch}).
				Obj(),
		},
		"resource claims are allowed when the runtime sets Job-level scheduling without resourceClaims": {
			info: runtime.NewInfo(runtime.WithScheduling(jobScheduling)),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(trainerWithClaims).
				RuntimePatches([]trainer.RuntimePatch{podClaimsPatch}).
				Obj(),
		},
		"runtimePatches without resource claims are allowed when the runtime sets Job-level resourceClaims": {
			info: runtime.NewInfo(runtime.WithScheduling(jobClaimsScheduling)),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimePatches([]trainer.RuntimePatch{nodeSelectorPatch}).
				Obj(),
		},
		"resourceClaimsPerNode is rejected when the runtime sets Job-level resourceClaims": {
			info: runtime.NewInfo(runtime.WithScheduling(jobClaimsScheduling)),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(trainerWithClaims).
				Obj(),
			wantError: field.ErrorList{
				field.Forbidden(trainerClaimsPath, ""),
			},
		},
		"runtimePatches Pod resourceClaims are rejected when the runtime sets Job-level resourceClaims": {
			info: runtime.NewInfo(runtime.WithScheduling(jobClaimsScheduling)),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimePatches([]trainer.RuntimePatch{podClaimsPatch}).
				Obj(),
			wantError: field.ErrorList{
				field.Forbidden(patchClaimsPath, ""),
			},
		},
		"update is allowed when the resource claims were already set on the old TrainJob": {
			info: runtime.NewInfo(runtime.WithScheduling(jobClaimsScheduling)),
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(trainerWithClaims).
				Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(trainerWithClaims).
				Suspend(true).
				Obj(),
		},
		"update adding runtimePatches Pod resourceClaims is rejected when the runtime sets Job-level resourceClaims": {
			info:   runtime.NewInfo(runtime.WithScheduling(jobClaimsScheduling)),
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimePatches([]trainer.RuntimePatch{podClaimsPatch}).
				Obj(),
			wantError: field.ErrorList{
				field.Forbidden(patchClaimsPath, ""),
			},
		},
		"update reordering runtimePatches rejects only the added Pod resourceClaims": {
			info: runtime.NewInfo(runtime.WithScheduling(jobClaimsScheduling)),
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimePatches([]trainer.RuntimePatch{newClaimsPatch("existing.io/manager", constants.Node)}).
				Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimePatches([]trainer.RuntimePatch{
					newClaimsPatch("added.io/manager", constants.Node),
					newClaimsPatch("existing.io/manager", constants.Node),
				}).
				Obj(),
			wantError: field.ErrorList{
				field.Forbidden(podClaimsPathAt(0, 0), ""),
			},
		},
		"update adding Pod resourceClaims to an existing runtimePatch is rejected": {
			info: runtime.NewInfo(runtime.WithScheduling(jobClaimsScheduling)),
			oldObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimePatches([]trainer.RuntimePatch{nodeSelectorPatch}).
				Obj(),
			newObj: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				RuntimePatches([]trainer.RuntimePatch{newClaimsPatch(nodeSelectorPatch.Manager, constants.Node)}).
				Obj(),
			wantError: field.ErrorList{
				field.Forbidden(podClaimsPathAt(0, 0), ""),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p, err := New(context.Background(), utiltesting.WithWorkloadRESTMapping(utiltesting.NewClientBuilder()).Build(), nil, nil)
			if err != nil {
				t.Fatalf("Failed to initialize WorkloadAwareScheduling plugin: %v", err)
			}
			_, gotError := p.(*WorkloadAwareScheduling).Validate(context.Background(), tc.info, tc.oldObj, tc.newObj)
			if diff := cmp.Diff(tc.wantError, gotError, cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); len(diff) != 0 {
				t.Errorf("Unexpected error from Validate (-want, +got): %s", diff)
			}
		})
	}
}

func TestWorkloadName(t *testing.T) {
	cases := map[string]struct {
		trainJob *trainer.TrainJob
		wantName string
	}{
		"TrainJob name is used as the prefix": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").UID("uid").Obj(),
			wantName: "test-job-e2577dd78b",
		},
		"long TrainJob name is truncated": {
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, strings.Repeat("a", 63)).UID("uid").Obj(),
			wantName: strings.Repeat("a", 52) + "-e2577dd78b",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := workloadName(tc.trainJob); got != tc.wantName {
				t.Errorf("Unexpected Workload name: got %q, want %q", got, tc.wantName)
			}
		})
	}
}

func TestPodGroupTemplateName(t *testing.T) {
	trainJob := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").UID("uid").Obj()
	longTargets := []string{strings.Repeat("a", 40), strings.Repeat("b", 40)}
	cases := map[string]struct {
		scheduling *trainer.TrainingRuntimeScheduling
		rJobName   string
		wantName   string
		wantOK     bool
	}{
		"TrainJob-level scheduling uses the TrainJob name": {
			scheduling: &trainer.TrainingRuntimeScheduling{},
			rJobName:   constants.Node,
			wantName:   "test-job",
			wantOK:     true,
		},
		"per-ReplicatedJob scheduling joins the entry index and targets": {
			scheduling: &trainer.TrainingRuntimeScheduling{
				ReplicatedJobs: []trainer.ReplicatedJobScheduling{
					{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer}},
					{TargetReplicatedJobs: []string{constants.Node}},
				},
			},
			rJobName: constants.ModelInitializer,
			wantName: "0-dataset-initializer-model-initializer",
			wantOK:   true,
		},
		"per-ReplicatedJob scheduling with a single target": {
			scheduling: &trainer.TrainingRuntimeScheduling{
				ReplicatedJobs: []trainer.ReplicatedJobScheduling{
					{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer}},
					{TargetReplicatedJobs: []string{constants.Node}},
				},
			},
			rJobName: constants.Node,
			wantName: "1-node",
			wantOK:   true,
		},
		"long per-ReplicatedJob name is truncated": {
			scheduling: &trainer.TrainingRuntimeScheduling{
				ReplicatedJobs: []trainer.ReplicatedJobScheduling{{TargetReplicatedJobs: longTargets}},
			},
			rJobName: longTargets[1],
			wantName: "0-" + strings.Repeat("a", 40) + "-" + strings.Repeat("b", 9) + "-4dced6bb4a",
			wantOK:   true,
		},
		"Job-level scheduling uses the ReplicatedJob name and Job index": {
			scheduling: &trainer.TrainingRuntimeScheduling{
				ReplicatedJobs: []trainer.ReplicatedJobScheduling{
					{TargetReplicatedJobs: []string{constants.Node}, Job: &trainer.JobScheduling{}},
				},
			},
			rJobName: constants.Node,
			wantName: "node-0",
			wantOK:   true,
		},
		"untargeted ReplicatedJob has no PodGroupTemplate": {
			scheduling: &trainer.TrainingRuntimeScheduling{
				ReplicatedJobs: []trainer.ReplicatedJobScheduling{
					{TargetReplicatedJobs: []string{constants.Node}},
				},
			},
			rJobName: constants.Launcher,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotName, gotOK := podGroupTemplateName(trainJob, tc.scheduling, tc.rJobName)
			if gotName != tc.wantName || gotOK != tc.wantOK {
				t.Errorf("Unexpected PodGroupTemplate name: got (%q, %v), want (%q, %v)", gotName, gotOK, tc.wantName, tc.wantOK)
			}
		})
	}
}

func TestPodGroupName(t *testing.T) {
	trainJob := utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").UID("uid").Obj()
	cases := map[string]struct {
		templateName string
		wantName     string
	}{
		"TrainJob-level PodGroupTemplate omits the redundant template segment": {
			templateName: "test-job",
			wantName:     "test-job-e2577dd78b-b6ff97ef4e",
		},
		"per-ReplicatedJob PodGroupTemplate is appended to the Workload name": {
			templateName: "1-node",
			wantName:     "test-job-e2577dd78b-1-node-cc90ebe740",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := podGroupName(trainJob, tc.templateName); got != tc.wantName {
				t.Errorf("Unexpected PodGroup name: got %q, want %q", got, tc.wantName)
			}
		})
	}
}

func TestHashedName(t *testing.T) {
	cases := map[string]struct {
		prefix   string
		hash     string
		wantName string
	}{
		"hash is appended to the prefix": {
			prefix:   "test-job",
			hash:     "0123456789",
			wantName: "test-job-0123456789",
		},
		"long prefix is truncated to fit into a DNS label": {
			prefix:   strings.Repeat("a", 63),
			hash:     "0123456789",
			wantName: strings.Repeat("a", 52) + "-0123456789",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := hashedName(tc.prefix, tc.hash); got != tc.wantName {
				t.Errorf("Unexpected hashed name: got %q, want %q", got, tc.wantName)
			}
		})
	}
}

func TestNameHash(t *testing.T) {
	if got, want := nameHash("uid"), "e2577dd78b"; got != want {
		t.Errorf("Unexpected name hash: got %q, want %q", got, want)
	}
}

func TestFindReplicatedJob(t *testing.T) {
	cases := map[string]struct {
		jobSetSpec *jobsetv1alpha2ac.JobSetSpecApplyConfiguration
		name       string
		wantRJob   *jobsetv1alpha2ac.ReplicatedJobApplyConfiguration
	}{
		"ReplicatedJob exists": {
			jobSetSpec: jobsetv1alpha2ac.JobSetSpec().WithReplicatedJobs(
				jobsetv1alpha2ac.ReplicatedJob().WithName(constants.DatasetInitializer),
				jobsetv1alpha2ac.ReplicatedJob().WithName(constants.Node).WithReplicas(2),
			),
			name:     constants.Node,
			wantRJob: jobsetv1alpha2ac.ReplicatedJob().WithName(constants.Node).WithReplicas(2),
		},
		"ReplicatedJob does not exist": {
			jobSetSpec: jobsetv1alpha2ac.JobSetSpec().WithReplicatedJobs(
				jobsetv1alpha2ac.ReplicatedJob().WithName(constants.Node),
			),
			name: constants.Launcher,
		},
		"JobSet spec is nil": {
			name: constants.Node,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tc.wantRJob, findReplicatedJob(tc.jobSetSpec, tc.name)); len(diff) != 0 {
				t.Errorf("Unexpected ReplicatedJob (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestMinCount(t *testing.T) {
	cases := map[string]struct {
		podSets      runtime.PodSets
		jobSetSpec   *jobsetv1alpha2ac.JobSetSpecApplyConfiguration
		rJobNames    []string
		wantMinCount int32
	}{
		"Pods of every ReplicatedJob are counted": {
			podSets: runtime.PodSets{
				{Name: constants.DatasetInitializer, Count: ptr.To[int32](1)},
				{Name: constants.Node, Count: ptr.To[int32](4)},
			},
			jobSetSpec: jobsetv1alpha2ac.JobSetSpec().WithReplicatedJobs(
				jobsetv1alpha2ac.ReplicatedJob().WithName(constants.DatasetInitializer).WithReplicas(1),
				jobsetv1alpha2ac.ReplicatedJob().WithName(constants.Node).WithReplicas(1),
			),
			rJobNames:    []string{constants.DatasetInitializer, constants.Node},
			wantMinCount: 5,
		},
		"Pods of every replica are counted": {
			podSets: runtime.PodSets{{Name: "sidecar", Count: ptr.To[int32](2)}},
			jobSetSpec: jobsetv1alpha2ac.JobSetSpec().WithReplicatedJobs(
				jobsetv1alpha2ac.ReplicatedJob().WithName("sidecar").WithReplicas(3),
			),
			rJobNames:    []string{"sidecar"},
			wantMinCount: 6,
		},
		"ReplicatedJob depending on another ReplicatedJob of the group is excluded": {
			podSets: runtime.PodSets{
				{Name: constants.Node, Count: ptr.To[int32](3)},
				{Name: constants.Launcher, Count: ptr.To[int32](1)},
			},
			jobSetSpec: jobsetv1alpha2ac.JobSetSpec().WithReplicatedJobs(
				jobsetv1alpha2ac.ReplicatedJob().WithName(constants.Node).WithReplicas(1),
				jobsetv1alpha2ac.ReplicatedJob().WithName(constants.Launcher).WithReplicas(1).
					WithDependsOn(jobsetv1alpha2ac.DependsOn().WithName(constants.Node).WithStatus(jobsetv1alpha2.DependencyReady)),
			),
			rJobNames:    []string{constants.Launcher, constants.Node},
			wantMinCount: 3,
		},
		"ReplicatedJob depending on a ReplicatedJob outside of the group is counted": {
			podSets: runtime.PodSets{{Name: constants.Node, Count: ptr.To[int32](4)}},
			jobSetSpec: jobsetv1alpha2ac.JobSetSpec().WithReplicatedJobs(
				jobsetv1alpha2ac.ReplicatedJob().WithName(constants.Node).WithReplicas(1).
					WithDependsOn(jobsetv1alpha2ac.DependsOn().WithName(constants.DatasetInitializer).WithStatus(jobsetv1alpha2.DependencyComplete)),
			),
			rJobNames:    []string{constants.Node},
			wantMinCount: 4,
		},
		"ReplicatedJob without a PodSet is not counted": {
			podSets:      runtime.PodSets{{Name: constants.Node, Count: ptr.To[int32](4)}},
			rJobNames:    []string{constants.Launcher, constants.Node},
			wantMinCount: 4,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := minCount(tc.podSets, tc.jobSetSpec, tc.rJobNames); got != tc.wantMinCount {
				t.Errorf("Unexpected minCount: got %d, want %d", got, tc.wantMinCount)
			}
		})
	}
}

func TestPriorityClassName(t *testing.T) {
	jobSetSpec := jobsetv1alpha2ac.JobSetSpec().WithReplicatedJobs(
		jobsetv1alpha2ac.ReplicatedJob().WithName(constants.Node).
			WithTemplate(batchv1ac.JobTemplateSpec().WithSpec(batchv1ac.JobSpec().
				WithTemplate(corev1ac.PodTemplateSpec().WithSpec(corev1ac.PodSpec().WithPriorityClassName("high-priority"))))),
	)
	cases := map[string]struct {
		jobSetSpec *jobsetv1alpha2ac.JobSetSpecApplyConfiguration
		rJobName   string
		wantName   string
	}{
		"priorityClassName of the ReplicatedJob Pod template": {
			jobSetSpec: jobSetSpec,
			rJobName:   constants.Node,
			wantName:   "high-priority",
		},
		"ReplicatedJob does not exist": {
			jobSetSpec: jobSetSpec,
			rJobName:   constants.Launcher,
		},
		"JobSet spec is nil": {
			rJobName: constants.Node,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := priorityClassName(tc.jobSetSpec, tc.rJobName); got != tc.wantName {
				t.Errorf("Unexpected priorityClassName: got %q, want %q", got, tc.wantName)
			}
		})
	}
}

func TestValidateScheduling(t *testing.T) {
	cases := map[string]struct {
		scheduling *trainer.TrainingRuntimeScheduling
		wantError  field.ErrorList
	}{
		"no validation when the runtime does not set scheduling": {},
		"valid TrainJob-level scheduling": {
			scheduling: &trainer.TrainingRuntimeScheduling{
				SchedulingPolicy: &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingPolicy{
					Gang: &schedulingv1alpha3.WorkloadCompositePodGroupGangSchedulingPolicy{},
				},
				DisruptionMode: &schedulingv1alpha3.WorkloadCompositePodGroupDisruptionMode{
					All: &schedulingv1alpha3.WorkloadCompositePodGroupAllDisruptionMode{},
				},
			},
		},
		"TrainJob-level basic scheduling with the all disruptionMode": {
			scheduling: &trainer.TrainingRuntimeScheduling{
				SchedulingPolicy: &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingPolicy{
					Basic: &schedulingv1alpha3.WorkloadCompositePodGroupBasicSchedulingPolicy{},
				},
				DisruptionMode: &schedulingv1alpha3.WorkloadCompositePodGroupDisruptionMode{
					All: &schedulingv1alpha3.WorkloadCompositePodGroupAllDisruptionMode{},
				},
			},
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("spec", "scheduling", "disruptionMode"), "", ""),
			},
		},
		"TrainJob-level scheduling with an invalid topology key": {
			scheduling: &trainer.TrainingRuntimeScheduling{
				SchedulingConstraints: &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingConstraints{
					Topology: []schedulingv1alpha3.TopologyConstraint{{Key: "invalid key"}},
				},
			},
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("spec", "scheduling", "schedulingConstraints", "topology").Index(0).Child("key"), "", ""),
			},
		},
		"per-ReplicatedJob basic scheduling with the all disruptionMode": {
			scheduling: &trainer.TrainingRuntimeScheduling{
				ReplicatedJobs: []trainer.ReplicatedJobScheduling{
					{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer}},
					{
						TargetReplicatedJobs: []string{constants.Node},
						SchedulingPolicy: &schedulingv1alpha3.WorkloadCompositePodGroupSchedulingPolicy{
							Basic: &schedulingv1alpha3.WorkloadCompositePodGroupBasicSchedulingPolicy{},
						},
						DisruptionMode: &schedulingv1alpha3.WorkloadCompositePodGroupDisruptionMode{
							All: &schedulingv1alpha3.WorkloadCompositePodGroupAllDisruptionMode{},
						},
					},
				},
			},
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("spec", "scheduling", "replicatedJobs").Index(1).Child("disruptionMode"), "", ""),
			},
		},
		"Job-level basic scheduling with the all disruptionMode": {
			scheduling: &trainer.TrainingRuntimeScheduling{
				ReplicatedJobs: []trainer.ReplicatedJobScheduling{
					{TargetReplicatedJobs: []string{constants.DatasetInitializer, constants.ModelInitializer}},
					{
						TargetReplicatedJobs: []string{constants.Node},
						Job: &trainer.JobScheduling{
							SchedulingPolicy: &schedulingv1alpha3.WorkloadPodGroupSchedulingPolicy{
								Basic: &schedulingv1alpha3.WorkloadPodGroupBasicSchedulingPolicy{},
							},
							DisruptionMode: &schedulingv1alpha3.WorkloadPodGroupDisruptionMode{
								All: &schedulingv1alpha3.WorkloadPodGroupAllDisruptionMode{},
							},
						},
					},
				},
			},
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("spec", "scheduling", "replicatedJobs").Index(1).Child("job", "disruptionMode"), "", ""),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			spec := utiltesting.MakeTrainingRuntimeSpecWrapper(utiltesting.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").Obj().Spec).
				Scheduling(tc.scheduling).
				Obj()
			gotError := ValidateScheduling(context.Background(), &spec)
			if diff := cmp.Diff(tc.wantError, gotError, cmpopts.IgnoreFields(field.Error{}, "Type", "Detail", "BadValue", "Origin")); len(diff) != 0 {
				t.Errorf("Unexpected errors from ValidateScheduling (-want, +got): %s", diff)
			}
		})
	}
}
