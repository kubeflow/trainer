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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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

func TestNetworkPolicyRuleLimits(t *testing.T) {
	for name, cfg := range map[string]*configapi.NetworkPolicyConfig{
		"ingress": {Ingress: make([]networkingv1.NetworkPolicyIngressRule, 129)},
		"egress":  {Egress: make([]networkingv1.NetworkPolicyEgressRule, 129)},
	} {
		t.Run(name, func(t *testing.T) {
			errs := validate(&configapi.Configuration{NetworkPolicy: cfg})
			if len(errs) != 1 || errs[0].Type != field.ErrorTypeTooMany || errs[0].Field != "networkPolicy."+name {
				t.Fatalf("unexpected errors: %v", errs)
			}
		})
	}
}

func TestNetworkPolicyRules(t *testing.T) {
	for name, tc := range map[string]struct {
		ports     []networkingv1.NetworkPolicyPort
		peers     []networkingv1.NetworkPolicyPeer
		field     string
		errorType field.ErrorType
	}{
		"empty rule allows all":  {},
		"numeric range":          {ports: []networkingv1.NetworkPolicyPort{{Port: ptr.To(intstr.FromInt32(80)), EndPort: ptr.To[int32](90)}}},
		"named SCTP port":        {ports: []networkingv1.NetworkPolicyPort{{Port: ptr.To(intstr.FromString("http")), Protocol: ptr.To(corev1.ProtocolSCTP)}}},
		"UDP all ports":          {ports: []networkingv1.NetworkPolicyPort{{Protocol: ptr.To(corev1.ProtocolUDP)}}},
		"IPv4 exception":         {peers: []networkingv1.NetworkPolicyPeer{{IPBlock: &networkingv1.IPBlock{CIDR: "10.0.0.0/8", Except: []string{"10.1.0.0/16"}}}}},
		"IPv6 exception":         {peers: []networkingv1.NetworkPolicyPeer{{IPBlock: &networkingv1.IPBlock{CIDR: "2001:db8::/32", Except: []string{"2001:db8:1::/48"}}}}},
		"selectors together":     {peers: []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{}, NamespaceSelector: &metav1.LabelSelector{}}}},
		"bad protocol":           {ports: []networkingv1.NetworkPolicyPort{{Protocol: ptr.To(corev1.Protocol("ICMP"))}}, field: "ports[0].protocol", errorType: field.ErrorTypeNotSupported},
		"zero port":              {ports: []networkingv1.NetworkPolicyPort{{Port: ptr.To(intstr.FromInt32(0))}}, field: "ports[0].port"},
		"port overflow":          {ports: []networkingv1.NetworkPolicyPort{{Port: ptr.To(intstr.FromInt32(65536))}}, field: "ports[0].port"},
		"invalid named port":     {ports: []networkingv1.NetworkPolicyPort{{Port: ptr.To(intstr.FromString("HTTP!"))}}, field: "ports[0].port"},
		"reversed range":         {ports: []networkingv1.NetworkPolicyPort{{Port: ptr.To(intstr.FromInt32(90)), EndPort: ptr.To[int32](80)}}, field: "ports[0].endPort"},
		"end without port":       {ports: []networkingv1.NetworkPolicyPort{{EndPort: ptr.To[int32](80)}}, field: "ports[0].endPort"},
		"end with named port":    {ports: []networkingv1.NetworkPolicyPort{{Port: ptr.To(intstr.FromString("http")), EndPort: ptr.To[int32](80)}}, field: "ports[0].endPort"},
		"end overflow":           {ports: []networkingv1.NetworkPolicyPort{{Port: ptr.To(intstr.FromInt32(80)), EndPort: ptr.To[int32](65536)}}, field: "ports[0].endPort"},
		"bad CIDR":               {peers: []networkingv1.NetworkPolicyPeer{{IPBlock: &networkingv1.IPBlock{CIDR: "10.0.0.0/33"}}}, field: "PEER[0].ipBlock.cidr"},
		"bad exception":          {peers: []networkingv1.NetworkPolicyPeer{{IPBlock: &networkingv1.IPBlock{CIDR: "10.0.0.0/8", Except: []string{"invalid"}}}}, field: "PEER[0].ipBlock.except[0]"},
		"exception outside CIDR": {peers: []networkingv1.NetworkPolicyPeer{{IPBlock: &networkingv1.IPBlock{CIDR: "10.0.0.0/8", Except: []string{"192.0.2.0/24"}}}}, field: "PEER[0].ipBlock.except[0]"},
		"exception equals CIDR":  {peers: []networkingv1.NetworkPolicyPeer{{IPBlock: &networkingv1.IPBlock{CIDR: "10.0.0.0/8", Except: []string{"10.0.0.0/8"}}}}, field: "PEER[0].ipBlock.except[0]"},
		"empty peer":             {peers: []networkingv1.NetworkPolicyPeer{{}}, field: "PEER[0]", errorType: field.ErrorTypeRequired},
		"mixed peer":             {peers: []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{}, IPBlock: &networkingv1.IPBlock{CIDR: "10.0.0.0/8"}}}, field: "PEER[0]", errorType: field.ErrorTypeForbidden},
		"invalid selector":       {peers: []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"invalid!": "yes"}}}}, field: "PEER[0].podSelector.matchLabels"},
	} {
		for direction, peerField := range map[string]string{"ingress": "from", "egress": "to"} {
			t.Run(name+"/"+direction, func(t *testing.T) {
				policy := &configapi.NetworkPolicyConfig{}
				if direction == "ingress" {
					policy.Ingress = []networkingv1.NetworkPolicyIngressRule{{Ports: tc.ports, From: tc.peers}}
				} else {
					policy.Egress = []networkingv1.NetworkPolicyEgressRule{{Ports: tc.ports, To: tc.peers}}
				}
				errs := validate(&configapi.Configuration{NetworkPolicy: policy})
				if tc.field == "" {
					if len(errs) != 0 {
						t.Fatalf("unexpected errors: %v", errs)
					}
					return
				}
				wantType := tc.errorType
				if wantType == "" {
					wantType = field.ErrorTypeInvalid
				}
				wantField := "networkPolicy." + direction + "[0]." + strings.ReplaceAll(tc.field, "PEER", peerField)
				if len(errs) != 1 || errs[0].Field != wantField || errs[0].Type != wantType {
					t.Fatalf("got %v, want %s on %s", errs, wantType, wantField)
				}
			})
		}
	}
}
