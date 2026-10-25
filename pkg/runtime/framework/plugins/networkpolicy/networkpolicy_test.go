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
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	jobset "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/features"
	utiltesting "github.com/kubeflow/trainer/v2/pkg/util/testing"
)

func TestNetworkPolicyBuild(t *testing.T) {
	for name, tc := range map[string]struct {
		config  *configapi.Configuration
		jobName string
		count   int
	}{
		"omitted":             {jobName: "alpha"},
		"disabled":            {config: &configapi.Configuration{NetworkPolicy: &configapi.NetworkPolicyConfig{}}, jobName: "alpha"},
		"enabled":             {config: &configapi.Configuration{NetworkPolicy: &configapi.NetworkPolicyConfig{Enabled: ptr.To(true)}}, jobName: "alpha", count: 2},
		"long valid job name": {config: &configapi.Configuration{NetworkPolicy: &configapi.NetworkPolicyConfig{Enabled: ptr.To(true)}}, jobName: strings.Repeat("a", 63), count: 1},
	} {
		t.Run(name, func(t *testing.T) {
			c := utiltesting.NewClientBuilder().WithObjects(networkDependencies()...).Build()
			plugin, err := New(context.Background(), c, nil, tc.config)
			if err != nil {
				t.Fatal(err)
			}
			job := &trainer.TrainJob{ObjectMeta: metav1.ObjectMeta{Name: tc.jobName, Namespace: "team-a", UID: "job-uid"}}
			objects, err := plugin.(*NetworkPolicy).Build(context.Background(), nil, job)
			if err != nil {
				t.Fatal(err)
			}
			if len(objects) != tc.count {
				t.Fatalf("policies = %d, want %d", len(objects), tc.count)
			}
			typed, err := utiltesting.ToObject(c.Scheme(), objects...)
			if err != nil {
				t.Fatal(err)
			}
			peers := []networkingv1.NetworkPolicyPeer{
				{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{jobset.JobSetNameKey: job.Name}}},
			}
			if tc.count == 2 {
				peers = append(peers, networkingv1.NetworkPolicyPeer{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{cacheNameLabel: job.Name + "-cache"}}})
			}
			for i, object := range typed {
				policy := object.(*networkingv1.NetworkPolicy)
				if !metav1.IsControlledBy(policy, job) || policy.Namespace != job.Namespace {
					t.Fatal("policy has incorrect ownership or namespace")
				}
				want := networkingv1.NetworkPolicySpec{
					PodSelector: *peers[i].PodSelector,
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
					Ingress:     []networkingv1.NetworkPolicyIngressRule{{From: peers}},
					Egress:      []networkingv1.NetworkPolicyEgressRule{{To: peers}},
				}
				infrastructure, err := plugin.(*NetworkPolicy).infrastructureEgress(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				want.Egress = append(want.Egress, infrastructure...)
				if diff := cmp.Diff(want, policy.Spec); diff != "" {
					t.Fatal(diff)
				}
			}
		})
	}
}

func TestNetworkPolicyConfiguredRules(t *testing.T) {
	cfg := &configapi.Configuration{NetworkPolicy: &configapi.NetworkPolicyConfig{
		Enabled: ptr.To(true),
		Ingress: []networkingv1.NetworkPolicyIngressRule{{From: []networkingv1.NetworkPolicyPeer{{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"name": "monitoring"}}}}}},
		Egress:  []networkingv1.NetworkPolicyEgressRule{{To: []networkingv1.NetworkPolicyPeer{{IPBlock: &networkingv1.IPBlock{CIDR: "192.0.2.0/24"}}}}},
	}}
	c := utiltesting.NewClientBuilder().WithObjects(networkDependencies()...).Build()
	p, err := New(context.Background(), c, nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	// The plugin must not retain mutable pointers into the caller's configuration.
	cfg.NetworkPolicy.Egress[0].To[0].IPBlock.CIDR = "0.0.0.0/0"
	objects, err := p.(*NetworkPolicy).Build(context.Background(), nil, &trainer.TrainJob{ObjectMeta: metav1.ObjectMeta{Name: "alpha", Namespace: "team-a", UID: "uid"}})
	if err != nil {
		t.Fatal(err)
	}
	typed, err := utiltesting.ToObject(c.Scheme(), objects...)
	if err != nil {
		t.Fatal(err)
	}
	for _, object := range typed {
		policy := object.(*networkingv1.NetworkPolicy)
		if diff := cmp.Diff(cfg.NetworkPolicy.Ingress, policy.Spec.Ingress[1:]); diff != "" {
			t.Fatal(diff)
		}
		if got := policy.Spec.Egress[len(policy.Spec.Egress)-1].To[0].IPBlock.CIDR; got != "192.0.2.0/24" {
			t.Fatalf("egress CIDR = %s", got)
		}
	}
}

