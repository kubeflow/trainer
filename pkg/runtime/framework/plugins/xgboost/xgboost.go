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

package xgboost

import (
	"context"
	///"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
)

// XGBoost implements the EnforceMLPolicyPlugin interface for distributed
// XGBoost training using Rabit coordination.
type XGBoost struct{}

var _ framework.EnforceMLPolicyPlugin = (*XGBoost)(nil)

const Name = "XGBoost"

func New(context.Context, client.Client, client.FieldIndexer) (framework.Plugin, error) {
	return &XGBoost{}, nil
}
func (x *XGBoost) Name() string {
	return Name
}

// TODO: Inject DMLC_* Rabit environment variables for
// distributed XGBoost training. See KEP for env var specification.
func (x *XGBoost) EnforceMLPolicy(info *runtime.Info, trainJob *trainer.TrainJob) error {
	return nil
}
