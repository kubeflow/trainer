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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v2alpha1 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v2alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeTrainingRuntimes implements TrainingRuntimeInterface
type FakeTrainingRuntimes struct {
	Fake *FakeKubeflowV2alpha1
	ns   string
}

var trainingruntimesResource = v2alpha1.SchemeGroupVersion.WithResource("trainingruntimes")

var trainingruntimesKind = v2alpha1.SchemeGroupVersion.WithKind("TrainingRuntime")

// Get takes name of the trainingRuntime, and returns the corresponding trainingRuntime object, and an error if there is any.
func (c *FakeTrainingRuntimes) Get(ctx context.Context, name string, options v1.GetOptions) (result *v2alpha1.TrainingRuntime, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(trainingruntimesResource, c.ns, name), &v2alpha1.TrainingRuntime{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v2alpha1.TrainingRuntime), err
}

// List takes label and field selectors, and returns the list of TrainingRuntimes that match those selectors.
func (c *FakeTrainingRuntimes) List(ctx context.Context, opts v1.ListOptions) (result *v2alpha1.TrainingRuntimeList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(trainingruntimesResource, trainingruntimesKind, c.ns, opts), &v2alpha1.TrainingRuntimeList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v2alpha1.TrainingRuntimeList{ListMeta: obj.(*v2alpha1.TrainingRuntimeList).ListMeta}
	for _, item := range obj.(*v2alpha1.TrainingRuntimeList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested trainingRuntimes.
func (c *FakeTrainingRuntimes) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(trainingruntimesResource, c.ns, opts))

}

// Create takes the representation of a trainingRuntime and creates it.  Returns the server's representation of the trainingRuntime, and an error, if there is any.
func (c *FakeTrainingRuntimes) Create(ctx context.Context, trainingRuntime *v2alpha1.TrainingRuntime, opts v1.CreateOptions) (result *v2alpha1.TrainingRuntime, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(trainingruntimesResource, c.ns, trainingRuntime), &v2alpha1.TrainingRuntime{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v2alpha1.TrainingRuntime), err
}

// Update takes the representation of a trainingRuntime and updates it. Returns the server's representation of the trainingRuntime, and an error, if there is any.
func (c *FakeTrainingRuntimes) Update(ctx context.Context, trainingRuntime *v2alpha1.TrainingRuntime, opts v1.UpdateOptions) (result *v2alpha1.TrainingRuntime, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(trainingruntimesResource, c.ns, trainingRuntime), &v2alpha1.TrainingRuntime{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v2alpha1.TrainingRuntime), err
}

// Delete takes name of the trainingRuntime and deletes it. Returns an error if one occurs.
func (c *FakeTrainingRuntimes) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(trainingruntimesResource, c.ns, name, opts), &v2alpha1.TrainingRuntime{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeTrainingRuntimes) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(trainingruntimesResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v2alpha1.TrainingRuntimeList{})
	return err
}

// Patch applies the patch and returns the patched trainingRuntime.
func (c *FakeTrainingRuntimes) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v2alpha1.TrainingRuntime, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(trainingruntimesResource, c.ns, name, pt, data, subresources...), &v2alpha1.TrainingRuntime{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v2alpha1.TrainingRuntime), err
}
