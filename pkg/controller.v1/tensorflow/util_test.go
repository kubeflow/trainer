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

package tensorflow

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	kubeflowv1 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v1"
	tftestutil "github.com/kubeflow/training-operator/pkg/controller.v1/tensorflow/testutil"
)

func TestGenOwnerReference(t *testing.T) {
	testUID := uuid.NewUUID()
	tfJob := &kubeflowv1.TFJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: tftestutil.TestTFJobName,
			UID:  testUID,
		},
	}

	ref := reconciler.GenOwnerReference(tfJob)
	if ref.UID != testUID {
		t.Errorf("Expected UID %s, got %s", testUID, ref.UID)
	}
	if ref.Name != tftestutil.TestTFJobName {
		t.Errorf("Expected Name %s, got %s", tftestutil.TestTFJobName, ref.Name)
	}
	if ref.APIVersion != kubeflowv1.SchemeGroupVersion.String() {
		t.Errorf("Expected APIVersion %s, got %s", kubeflowv1.SchemeGroupVersion.String(), ref.APIVersion)
	}
}

func TestGenLabels(t *testing.T) {
	testJobName := "test/key"
	expctedVal := "test-key"

	labels := reconciler.GenLabels(testJobName)
	jobNameLabel := kubeflowv1.JobNameLabel

	if labels[jobNameLabel] != expctedVal {
		t.Errorf("Expected %s %s, got %s", jobNameLabel, expctedVal, jobNameLabel)
	}

	if labels[kubeflowv1.OperatorNameLabel] != controllerName {
		t.Errorf("Expected %s %s, got %s", kubeflowv1.OperatorNameLabel, controllerName,
			labels[kubeflowv1.OperatorNameLabel])
	}
}
