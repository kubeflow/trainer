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
	json "encoding/json"
	"fmt"

	v2alpha1 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v2alpha1"
	kubefloworgv2alpha1 "github.com/kubeflow/training-operator/pkg/client/applyconfiguration/kubeflow.org/v2alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeTrainJobs implements TrainJobInterface
type FakeTrainJobs struct {
	Fake *FakeKubeflowV2alpha1
	ns   string
}

var trainjobsResource = v2alpha1.SchemeGroupVersion.WithResource("trainjobs")

var trainjobsKind = v2alpha1.SchemeGroupVersion.WithKind("TrainJob")

// Get takes name of the trainJob, and returns the corresponding trainJob object, and an error if there is any.
func (c *FakeTrainJobs) Get(ctx context.Context, name string, options v1.GetOptions) (result *v2alpha1.TrainJob, err error) {
	emptyResult := &v2alpha1.TrainJob{}
	obj, err := c.Fake.
		Invokes(testing.NewGetActionWithOptions(trainjobsResource, c.ns, name, options), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v2alpha1.TrainJob), err
}

// List takes label and field selectors, and returns the list of TrainJobs that match those selectors.
func (c *FakeTrainJobs) List(ctx context.Context, opts v1.ListOptions) (result *v2alpha1.TrainJobList, err error) {
	emptyResult := &v2alpha1.TrainJobList{}
	obj, err := c.Fake.
		Invokes(testing.NewListActionWithOptions(trainjobsResource, trainjobsKind, c.ns, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v2alpha1.TrainJobList{ListMeta: obj.(*v2alpha1.TrainJobList).ListMeta}
	for _, item := range obj.(*v2alpha1.TrainJobList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested trainJobs.
func (c *FakeTrainJobs) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchActionWithOptions(trainjobsResource, c.ns, opts))

}

// Create takes the representation of a trainJob and creates it.  Returns the server's representation of the trainJob, and an error, if there is any.
func (c *FakeTrainJobs) Create(ctx context.Context, trainJob *v2alpha1.TrainJob, opts v1.CreateOptions) (result *v2alpha1.TrainJob, err error) {
	emptyResult := &v2alpha1.TrainJob{}
	obj, err := c.Fake.
		Invokes(testing.NewCreateActionWithOptions(trainjobsResource, c.ns, trainJob, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v2alpha1.TrainJob), err
}

// Update takes the representation of a trainJob and updates it. Returns the server's representation of the trainJob, and an error, if there is any.
func (c *FakeTrainJobs) Update(ctx context.Context, trainJob *v2alpha1.TrainJob, opts v1.UpdateOptions) (result *v2alpha1.TrainJob, err error) {
	emptyResult := &v2alpha1.TrainJob{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateActionWithOptions(trainjobsResource, c.ns, trainJob, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v2alpha1.TrainJob), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeTrainJobs) UpdateStatus(ctx context.Context, trainJob *v2alpha1.TrainJob, opts v1.UpdateOptions) (result *v2alpha1.TrainJob, err error) {
	emptyResult := &v2alpha1.TrainJob{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceActionWithOptions(trainjobsResource, "status", c.ns, trainJob, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v2alpha1.TrainJob), err
}

// Delete takes name of the trainJob and deletes it. Returns an error if one occurs.
func (c *FakeTrainJobs) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(trainjobsResource, c.ns, name, opts), &v2alpha1.TrainJob{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeTrainJobs) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionActionWithOptions(trainjobsResource, c.ns, opts, listOpts)

	_, err := c.Fake.Invokes(action, &v2alpha1.TrainJobList{})
	return err
}

// Patch applies the patch and returns the patched trainJob.
func (c *FakeTrainJobs) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v2alpha1.TrainJob, err error) {
	emptyResult := &v2alpha1.TrainJob{}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceActionWithOptions(trainjobsResource, c.ns, name, pt, data, opts, subresources...), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v2alpha1.TrainJob), err
}

// Apply takes the given apply declarative configuration, applies it and returns the applied trainJob.
func (c *FakeTrainJobs) Apply(ctx context.Context, trainJob *kubefloworgv2alpha1.TrainJobApplyConfiguration, opts v1.ApplyOptions) (result *v2alpha1.TrainJob, err error) {
	if trainJob == nil {
		return nil, fmt.Errorf("trainJob provided to Apply must not be nil")
	}
	data, err := json.Marshal(trainJob)
	if err != nil {
		return nil, err
	}
	name := trainJob.Name
	if name == nil {
		return nil, fmt.Errorf("trainJob.Name must be provided to Apply")
	}
	emptyResult := &v2alpha1.TrainJob{}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceActionWithOptions(trainjobsResource, c.ns, *name, types.ApplyPatchType, data, opts.ToPatchOptions()), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v2alpha1.TrainJob), err
}

// ApplyStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating ApplyStatus().
func (c *FakeTrainJobs) ApplyStatus(ctx context.Context, trainJob *kubefloworgv2alpha1.TrainJobApplyConfiguration, opts v1.ApplyOptions) (result *v2alpha1.TrainJob, err error) {
	if trainJob == nil {
		return nil, fmt.Errorf("trainJob provided to Apply must not be nil")
	}
	data, err := json.Marshal(trainJob)
	if err != nil {
		return nil, err
	}
	name := trainJob.Name
	if name == nil {
		return nil, fmt.Errorf("trainJob.Name must be provided to Apply")
	}
	emptyResult := &v2alpha1.TrainJob{}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceActionWithOptions(trainjobsResource, c.ns, *name, types.ApplyPatchType, data, opts.ToPatchOptions(), "status"), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v2alpha1.TrainJob), err
}
