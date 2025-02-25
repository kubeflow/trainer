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

package testing

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
)

func NewClientBuilder(addToSchemes ...func(s *runtime.Scheme) error) *fake.ClientBuilder {
	scm := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scm))
	utilruntime.Must(trainer.AddToScheme(scm))
	utilruntime.Must(jobsetv1alpha2.AddToScheme(scm))
	utilruntime.Must(schedulerpluginsv1alpha1.AddToScheme(scm))
	for i := range addToSchemes {
		utilruntime.Must(addToSchemes[i](scm))
	}
	return fake.NewClientBuilder().
		WithScheme(scm)
}

type builderIndexer struct {
	*fake.ClientBuilder
}

var _ client.FieldIndexer = (*builderIndexer)(nil)

func (b *builderIndexer) IndexField(_ context.Context, obj client.Object, field string, extractValue client.IndexerFunc) error {
	if obj == nil || field == "" || extractValue == nil {
		return fmt.Errorf("error from test indexer")
	}
	b.ClientBuilder = b.ClientBuilder.WithIndex(obj, field, extractValue)
	return nil
}

func AsIndex(builder *fake.ClientBuilder) client.FieldIndexer {
	return &builderIndexer{ClientBuilder: builder}
}
