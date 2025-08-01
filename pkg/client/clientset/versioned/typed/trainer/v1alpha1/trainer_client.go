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

package v1alpha1

import (
	http "net/http"

	trainerv1alpha1 "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	scheme "github.com/kubeflow/trainer/v2/pkg/client/clientset/versioned/scheme"
	rest "k8s.io/client-go/rest"
)

type TrainerV1alpha1Interface interface {
	RESTClient() rest.Interface
	ClusterTrainingRuntimesGetter
	TrainJobsGetter
	TrainingRuntimesGetter
}

// TrainerV1alpha1Client is used to interact with features provided by the trainer.kubeflow.org group.
type TrainerV1alpha1Client struct {
	restClient rest.Interface
}

func (c *TrainerV1alpha1Client) ClusterTrainingRuntimes() ClusterTrainingRuntimeInterface {
	return newClusterTrainingRuntimes(c)
}

func (c *TrainerV1alpha1Client) TrainJobs(namespace string) TrainJobInterface {
	return newTrainJobs(c, namespace)
}

func (c *TrainerV1alpha1Client) TrainingRuntimes(namespace string) TrainingRuntimeInterface {
	return newTrainingRuntimes(c, namespace)
}

// NewForConfig creates a new TrainerV1alpha1Client for the given config.
// NewForConfig is equivalent to NewForConfigAndClient(c, httpClient),
// where httpClient was generated with rest.HTTPClientFor(c).
func NewForConfig(c *rest.Config) (*TrainerV1alpha1Client, error) {
	config := *c
	setConfigDefaults(&config)
	httpClient, err := rest.HTTPClientFor(&config)
	if err != nil {
		return nil, err
	}
	return NewForConfigAndClient(&config, httpClient)
}

// NewForConfigAndClient creates a new TrainerV1alpha1Client for the given config and http client.
// Note the http client provided takes precedence over the configured transport values.
func NewForConfigAndClient(c *rest.Config, h *http.Client) (*TrainerV1alpha1Client, error) {
	config := *c
	setConfigDefaults(&config)
	client, err := rest.RESTClientForConfigAndClient(&config, h)
	if err != nil {
		return nil, err
	}
	return &TrainerV1alpha1Client{client}, nil
}

// NewForConfigOrDie creates a new TrainerV1alpha1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *TrainerV1alpha1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new TrainerV1alpha1Client for the given RESTClient.
func New(c rest.Interface) *TrainerV1alpha1Client {
	return &TrainerV1alpha1Client{c}
}

func setConfigDefaults(config *rest.Config) {
	gv := trainerv1alpha1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = rest.CodecFactoryForGeneratedClient(scheme.Scheme, scheme.Codecs).WithoutConversion()

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *TrainerV1alpha1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
