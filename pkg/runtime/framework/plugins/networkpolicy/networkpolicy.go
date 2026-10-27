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

package networkpolicy

import (
	"context"
	"fmt"
	"net/netip"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	networkingv1ac "k8s.io/client-go/applyconfigurations/networking/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	jobset "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/apply"
	"github.com/kubeflow/trainer/v2/pkg/features"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
	"github.com/kubeflow/trainer/v2/pkg/util/cert"
)

const Name = "NetworkPolicy"

// LWS sets this label on both leader and worker Pods, including existing cache clusters.
const cacheNameLabel = "leaderworkerset.sigs.k8s.io/name"

type NetworkPolicy struct {
	client        client.Client
	config        configapi.NetworkPolicyConfig
	statusService string
}

var _ framework.ComponentBuilderPlugin = (*NetworkPolicy)(nil)
var _ framework.ComponentDeleterPlugin = (*NetworkPolicy)(nil)
var _ framework.WatchExtensionPlugin = (*NetworkPolicy)(nil)

// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=create;get;list;watch;patch;delete

func New(_ context.Context, c client.Client, _ client.FieldIndexer, cfg *configapi.Configuration) (framework.Plugin, error) {
	p := &NetworkPolicy{client: c}
	if cfg != nil && cfg.NetworkPolicy != nil {
		p.config = *cfg.NetworkPolicy.DeepCopy()
	}
	if features.Enabled(features.TrainJobStatus) && cfg != nil && cfg.CertManagement != nil {
		p.statusService = cfg.CertManagement.WebhookServiceName
	}
	return p, nil
}

func (p *NetworkPolicy) Name() string { return Name }

func (p *NetworkPolicy) ReconcilerBuilders() []runtime.ReconcilerBuilder {
	return []runtime.ReconcilerBuilder{
		func(b *builder.Builder, _ client.Client, _ cache.Cache) *builder.Builder {
			b = b.Watches(&networkingv1.NetworkPolicy{}, handler.EnqueueRequestForOwner(
				p.client.Scheme(), p.client.RESTMapper(), &trainer.TrainJob{}, handler.OnlyControllerOwner()))
			if !ptr.Deref(p.config.Enabled, false) {
				return b
			}
			return b.Watches(&corev1.Service{}, handler.EnqueueRequestsFromMapFunc(p.infrastructureChanged)).
				Watches(&discoveryv1.EndpointSlice{}, handler.EnqueueRequestsFromMapFunc(p.infrastructureChanged))
		},
	}
}

func (p *NetworkPolicy) Build(ctx context.Context, _ *runtime.Info, trainJob *trainer.TrainJob) ([]apiruntime.ApplyConfiguration, error) {
	if !ptr.Deref(p.config.Enabled, false) {
		return nil, nil
	}
	egress, err := p.infrastructureEgress(ctx)
	if err != nil {
		return nil, err
	}
	// Reuse controller-provided labels so policies also cover already-running jobs and caches.
	peers := []networkingv1.NetworkPolicyPeer{
		{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{jobset.JobSetNameKey: trainJob.Name}}},
	}
	// LWS names must also fit into a Kubernetes label value.
	if len(trainJob.Name+"-cache") <= 63 {
		peers = append(peers, networkingv1.NetworkPolicyPeer{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{cacheNameLabel: trainJob.Name + "-cache"}}})
	}
	var objects []apiruntime.ApplyConfiguration
	for i, name := range policyNames(trainJob)[:len(peers)] {
		existing := &networkingv1.NetworkPolicy{}
		err := p.client.Get(ctx, client.ObjectKey{Namespace: trainJob.Namespace, Name: name}, existing)
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}
		if err == nil && !metav1.IsControlledBy(existing, trainJob) {
			return nil, fmt.Errorf("NetworkPolicy %s/%s is not owned by TrainJob %s", trainJob.Namespace, name, trainJob.Name)
		}
		policy := &networkingv1.NetworkPolicy{
			TypeMeta: metav1.TypeMeta{APIVersion: networkingv1.SchemeGroupVersion.String(), Kind: "NetworkPolicy"},
			ObjectMeta: metav1.ObjectMeta{
				Name: name, Namespace: trainJob.Namespace,
				OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(trainJob, trainer.GroupVersion.WithKind(trainer.TrainJobKind))},
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: *peers[i].PodSelector.DeepCopy(),
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
				Ingress:     append([]networkingv1.NetworkPolicyIngressRule{{From: peers}}, p.config.Ingress...),
				Egress:      append(append([]networkingv1.NetworkPolicyEgressRule{{To: peers}}, egress...), p.config.Egress...),
			},
		}
		obj, err := apply.FromTypedObjWithFields[networkingv1ac.NetworkPolicyApplyConfiguration](policy)
		if err != nil {
			return nil, err
		}
		objects = append(objects, obj)
	}
	return objects, nil
}

func (p *NetworkPolicy) Delete(ctx context.Context, trainJob *trainer.TrainJob) ([]client.Object, error) {
	if ptr.Deref(p.config.Enabled, false) {
		return nil, nil
	}
	var objects []client.Object
	for _, name := range policyNames(trainJob) {
		policy := &networkingv1.NetworkPolicy{}
		err := p.client.Get(ctx, client.ObjectKey{Namespace: trainJob.Namespace, Name: name}, policy)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if metav1.IsControlledBy(policy, trainJob) {
			objects = append(objects, policy)
		}
	}
	return objects, nil
}

