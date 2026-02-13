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

package config

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
)

func TestValidate(t *testing.T) {
	testCases := map[string]struct {
		cfg     *configapi.Configuration
		wantErr field.ErrorList
	}{
		"valid empty configuration": {
			cfg:     &configapi.Configuration{},
			wantErr: nil,
		},
		"valid complete configuration": {
			cfg: &configapi.Configuration{
				Webhook: configapi.ControllerWebhook{
					Port: ptr.To[int32](9443),
					Host: ptr.To("0.0.0.0"),
				},
				Metrics: configapi.ControllerMetrics{
					BindAddress:   ":8443",
					SecureServing: ptr.To(true),
				},
				Health: configapi.ControllerHealth{
					HealthProbeBindAddress: ":8081",
					ReadinessEndpointName:  "readyz",
					LivenessEndpointName:   "healthz",
				},
				ClientConnection: &configapi.ClientConnection{
					QPS:   ptr.To[float32](50),
					Burst: ptr.To[int32](100),
				},
				Controller: &configapi.ControllerConfigurationSpec{
					GroupKindConcurrency: map[string]int32{
						"TrainJob.trainer.kubeflow.org": 5,
					},
				},
			},
			wantErr: nil,
		},
		"invalid webhook port too low": {
			cfg: &configapi.Configuration{
				Webhook: configapi.ControllerWebhook{
					Port: ptr.To[int32](0),
				},
			},
			wantErr: field.ErrorList{
				&field.Error{
					Type:  field.ErrorTypeInvalid,
					Field: "webhook.port",
				},
			},
		},
		"invalid webhook port too high": {
			cfg: &configapi.Configuration{
				Webhook: configapi.ControllerWebhook{
					Port: ptr.To[int32](70000),
				},
			},
			wantErr: field.ErrorList{
				&field.Error{
					Type:  field.ErrorTypeInvalid,
					Field: "webhook.port",
				},
			},
		},
		"valid webhook port at lower boundary": {
			cfg: &configapi.Configuration{
				Webhook: configapi.ControllerWebhook{
					Port: ptr.To[int32](1),
				},
			},
			wantErr: nil,
		},
		"valid webhook port at upper boundary": {
			cfg: &configapi.Configuration{
				Webhook: configapi.ControllerWebhook{
					Port: ptr.To[int32](65535),
				},
			},
			wantErr: nil,
		},
		"invalid negative QPS": {
			cfg: &configapi.Configuration{
				ClientConnection: &configapi.ClientConnection{
					QPS: ptr.To[float32](-1),
				},
			},
			wantErr: field.ErrorList{
				&field.Error{
					Type:  field.ErrorTypeInvalid,
					Field: "clientConnection.qps",
				},
			},
		},
		"valid QPS at zero": {
			cfg: &configapi.Configuration{
				ClientConnection: &configapi.ClientConnection{
					QPS: ptr.To[float32](0),
				},
			},
			wantErr: nil,
		},
		"valid QPS with decimal": {
			cfg: &configapi.Configuration{
				ClientConnection: &configapi.ClientConnection{
					QPS: ptr.To[float32](123.456),
				},
			},
			wantErr: nil,
		},
		"invalid negative Burst": {
			cfg: &configapi.Configuration{
				ClientConnection: &configapi.ClientConnection{
					Burst: ptr.To[int32](-1),
				},
			},
			wantErr: field.ErrorList{
				&field.Error{
					Type:  field.ErrorTypeInvalid,
					Field: "clientConnection.burst",
				},
			},
		},
		"valid Burst at zero": {
			cfg: &configapi.Configuration{
				ClientConnection: &configapi.ClientConnection{
					Burst: ptr.To[int32](0),
				},
			},
			wantErr: nil,
		},
		"invalid concurrency zero": {
			cfg: &configapi.Configuration{
				Controller: &configapi.ControllerConfigurationSpec{
					GroupKindConcurrency: map[string]int32{
						"TrainJob.trainer.kubeflow.org": 0,
					},
				},
			},
			wantErr: field.ErrorList{
				&field.Error{
					Type:  field.ErrorTypeInvalid,
					Field: "controller.groupKindConcurrency[TrainJob.trainer.kubeflow.org]",
				},
			},
		},
		"invalid concurrency negative": {
			cfg: &configapi.Configuration{
				Controller: &configapi.ControllerConfigurationSpec{
					GroupKindConcurrency: map[string]int32{
						"TrainJob.trainer.kubeflow.org": -5,
					},
				},
			},
			wantErr: field.ErrorList{
				&field.Error{
					Type:  field.ErrorTypeInvalid,
					Field: "controller.groupKindConcurrency[TrainJob.trainer.kubeflow.org]",
				},
			},
		},
		"valid concurrency at minimum": {
			cfg: &configapi.Configuration{
				Controller: &configapi.ControllerConfigurationSpec{
					GroupKindConcurrency: map[string]int32{
						"TrainJob.trainer.kubeflow.org": 1,
					},
				},
			},
			wantErr: nil,
		},
		"valid high concurrency": {
			cfg: &configapi.Configuration{
				Controller: &configapi.ControllerConfigurationSpec{
					GroupKindConcurrency: map[string]int32{
						"TrainJob.trainer.kubeflow.org": 1000,
					},
				},
			},
			wantErr: nil,
		},
		"multiple validation errors": {
			cfg: &configapi.Configuration{
				Webhook: configapi.ControllerWebhook{
					Port: ptr.To[int32](99999),
				},
				ClientConnection: &configapi.ClientConnection{
					QPS:   ptr.To[float32](-10),
					Burst: ptr.To[int32](-20),
				},
				Controller: &configapi.ControllerConfigurationSpec{
					GroupKindConcurrency: map[string]int32{
						"TrainJob.trainer.kubeflow.org": -1,
					},
				},
			},
			wantErr: field.ErrorList{
				&field.Error{
					Type:  field.ErrorTypeInvalid,
					Field: "webhook.port",
				},
				&field.Error{
					Type:  field.ErrorTypeInvalid,
					Field: "clientConnection.qps",
				},
				&field.Error{
					Type:  field.ErrorTypeInvalid,
					Field: "clientConnection.burst",
				},
				&field.Error{
					Type:  field.ErrorTypeInvalid,
					Field: "controller.groupKindConcurrency[TrainJob.trainer.kubeflow.org]",
				},
			},
		},
		"multiple resources with mixed validity": {
			cfg: &configapi.Configuration{
				Controller: &configapi.ControllerConfigurationSpec{
					GroupKindConcurrency: map[string]int32{
						"TrainJob.trainer.kubeflow.org":               10,
						"TrainingRuntime.trainer.kubeflow.org":        -1,
						"ClusterTrainingRuntime.trainer.kubeflow.org": 5,
					},
				},
			},
			wantErr: field.ErrorList{
				&field.Error{
					Type:  field.ErrorTypeInvalid,
					Field: "controller.groupKindConcurrency[TrainingRuntime.trainer.kubeflow.org]",
				},
			},
		},
		"nil pointer fields are valid": {
			cfg: &configapi.Configuration{
				ClientConnection: nil,
				Controller:       nil,
			},
			wantErr: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			errs := validate(tc.cfg)
			if diff := cmp.Diff(tc.wantErr, errs, cmpopts.IgnoreFields(field.Error{}, "BadValue", "Detail")); diff != "" {
				t.Errorf("Unexpected validation errors (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestValidateClientConnection(t *testing.T) {
	testCases := map[string]struct {
		cfg     *configapi.Configuration
		wantErr field.ErrorList
	}{
		"nil client connection": {
			cfg: &configapi.Configuration{
				ClientConnection: nil,
			},
		},
		"valid QPS and Burst at zero": {
			cfg: &configapi.Configuration{
				ClientConnection: &configapi.ClientConnection{
					QPS:   ptr.To[float32](0),
					Burst: ptr.To[int32](0),
				},
			},
		},
		"valid QPS and Burst positive": {
			cfg: &configapi.Configuration{
				ClientConnection: &configapi.ClientConnection{
					QPS:   ptr.To[float32](100),
					Burst: ptr.To[int32](200),
				},
			},
		},
		"valid large QPS and Burst": {
			cfg: &configapi.Configuration{
				ClientConnection: &configapi.ClientConnection{
					QPS:   ptr.To[float32](999999.99),
					Burst: ptr.To[int32](999999),
				},
			},
		},
		"invalid negative QPS": {
			cfg: &configapi.Configuration{
				ClientConnection: &configapi.ClientConnection{
					QPS:   ptr.To[float32](-0.1),
					Burst: ptr.To[int32](100),
				},
			},
			wantErr: field.ErrorList{
				&field.Error{
					Type:  field.ErrorTypeInvalid,
					Field: "clientConnection.qps",
				},
			},
		},
		"invalid negative Burst": {
			cfg: &configapi.Configuration{
				ClientConnection: &configapi.ClientConnection{
					QPS:   ptr.To[float32](100),
					Burst: ptr.To[int32](-1),
				},
			},
			wantErr: field.ErrorList{
				&field.Error{
					Type:  field.ErrorTypeInvalid,
					Field: "clientConnection.burst",
				},
			},
		},
		"invalid both negative": {
			cfg: &configapi.Configuration{
				ClientConnection: &configapi.ClientConnection{
					QPS:   ptr.To[float32](-1),
					Burst: ptr.To[int32](-1),
				},
			},
			wantErr: field.ErrorList{
				&field.Error{
					Type:  field.ErrorTypeInvalid,
					Field: "clientConnection.qps",
				},
				&field.Error{
					Type:  field.ErrorTypeInvalid,
					Field: "clientConnection.burst",
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			errs := validate(tc.cfg)
			if diff := cmp.Diff(tc.wantErr, errs, cmpopts.IgnoreFields(field.Error{}, "BadValue", "Detail")); diff != "" {
				t.Errorf("Unexpected validation errors (-want,+got):\n%s", diff)
			}
		})
	}
}
