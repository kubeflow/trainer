// Copyright 2024 The Kubeflow Authors
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

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	trainerv1alpha1 "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
	labels "k8s.io/apimachinery/pkg/labels"
	listers "k8s.io/client-go/listers"
	cache "k8s.io/client-go/tools/cache"
)

// ClusterTrainingRuntimeLister helps list ClusterTrainingRuntimes.
// All objects returned here must be treated as read-only.
type ClusterTrainingRuntimeLister interface {
	// List lists all ClusterTrainingRuntimes in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*trainerv1alpha1.ClusterTrainingRuntime, err error)
	// Get retrieves the ClusterTrainingRuntime from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*trainerv1alpha1.ClusterTrainingRuntime, error)
	ClusterTrainingRuntimeListerExpansion
}

// clusterTrainingRuntimeLister implements the ClusterTrainingRuntimeLister interface.
type clusterTrainingRuntimeLister struct {
	listers.ResourceIndexer[*trainerv1alpha1.ClusterTrainingRuntime]
}

// NewClusterTrainingRuntimeLister returns a new ClusterTrainingRuntimeLister.
func NewClusterTrainingRuntimeLister(indexer cache.Indexer) ClusterTrainingRuntimeLister {
	return &clusterTrainingRuntimeLister{listers.New[*trainerv1alpha1.ClusterTrainingRuntime](indexer, trainerv1alpha1.Resource("clustertrainingruntime"))}
}