func policyNames(job *trainer.TrainJob) []string {
	return []string{"trainjob-" + string(job.UID), "trainjob-cache-" + string(job.UID)}
}

func (p *NetworkPolicy) infrastructureChanged(ctx context.Context, object client.Object) []reconcile.Request {
	name := object.GetName()
	if _, ok := object.(*discoveryv1.EndpointSlice); ok {
		name = object.GetLabels()[discoveryv1.LabelServiceName]
	}
	apiService := object.GetNamespace() == "default" && name == "kubernetes"
	statusService := p.statusService != "" && object.GetNamespace() == cert.GetOperatorNamespace() && name == p.statusService
	if !ptr.Deref(p.config.Enabled, false) || !apiService && !statusService {
		return nil
	}
	jobs := &trainer.TrainJobList{}
	if err := p.client.List(ctx, jobs); err != nil {
		log.FromContext(ctx).Error(err, "Unable to enqueue TrainJobs after network dependency changed")
		return nil
	}
	requests := make([]reconcile.Request, 0, len(jobs.Items))
	for _, job := range jobs.Items {
		requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&job)})
	}
	return requests
}

func (p *NetworkPolicy) infrastructureEgress(ctx context.Context) ([]networkingv1.NetworkPolicyEgressRule, error) {
	rules := []networkingv1.NetworkPolicyEgressRule{{
		To: []networkingv1.NetworkPolicyPeer{{
			NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{corev1.LabelMetadataName: "kube-system"}},
			PodSelector:       &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "k8s-app", Operator: metav1.LabelSelectorOpIn, Values: []string{"kube-dns", "node-local-dns"}}}},
		}},
		Ports: []networkingv1.NetworkPolicyPort{{Protocol: ptr.To(corev1.ProtocolUDP), Port: ptr.To(intstr.FromInt32(53))}, {Protocol: ptr.To(corev1.ProtocolTCP), Port: ptr.To(intstr.FromInt32(53))}},
	}}
	api, err := p.serviceEgress(ctx, client.ObjectKey{Namespace: "default", Name: "kubernetes"}, "https")
	if err != nil {
		return nil, err
	}
	rules = append(rules, api...)
	if p.statusService != "" {
		status, err := p.serviceEgress(ctx, client.ObjectKey{Namespace: cert.GetOperatorNamespace(), Name: p.statusService}, "status-server")
		if err != nil {
			return nil, err
		}
		rules = append(rules, status...)
	}
	return rules, nil
}

// Include Service and endpoint addresses because CNIs apply policy before or after DNAT.
func (p *NetworkPolicy) serviceEgress(ctx context.Context, key client.ObjectKey, portName string) ([]networkingv1.NetworkPolicyEgressRule, error) {
	service := &corev1.Service{}
	if err := p.client.Get(ctx, key, service); err != nil {
		return nil, fmt.Errorf("discovering network dependency %s: %w", key, err)
	}
	slices := &discoveryv1.EndpointSliceList{}
	if err := p.client.List(ctx, slices, client.InNamespace(key.Namespace), client.MatchingLabels{discoveryv1.LabelServiceName: key.Name}); err != nil {
		return nil, err
	}
	var rules []networkingv1.NetworkPolicyEgressRule
	add := func(addresses []string, port int32) error {
		rule := networkingv1.NetworkPolicyEgressRule{Ports: []networkingv1.NetworkPolicyPort{{Protocol: ptr.To(corev1.ProtocolTCP), Port: ptr.To(intstr.FromInt32(port))}}}
		if len(service.Spec.Selector) > 0 {
			rule.To = []networkingv1.NetworkPolicyPeer{{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{corev1.LabelMetadataName: key.Namespace}}, PodSelector: &metav1.LabelSelector{MatchLabels: service.Spec.Selector}}}
		} else {
			for _, address := range addresses {
				ip, err := netip.ParseAddr(address)
				if err != nil {
					return fmt.Errorf("invalid network dependency address %q: %w", address, err)
				}
				rule.To = append(rule.To, networkingv1.NetworkPolicyPeer{IPBlock: &networkingv1.IPBlock{CIDR: netip.PrefixFrom(ip, ip.BitLen()).String()}})
			}
		}

		if len(rule.To) > 0 {
			rules = append(rules, rule)
		}
		return nil
	}
	ips := service.Spec.ClusterIPs
	if len(ips) == 0 && service.Spec.ClusterIP != "" && service.Spec.ClusterIP != corev1.ClusterIPNone {
		ips = []string{service.Spec.ClusterIP}
	}
	for _, port := range service.Spec.Ports {
		if port.Name == portName && len(service.Spec.Selector) == 0 {
			if err := add(ips, port.Port); err != nil {
				return nil, err
			}
		}
	}
	for _, slice := range slices.Items {
		for _, port := range slice.Ports {
			if ptr.Deref(port.Name, "") != portName || port.Port == nil || ptr.Deref(port.Protocol, corev1.ProtocolTCP) != corev1.ProtocolTCP {
				continue
			}
			if len(service.Spec.Selector) > 0 {
				if err := add(nil, *port.Port); err != nil {
					return nil, err
				}
				continue
			}
			for _, endpoint := range slice.Endpoints {
				if !ptr.Deref(endpoint.Conditions.Ready, true) {
					continue
				}
				if err := add(endpoint.Addresses, *port.Port); err != nil {
					return nil, err
				}
			}
		}
	}
	if len(rules) == 0 {
		return nil, fmt.Errorf("no %s destinations found for network dependency %s", portName, key)
	}
	return rules, nil
}
