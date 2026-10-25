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

	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	networkingv1ac "k8s.io/client-go/applyconfigurations/networking/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	jobset "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/apply"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
)

const Name = "NetworkPolicy"

// LWS sets this label on both leader and worker Pods, including existing cache clusters.
const cacheNameLabel = "leaderworkerset.sigs.k8s.io/name"

type NetworkPolicy struct {
	client client.Client
	config configapi.NetworkPolicyConfig
}

var _ framework.ComponentBuilderPlugin = (*NetworkPolicy)(nil)
var _ framework.ComponentDeleterPlugin = (*NetworkPolicy)(nil)
var _ framework.WatchExtensionPlugin = (*NetworkPolicy)(nil)

// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=create;get;list;watch;patch;delete

func New(_ context.Context, c client.Client, _ client.FieldIndexer, cfg *configapi.Configuration) (framework.Plugin, error) {
	p := &NetworkPolicy{client: c}
	if cfg != nil && cfg.NetworkPolicy != nil {
		p.config = *cfg.NetworkPolicy.DeepCopy()
	}
	return p, nil
}

func (p *NetworkPolicy) Name() string { return Name }

func (p *NetworkPolicy) ReconcilerBuilders() []runtime.ReconcilerBuilder {
	return []runtime.ReconcilerBuilder{
		func(b *builder.Builder, _ client.Client, _ cache.Cache) *builder.Builder {
			return b.Watches(&networkingv1.NetworkPolicy{}, handler.EnqueueRequestForOwner(
				p.client.Scheme(), p.client.RESTMapper(), &trainer.TrainJob{}, handler.OnlyControllerOwner()))
		},
	}
}

func (p *NetworkPolicy) Build(ctx context.Context, _ *runtime.Info, trainJob *trainer.TrainJob) ([]apiruntime.ApplyConfiguration, error) {
	if !ptr.Deref(p.config.Enabled, false) {
		return nil, nil
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
				Egress:      append([]networkingv1.NetworkPolicyEgressRule{{To: peers}}, p.config.Egress...),
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
