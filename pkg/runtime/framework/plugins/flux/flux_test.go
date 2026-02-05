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
	"fmt"
	"strings"
	"testing"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

	var procs int32 = 1

	cases := map[string]struct {
		info               *runtime.Info
		trainJob           *trainer.TrainJob
		wantMLPolicyError  error
		wantInfo           *runtime.Info
		wantBuildError     error
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
			if diff := gocmp.Diff(tc.wantMLPolicyError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error from EnforceMLPolicy (-want, +got): %s", diff)
			}
			if tc.wantInfo != nil {
				if diff := gocmp.Diff(tc.wantInfo, tc.info,
					cmpopts.SortSlices(func(a, b string) bool { return a < b }),
					cmpopts.SortMaps(func(a, b int) bool { return a < b }),
					utiltesting.PodSetEndpointsCmpOpts,
				); len(diff) != 0 {
					t.Errorf("Unexpected info from EnforceMLPolicy (-want, +got): %s", diff)
				}
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

			var objs []apiruntime.ApplyConfiguration
			objs, err = p.(framework.ComponentBuilderPlugin).Build(ctx, tc.info, tc.trainJob)
			if diff := gocmp.Diff(tc.wantBuildError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error from Build (-want, +got): %s", diff)
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

func TestGenerateRange(t *testing.T) {
	cases := []struct {
		name  string
		size  int32
		start int32
		want  string
	}{
		{
			name:  "single node",
			size:  1,
			start: 0,
			want:  "0",
		},
		{
			name:  "multiple nodes starting at zero",
			size:  4,
			start: 0,
			want:  "0-3",
		},
		{
			name:  "multiple nodes with offset start",
			size:  3,
			start: 10,
			want:  "10-12",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := generateRange(tc.size, tc.start)
			if got != tc.want {
				t.Errorf("generateRange(%d, %d) = %q; want %q", tc.size, tc.start, got, tc.want)
			}
		})
	}
}

func TestGenerateHostlist(t *testing.T) {
	cases := []struct {
		name   string
		prefix string
		size   int32
		want   string
	}{
		{
			name:   "prefix with one node",
			prefix: "lammps-job",
			size:   1,
			want:   fmt.Sprintf("lammps-job-%s-0-[0]", constants.Node),
		},
		{
			name:   "prefix with four nodes",
			prefix: "flux-cluster",
			size:   4,
			want:   fmt.Sprintf("flux-cluster-%s-0-[0-3]", constants.Node),
		},
		{
			name:   "empty prefix handled",
			prefix: "",
			size:   2,
			want:   fmt.Sprintf("-%s-0-[0-1]", constants.Node),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := generateHostlist(tc.prefix, tc.size)
			if got != tc.want {
				t.Errorf("generateHostlist(%q, %d) = %q; want %q", tc.prefix, tc.size, got, tc.want)
			}
		})
	}
}

func TestEncodeZ85(t *testing.T) {
	cases := []struct {
		name     string
		input    []byte
		wantLen  int
		expected string
	}{
		{
			name:    "32 bytes produces 40 characters",
			input:   make([]byte, 32),
			wantLen: 40,
		},
		{
			name:    "invalid length returns empty string",
			input:   []byte{1, 2, 3}, // Not a multiple of 4
			wantLen: 0,
		},
		{
			name:     "all zeros produces zeros in Z85",
			input:    []byte{0, 0, 0, 0},
			wantLen:  5,
			expected: "00000",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := encodeZ85(tc.input)
			if len(got) != tc.wantLen {
				t.Errorf("encodeZ85() length = %d; want %d", len(got), tc.wantLen)
			}
			if tc.expected != "" && got != tc.expected {
				t.Errorf("encodeZ85() = %q; want %q", got, tc.expected)
			}
		})
	}
}

func TestBuildCurveSecret(t *testing.T) {
	f := &Flux{}
	trainJob := &trainer.TrainJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
			UID:       types.UID("12345-67890"),
		},
	}

	// Check generation
	secretApply, err := f.buildCurveSecret(trainJob)
	if err != nil {
		t.Fatalf("buildCurveSecret failed: %v", err)
	}

	if *secretApply.Name != "test-job-flux-curve" {
		t.Errorf("Expected secret name test-job-flux-curve, got %s", *secretApply.Name)
	}

	// Check format of the certificate content
	certBytes, ok := secretApply.Data["curve.cert"]
	if !ok {
		t.Fatal("curve.cert key missing from secret data")
	}
	certContent := string(certBytes)

	requiredHeaders := []string{"metadata", "curve", "public-key =", "secret-key =", "name = \"test-job\""}
	for _, header := range requiredHeaders {
		if !strings.Contains(certContent, header) {
			t.Errorf("certContent missing required header/field: %q", header)
		}
	}

	// Check Determinism: Same UID must produce same keys
	secretApply2, _ := f.buildCurveSecret(trainJob)
	if string(secretApply.Data["curve.cert"]) != string(secretApply2.Data["curve.cert"]) {
		t.Error("buildCurveSecret is not deterministic; generated different certs for the same UID")
	}

	// Check Uniqueness: Different UID must produce different keys
	trainJob.UID = types.UID("different-uid")
	secretApply3, _ := f.buildCurveSecret(trainJob)
	if string(secretApply.Data["curve.cert"]) == string(secretApply3.Data["curve.cert"]) {
		t.Error("buildCurveSecret produced the same cert for different UIDs")
	}

	// Verify indentation (Flux/CZMQ requires 4 spaces)
	if !strings.Contains(certContent, "    public-key =") {
		t.Error("certContent does not use the required 4-space indentation for key fields")
	}
}

func TestGetOriginalCommand(t *testing.T) {
	cases := []struct {
		name     string
		trainJob *trainer.TrainJob
		info     *runtime.Info
		want     string
	}{
		{
			name: "full command and args",
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().
					Container("image", []string{"python"}, []string{"train.py", "--epochs", "10"}, nil).
					Obj()).
				Obj(),
			info: &runtime.Info{},
			want: "python train.py --epochs 10",
		},
		{
			name: "command and args with extra spaces",
			trainJob: utiltesting.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").
				Trainer(utiltesting.MakeTrainJobTrainerWrapper().
					Container("image", []string{"  python  "}, []string{" script.py "}, nil).
					Obj()).
				Obj(),
			info: &runtime.Info{},
			want: "python    script.py",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := getOriginalCommand(tc.trainJob, tc.info)
			if got != tc.want {
				t.Errorf("getOriginalCommand() = %q; want %q", got, tc.want)
			}
		})
	}
}
