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

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	netutils "k8s.io/utils/net"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
)

// validate validates the configuration.
func validate(cfg *configapi.Configuration) field.ErrorList {
	var allErrs field.ErrorList

	// Validate webhook port
	if cfg.Webhook.Port != nil && (*cfg.Webhook.Port < 1 || *cfg.Webhook.Port > 65535) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("webhook", "port"), *cfg.Webhook.Port, "must be between 1 and 65535"))
	}

	// Validate client connection QPS and Burst
	if cfg.ClientConnection != nil {
		if cfg.ClientConnection.QPS != nil && *cfg.ClientConnection.QPS < 0 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("clientConnection", "qps"), *cfg.ClientConnection.QPS, "must be greater than or equal to 0"))
		}
		if cfg.ClientConnection.Burst != nil && *cfg.ClientConnection.Burst < 0 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("clientConnection", "burst"), *cfg.ClientConnection.Burst, "must be greater than or equal to 0"))
		}
	}

	// Validate GroupKindConcurrency values
	if cfg.Controller != nil && cfg.Controller.GroupKindConcurrency != nil {
		for gk, concurrency := range cfg.Controller.GroupKindConcurrency {
			if concurrency < 1 {
				allErrs = append(allErrs, field.Invalid(field.NewPath("controller", "groupKindConcurrency").Key(gk), concurrency, "must be greater than 0"))
			}
		}
	}

	// Validate status server config
	if cfg.StatusServer != nil {
		if cfg.StatusServer.Port != nil && (*cfg.StatusServer.Port < 1 || *cfg.StatusServer.Port > 65535) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("statusServer", "port"), *cfg.StatusServer.Port, "must be between 1 and 65535"))
		}
		if cfg.StatusServer.QPS != nil && *cfg.StatusServer.QPS < 0 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("statusServer", "qps"), *cfg.StatusServer.QPS, "must be greater than or equal to 0"))
		}
		if cfg.StatusServer.Burst != nil && *cfg.StatusServer.Burst < 0 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("statusServer", "burst"), *cfg.StatusServer.Burst, "must be greater than or equal to 0"))
		}
	}

	if policy := cfg.NetworkPolicy; policy != nil {
		for i, rule := range policy.Ingress {
			allErrs = append(allErrs, validateNetworkPolicyRule(rule.Ports, rule.From, field.NewPath("networkPolicy", "ingress").Index(i), "from")...)
		}
		for i, rule := range policy.Egress {
			allErrs = append(allErrs, validateNetworkPolicyRule(rule.Ports, rule.To, field.NewPath("networkPolicy", "egress").Index(i), "to")...)
		}
		if len(policy.Ingress) > 128 {
			allErrs = append(allErrs, field.TooMany(field.NewPath("networkPolicy", "ingress"), len(policy.Ingress), 128))
		}
		if len(policy.Egress) > 128 {
			allErrs = append(allErrs, field.TooMany(field.NewPath("networkPolicy", "egress"), len(policy.Egress), 128))
		}
	}
	return allErrs
}

// validateNetworkPolicyRule checks administrator exceptions before the controller starts.
func validateNetworkPolicyRule(ports []networkingv1.NetworkPolicyPort, peers []networkingv1.NetworkPolicyPeer, path *field.Path, peerField string) field.ErrorList {
	var errs field.ErrorList
	for i, port := range ports {
		p := path.Child("ports").Index(i)
		if port.Protocol != nil && *port.Protocol != corev1.ProtocolTCP && *port.Protocol != corev1.ProtocolUDP && *port.Protocol != corev1.ProtocolSCTP {
			errs = append(errs, field.NotSupported(p.Child("protocol"), *port.Protocol, []string{"TCP", "UDP", "SCTP"}))
		}
		if port.Port != nil {
			var messages []string
			if port.Port.Type == intstr.Int {
				messages = validation.IsValidPortNum(int(port.Port.IntVal))
			} else {
				messages = validation.IsValidPortName(port.Port.StrVal)
			}
			if len(messages) > 0 {
				errs = append(errs, field.Invalid(p.Child("port"), port.Port, strings.Join(messages, "; ")))
			}
		}
		if port.EndPort != nil {
			if port.Port == nil || port.Port.Type != intstr.Int {
				errs = append(errs, field.Invalid(p.Child("endPort"), *port.EndPort, "requires a numeric port"))
			} else if *port.EndPort < port.Port.IntVal {
				errs = append(errs, field.Invalid(p.Child("endPort"), *port.EndPort, "must be greater than or equal to port"))
			}
			if messages := validation.IsValidPortNum(int(*port.EndPort)); len(messages) > 0 {
				errs = append(errs, field.Invalid(p.Child("endPort"), *port.EndPort, strings.Join(messages, "; ")))
			}
		}
	}
	for i, peer := range peers {
		p := path.Child(peerField).Index(i)
		if peer.PodSelector != nil {
			errs = append(errs, metav1validation.ValidateLabelSelector(peer.PodSelector, metav1validation.LabelSelectorValidationOptions{}, p.Child("podSelector"))...)
		}
		if peer.NamespaceSelector != nil {
			errs = append(errs, metav1validation.ValidateLabelSelector(peer.NamespaceSelector, metav1validation.LabelSelectorValidationOptions{}, p.Child("namespaceSelector"))...)
		}
		if peer.IPBlock == nil {
			if peer.PodSelector == nil && peer.NamespaceSelector == nil {
				errs = append(errs, field.Required(p, "must specify a peer"))
			}
			continue
		}
		if peer.PodSelector != nil || peer.NamespaceSelector != nil {
			errs = append(errs, field.Forbidden(p, "may not combine ipBlock with selectors"))
		}
		block := peer.IPBlock
		p = p.Child("ipBlock")
		errs = append(errs, validation.IsValidCIDRForLegacyField(p.Child("cidr"), block.CIDR, true, nil)...)
		_, cidr, _ := netutils.ParseCIDRSloppy(block.CIDR)
		for j, exception := range block.Except {
			ep := p.Child("except").Index(j)
			errs = append(errs, validation.IsValidCIDRForLegacyField(ep, exception, true, nil)...)
			_, excluded, err := netutils.ParseCIDRSloppy(exception)
			if cidr != nil && err == nil {
				size, _ := cidr.Mask.Size()
				excludedSize, _ := excluded.Mask.Size()
				if !cidr.Contains(excluded.IP) || excludedSize <= size {
					errs = append(errs, field.Invalid(ep, exception, "must be a strict subset of cidr"))
				}
			}
		}
	}
	return errs
}
