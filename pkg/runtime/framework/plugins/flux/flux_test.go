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

package flux

import (
	"cmp"
	"strings"
	"testing"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	batchv1ac "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestFlux(t *testing.T) {
	objCmpOpts := []gocmp.Option{
		cmpopts.SortSlices(func(a, b apiruntime.Object) int {
			return cmp.Compare(a.GetObjectKind().GroupVersionKind().String(), b.GetObjectKind().GroupVersionKind().String())
		}),
		cmpopts.SortSlices(func(a, b corev1.EnvVar) int { return cmp.Compare(a.Name, b.Name) }),
		cmpopts.IgnoreFields(corev1.ConfigMap{}, "Data"),
		cmpopts.IgnoreFields(corev1.Secret{}, "Data"),
	}

	procs := intstr.FromInt32(1)

	cases := map[string]struct {
		info               *runtime.Info
		trainJob           *trainer.TrainJob
		wantObjs           []apiruntime.Object
		wantInitContainers []string
		wantCommand        []string
		wantTTY            bool
	}{
		"no action when flux policy is nil": {
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").Obj(),
		},
		"flux mutations are applied correctly": {
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					FluxPolicySource: &trainer.FluxMLPolicySource{
						NumProcPerNode: &procs,
					},
				},
				TemplateSpec: runtime.TemplateSpec{
					ObjApply: v1alpha2.JobSetSpec().WithReplicatedJobs(
						v1alpha2.ReplicatedJob().WithTemplate(
							batchv1ac.JobTemplateSpec().WithSpec(
								batchv1ac.JobSpec().WithTemplate(
									corev1ac.PodTemplateSpec().WithSpec(
										corev1ac.PodSpec().WithContainers(
											corev1ac.Container().WithName(constants.Node),
										),
									),
								),
							),
						),
					),
					PodSets: []runtime.PodSet{
						{
							Name: constants.Node,
						},
					},
				},
			},
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("test-uid").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().NumNodes(2).Obj()).
				Obj(),
			wantInitContainers: []string{"flux-installer"},
			wantCommand:        []string{"/bin/bash", "/etc/flux-config/entrypoint.sh"},
			wantTTY:            true,
			wantObjs: []apiruntime.Object{
				utiltesting.MakeConfigMapWrapper("test-job-flux-entrypoint", metav1.NamespaceDefault).
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "test-uid").
					Obj(),
				utiltesting.MakeSecretWrapper("test-job-flux-curve", metav1.NamespaceDefault).
					ControllerReference(trainer.SchemeGroupVersion.WithKind(trainer.TrainJobKind), "test-job", "test-uid").
					Obj(),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			cli := utiltesting.NewClientBuilder().Build()
			p, _ := New(ctx, cli, nil)

			err := p.(framework.EnforceMLPolicyPlugin).EnforceMLPolicy(tc.info, tc.trainJob)
			if err != nil {
				t.Fatalf("EnforceMLPolicy failed: %v", err)
			}

			if tc.info.RuntimePolicy.FluxPolicySource != nil && tc.info.TemplateSpec.ObjApply != nil {
				js := tc.info.TemplateSpec.ObjApply.(*v1alpha2.JobSetSpecApplyConfiguration)
				for _, rj := range js.ReplicatedJobs {
					if ptr.Deref(rj.Name, "") == constants.Node {
						podSpec := rj.Template.Spec.Template.Spec
						var icNames []string
						for _, ic := range podSpec.InitContainers {
							icNames = append(icNames, ptr.Deref(ic.Name, ""))
						}
						if diff := gocmp.Diff(tc.wantInitContainers, icNames); len(diff) != 0 {
							t.Errorf("Unexpected init containers (-want, +got): %s", diff)
						}
						for _, c := range podSpec.Containers {
							if ptr.Deref(c.Name, "") == constants.Node {
								if diff := gocmp.Diff(tc.wantCommand, c.Command); len(diff) != 0 {
									t.Errorf("Unexpected command (-want, +got): %s", diff)
								}
								if ptr.Deref(c.TTY, false) != tc.wantTTY {
									t.Errorf("Expected TTY %v, got %v", tc.wantTTY, ptr.Deref(c.TTY, false))
								}
							}
						}
					}
				}
			}

			objs, err := p.(framework.ComponentBuilderPlugin).Build(ctx, tc.info, tc.trainJob)
			if err != nil {
				t.Fatalf("Build failed: %v", err)
			}

			typedObjs, _ := utiltesting.ToObject(cli.Scheme(), objs...)
			if diff := gocmp.Diff(tc.wantObjs, typedObjs, objCmpOpts...); len(diff) != 0 {
				t.Errorf("Unexpected objects from Build (-want, +got): %s", diff)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	cases := map[string]struct {
		info   *runtime.Info
		newObj *trainer.TrainJob
	}{
		"valid when flux policy is nil": {
			info:   &runtime.Info{},
			newObj: &trainer.TrainJob{},
		},
		"valid when flux policy is present": {
			info: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					FluxPolicySource: &trainer.FluxMLPolicySource{},
				},
			},
			newObj: &trainer.TrainJob{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			p, _ := New(ctx, utiltesting.NewClientBuilder().Build(), nil)

			_, errs := p.(framework.CustomValidationPlugin).Validate(ctx, tc.info, nil, tc.newObj)
			if len(errs) > 0 {
				t.Errorf("Unexpected validation error: %v", errs)
			}
		})
	}
}

func TestDeterministicCurve(t *testing.T) {
	p := &Flux{}
	job1 := &trainer.TrainJob{
		ObjectMeta: metav1.ObjectMeta{Name: "job", Namespace: "ns", UID: "uid-123"},
	}
	job2 := &trainer.TrainJob{
		ObjectMeta: metav1.ObjectMeta{Name: "job", Namespace: "ns", UID: "uid-123"},
	}

	sec1, err := p.buildCurveSecret(job1)
	if err != nil {
		t.Fatalf("Failed to build secret 1: %v", err)
	}
	sec2, err := p.buildCurveSecret(job2)
	if err != nil {
		t.Fatalf("Failed to build secret 2: %v", err)
	}

	data1 := string(sec1.Data["curve.cert"])
	data2 := string(sec2.Data["curve.cert"])

	if data1 != data2 {
		t.Error("Deterministic curve generation failed: secrets are not identical for the same UID")
	}

	if !strings.Contains(data1, "curve") || !strings.Contains(data1, "public-key") {
		t.Error("Secret data missing expected CZMQ headers or fields")
	}
}
