/*
Copyright 2024 The Kubeflow Authors.

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

package runtime

import (
	"encoding/json"
	"maps"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
)

// resourceRequirementsPatchMeta defines the strategic merge patch strategy for ResourceRequirements.
// Claims are merged by name, matching the Kubernetes strategic merge patch semantics requested by Andrey.
type resourceRequirementsPatchMeta struct {
	Limits   corev1.ResourceList    `json:"limits,omitempty"`
	Requests corev1.ResourceList    `json:"requests,omitempty"`
	Claims   []corev1.ResourceClaim `json:"claims,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
}

// MergeResourceRequirements overlays override onto base using strategic merge patch.
// Limits and requests merge per-key; claims merge by name (patchMergeKey:"name").
func MergeResourceRequirements(base, override corev1.ResourceRequirements) (corev1.ResourceRequirements, error) {
	src, err := json.Marshal(base)
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}
	patch, err := json.Marshal(override)
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}
	merged, err := strategicpatch.StrategicMergePatch(src, patch, resourceRequirementsPatchMeta{})
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}
	var out corev1.ResourceRequirements
	if err := json.Unmarshal(merged, &out); err != nil {
		return corev1.ResourceRequirements{}, err
	}
	return out, nil
}

// ResourceRequirementsFromApply converts an apply configuration back to a typed ResourceRequirements.
func ResourceRequirementsFromApply(res *corev1ac.ResourceRequirementsApplyConfiguration) corev1.ResourceRequirements {
	if res == nil {
		return corev1.ResourceRequirements{}
	}
	out := corev1.ResourceRequirements{}
	if res.Limits != nil {
		out.Limits = maps.Clone(*res.Limits)
	}
	if res.Requests != nil {
		out.Requests = maps.Clone(*res.Requests)
	}
	if res.Claims != nil {
		out.Claims = make([]corev1.ResourceClaim, 0, len(res.Claims))
		for i := range res.Claims {
			claim := res.Claims[i]
			if claim.Name == nil || claim.Request == nil {
				continue
			}
			out.Claims = append(out.Claims, corev1.ResourceClaim{
				Name:    *claim.Name,
				Request: *claim.Request,
			})
		}
	}
	return out
}

// ToResourceRequirementsApplyConfiguration converts typed ResourceRequirements to apply configuration.
func ToResourceRequirementsApplyConfiguration(res corev1.ResourceRequirements) *corev1ac.ResourceRequirementsApplyConfiguration {
	requirements := corev1ac.ResourceRequirements()
	if limits := res.Limits; limits != nil {
		requirements.WithLimits(maps.Clone(limits))
	}
	if requests := res.Requests; requests != nil {
		requirements.WithRequests(maps.Clone(requests))
	}
	if claims := res.Claims; len(claims) > 0 {
		claimApplies := make([]*corev1ac.ResourceClaimApplyConfiguration, 0, len(claims))
		for i := range claims {
			claim := claims[i]
			claimApplies = append(claimApplies, corev1ac.ResourceClaim().
				WithName(claim.Name).
				WithRequest(claim.Request))
		}
		requirements.WithClaims(claimApplies...)
	}
	return requirements
}