func TestNetworkPolicyOwnershipAndOptOut(t *testing.T) {
	job := &trainer.TrainJob{ObjectMeta: metav1.ObjectMeta{Name: "alpha", Namespace: "team-a", UID: "uid"}}
	c := utiltesting.NewClientBuilder().WithObjects(networkDependencies()...).Build()
	enabled := &NetworkPolicy{client: c, config: configapi.NetworkPolicyConfig{Enabled: ptr.To(true)}}
	objects, err := enabled.Build(context.Background(), nil, job)
	if err != nil {
		t.Fatal(err)
	}
	typed, err := utiltesting.ToObject(c.Scheme(), objects...)
	if err != nil {
		t.Fatal(err)
	}
	for _, object := range typed {
		if err := c.Create(context.Background(), object.(client.Object)); err != nil {
			t.Fatal(err)
		}
	}
	// Completion must not remove isolation while the job's pods still exist.
	job.Status.Conditions = []metav1.Condition{{Type: trainer.TrainJobComplete, Status: metav1.ConditionTrue}}
	if got, err := enabled.Delete(context.Background(), job); err != nil || len(got) != 0 {
		t.Fatalf("enabled cleanup: %v, %v", got, err)
	}
	disabled := &NetworkPolicy{client: c}
	got, err := disabled.Delete(context.Background(), job)
	if err != nil || len(got) != 2 {
		t.Fatalf("opt-out cleanup: %v, %v", got, err)
	}
	foreign := typed[0].(*networkingv1.NetworkPolicy)
	foreign.OwnerReferences = nil
	if err := c.Update(context.Background(), foreign); err != nil {
		t.Fatal(err)
	}
	if _, err := enabled.Build(context.Background(), nil, job); err == nil || err.Error() != "NetworkPolicy team-a/trainjob-uid is not owned by TrainJob alpha" {
		t.Fatalf("unexpected ownership error: %v", err)
	}
	got, err = disabled.Delete(context.Background(), job)
	if err != nil || len(got) != 1 {
		t.Fatalf("foreign policy must survive opt-out: %v, %v", got, err)
	}
	if err := c.Delete(context.Background(), typed[1].(client.Object)); err != nil {
		t.Fatal(err)
	}
	if got, err := disabled.Delete(context.Background(), job); err != nil || len(got) != 0 {
		t.Fatalf("missing policies: %v, %v", got, err)
	}
}

func TestNetworkPolicyClientErrors(t *testing.T) {
	denied := errors.New("forbidden")
	c := utiltesting.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
			return denied
		},
	}).Build()
	p := &NetworkPolicy{client: c, config: configapi.NetworkPolicyConfig{Enabled: ptr.To(true)}}
	job := &trainer.TrainJob{ObjectMeta: metav1.ObjectMeta{Name: "alpha", Namespace: "team-a", UID: "uid"}}
	if _, err := p.Build(context.Background(), nil, job); !errors.Is(err, denied) {
		t.Fatalf("build error = %v", err)
	}
	p.config.Enabled = ptr.To(false)
	if _, err := p.Delete(context.Background(), job); !errors.Is(err, denied) {
		t.Fatalf("delete error = %v", err)
	}
}

func networkDependencies() []client.Object {
	return []client.Object{
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "kubernetes"}, Spec: corev1.ServiceSpec{ClusterIPs: []string{"10.96.0.1", "fd00::1"}, Ports: []corev1.ServicePort{{Name: "https", Port: 443}}}},
		&discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "kubernetes", Labels: map[string]string{discoveryv1.LabelServiceName: "kubernetes"}}, AddressType: discoveryv1.AddressTypeIPv4, Ports: []discoveryv1.EndpointPort{{Name: ptr.To("https"), Port: ptr.To[int32](6443)}}, Endpoints: []discoveryv1.Endpoint{{Addresses: []string{"192.0.2.10"}}, {Addresses: []string{"192.0.2.11"}, Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(false)}}}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: "kubeflow-system", Name: "custom-trainer"}, Spec: corev1.ServiceSpec{ClusterIP: "10.96.0.2", Selector: map[string]string{"app": "custom-trainer"}, Ports: []corev1.ServicePort{{Name: "status-server", Port: 10443}, {Name: "webhook", Port: 443}}}},
		&discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{Namespace: "kubeflow-system", Name: "custom-trainer", Labels: map[string]string{discoveryv1.LabelServiceName: "custom-trainer"}}, AddressType: discoveryv1.AddressTypeIPv4, Ports: []discoveryv1.EndpointPort{{Name: ptr.To("status-server"), Port: ptr.To[int32](10443)}, {Name: ptr.To("webhook"), Port: ptr.To[int32](9443)}}, Endpoints: []discoveryv1.Endpoint{{Addresses: []string{"10.244.0.10"}}}},
	}
}

