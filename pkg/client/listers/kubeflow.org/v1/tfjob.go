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

package v1

import (
	v1 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/listers"
	"k8s.io/client-go/tools/cache"
)

// TFJobLister helps list TFJobs.
// All objects returned here must be treated as read-only.
type TFJobLister interface {
	// List lists all TFJobs in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.TFJob, err error)
	// TFJobs returns an object that can list and get TFJobs.
	TFJobs(namespace string) TFJobNamespaceLister
	TFJobListerExpansion
}

// tFJobLister implements the TFJobLister interface.
type tFJobLister struct {
	listers.ResourceIndexer[*v1.TFJob]
}

// NewTFJobLister returns a new TFJobLister.
func NewTFJobLister(indexer cache.Indexer) TFJobLister {
	return &tFJobLister{listers.New[*v1.TFJob](indexer, v1.Resource("tfjob"))}
}

// TFJobs returns an object that can list and get TFJobs.
func (s *tFJobLister) TFJobs(namespace string) TFJobNamespaceLister {
	return tFJobNamespaceLister{listers.NewNamespaced[*v1.TFJob](s.ResourceIndexer, namespace)}
}

// TFJobNamespaceLister helps list and get TFJobs.
// All objects returned here must be treated as read-only.
type TFJobNamespaceLister interface {
	// List lists all TFJobs in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.TFJob, err error)
	// Get retrieves the TFJob from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1.TFJob, error)
	TFJobNamespaceListerExpansion
}

// tFJobNamespaceLister implements the TFJobNamespaceLister
// interface.
type tFJobNamespaceLister struct {
	listers.ResourceIndexer[*v1.TFJob]
}