func TestNetworkPolicyInfrastructureEgress(t *testing.T) {
	c := utiltesting.NewClientBuilder().WithObjects(networkDependencies()...).Build()
	p := &NetworkPolicy{client: c, statusService: "custom-trainer"}
	rules, err := p.infrastructureEgress(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []networkingv1.NetworkPolicyEgressRule{
		{To: []networkingv1.NetworkPolicyPeer{{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": "kube-system"}}, PodSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "k8s-app", Operator: metav1.LabelSelectorOpIn, Values: []string{"kube-dns", "node-local-dns"}}}}}}, Ports: []networkingv1.NetworkPolicyPort{{Protocol: ptr.To(corev1.ProtocolUDP), Port: ptr.To(intstr.FromInt32(53))}, {Protocol: ptr.To(corev1.ProtocolTCP), Port: ptr.To(intstr.FromInt32(53))}}},
	}
	for _, item := range []struct {
		cidrs []string
		port  int32
	}{
		{[]string{"10.96.0.1/32", "fd00::1/128"}, 443}, {[]string{"192.0.2.10/32"}, 6443},
	} {
		rule := networkingv1.NetworkPolicyEgressRule{Ports: []networkingv1.NetworkPolicyPort{{Protocol: ptr.To(corev1.ProtocolTCP), Port: ptr.To(intstr.FromInt32(item.port))}}}
		for _, cidr := range item.cidrs {
			rule.To = append(rule.To, networkingv1.NetworkPolicyPeer{IPBlock: &networkingv1.IPBlock{CIDR: cidr}})
		}
		want = append(want, rule)
	}
	want = append(want, networkingv1.NetworkPolicyEgressRule{To: []networkingv1.NetworkPolicyPeer{{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": "kubeflow-system"}}, PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "custom-trainer"}}}}, Ports: []networkingv1.NetworkPolicyPort{{Protocol: ptr.To(corev1.ProtocolTCP), Port: ptr.To(intstr.FromInt32(10443))}}})
	if diff := cmp.Diff(want, rules); diff != "" {
		t.Fatal(diff)
	}
}

func TestNetworkPolicyServiceDiscoveryErrors(t *testing.T) {
	for name, interceptors := range map[string]interceptor.Funcs{
		"get": {Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
			return errors.New("get failed")
		}},
		"list": {List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
			return errors.New("list failed")
		}},
	} {
		t.Run(name, func(t *testing.T) {
			c := utiltesting.NewClientBuilder().WithObjects(networkDependencies()...).WithInterceptorFuncs(interceptors).Build()
			p := &NetworkPolicy{client: c}
			if _, err := p.serviceEgress(context.Background(), client.ObjectKey{Namespace: "default", Name: "kubernetes"}, "https"); err == nil || !strings.Contains(err.Error(), name+" failed") {
				t.Fatal(err)
			}
		})
	}
}

func TestNetworkPolicyInfrastructureChanged(t *testing.T) {
	job := &trainer.TrainJob{ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "alpha"}}
	c := utiltesting.NewClientBuilder().WithObjects(job).Build()
	p := &NetworkPolicy{client: c, config: configapi.NetworkPolicyConfig{Enabled: ptr.To(true)}, statusService: "custom-trainer"}
	for _, object := range networkDependencies() {
		if got := p.infrastructureChanged(context.Background(), object); len(got) != 1 || got[0].NamespacedName != client.ObjectKeyFromObject(job) {
			t.Fatalf("requests for %T: %v", object, got)
		}
	}
	if got := p.infrastructureChanged(context.Background(), &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "unrelated"}}); len(got) != 0 {
		t.Fatal(got)
	}
	p.config.Enabled = ptr.To(false)
	if got := p.infrastructureChanged(context.Background(), networkDependencies()[0]); len(got) != 0 {
		t.Fatal(got)
	}
}

func TestNetworkPolicyStatusService(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.TrainJobStatus, enabled)
			c := utiltesting.NewClientBuilder().WithObjects(networkDependencies()...).Build()
			plugin, err := New(context.Background(), c, nil, &configapi.Configuration{CertManagement: &configapi.CertManagement{WebhookServiceName: "custom-trainer"}})
			if err != nil {
				t.Fatal(err)
			}
			want := ""
			if enabled {
				want = "custom-trainer"
			}
			if got := plugin.(*NetworkPolicy).statusService; got != want {
				t.Fatalf("status service = %q, want %q", got, want)
			}
		})
	}
}
