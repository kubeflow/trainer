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

package core

import (
	"context"
	"errors"
	"time"

	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	configapi "github.com/kubeflow/trainer/v2/pkg/apis/config/v1alpha1"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/metrics"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
	fwkplugins "github.com/kubeflow/trainer/v2/pkg/runtime/framework/plugins"
	index "github.com/kubeflow/trainer/v2/pkg/runtime/indexer"
)

var errorTooManyTrainJobStatusPlugin = errors.New("too many TrainJobStatus plugins are registered")

type Framework struct {
	registry                     fwkplugins.Registry
	plugins                      map[string]framework.Plugin
	enforceMLPlugins             []framework.EnforceMLPolicyPlugin
	enforcePodGroupPolicyPlugins []framework.EnforcePodGroupPolicyPlugin
	customValidationPlugins      []framework.CustomValidationPlugin
	watchExtensionPlugins        []framework.WatchExtensionPlugin
	podNetworkPlugins            []framework.PodNetworkPlugin
	componentBuilderPlugins      []framework.ComponentBuilderPlugin
	trainJobStatusPlugin         framework.TrainJobStatusPlugin
}

func New(ctx context.Context, c client.Client, r fwkplugins.Registry, indexer client.FieldIndexer, cfg *configapi.Configuration) (*Framework, error) {
	f := &Framework{
		registry: r,
	}
	plugins := make(map[string]framework.Plugin, len(r))
	if err := f.SetupRuntimeClassIndexer(ctx, indexer); err != nil {
		return nil, err
	}

	for name, factory := range r {
		plugin, err := factory(ctx, c, indexer, cfg)
		if err != nil {
			return nil, err
		}
		plugins[name] = plugin
		if p, ok := plugin.(framework.EnforceMLPolicyPlugin); ok {
			f.enforceMLPlugins = append(f.enforceMLPlugins, p)
		}
		if p, ok := plugin.(framework.EnforcePodGroupPolicyPlugin); ok {
			f.enforcePodGroupPolicyPlugins = append(f.enforcePodGroupPolicyPlugins, p)
		}
		if p, ok := plugin.(framework.CustomValidationPlugin); ok {
			f.customValidationPlugins = append(f.customValidationPlugins, p)
		}
		if p, ok := plugin.(framework.WatchExtensionPlugin); ok {
			f.watchExtensionPlugins = append(f.watchExtensionPlugins, p)
		}
		if p, ok := plugin.(framework.PodNetworkPlugin); ok {
			f.podNetworkPlugins = append(f.podNetworkPlugins, p)
		}
		if p, ok := plugin.(framework.ComponentBuilderPlugin); ok {
			f.componentBuilderPlugins = append(f.componentBuilderPlugins, p)
		}
		if p, ok := plugin.(framework.TrainJobStatusPlugin); ok {
			if f.trainJobStatusPlugin != nil {
				return nil, errorTooManyTrainJobStatusPlugin
			}
			f.trainJobStatusPlugin = p
		}
	}
	f.plugins = plugins
	return f, nil
}

func (f *Framework) RunEnforceMLPolicyPlugins(info *runtime.Info, trainJob *trainer.TrainJob) error {
	for _, plugin := range f.enforceMLPlugins {
		start := time.Now()
		err := plugin.EnforceMLPolicy(info, trainJob)
		metrics.ObservePlugin(plugin.Name(), "enforce_ml_policy", time.Since(start), err)
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *Framework) RunEnforcePodGroupPolicyPlugins(info *runtime.Info, trainJob *trainer.TrainJob) error {
	for _, plugin := range f.enforcePodGroupPolicyPlugins {
		start := time.Now()
		err := plugin.EnforcePodGroupPolicy(info, trainJob)
		metrics.ObservePlugin(plugin.Name(), "enforce_pod_group_policy", time.Since(start), err)
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *Framework) RunCustomValidationPlugins(ctx context.Context, info *runtime.Info, oldObj, newObj *trainer.TrainJob) (admission.Warnings, field.ErrorList) {
	var aggregatedWarnings admission.Warnings
	var aggregatedErrors field.ErrorList
	for _, plugin := range f.customValidationPlugins {
		start := time.Now()
		warnings, errs := plugin.Validate(ctx, info, oldObj, newObj)
		var pluginErr error
		if len(errs) > 0 {
			pluginErr = errs.ToAggregate()
		}
		metrics.ObservePlugin(plugin.Name(), "validate", time.Since(start), pluginErr)
		if len(warnings) != 0 {
			aggregatedWarnings = append(aggregatedWarnings, warnings...)
		}
		if errs != nil {
			aggregatedErrors = append(aggregatedErrors, errs...)
		}
	}
	return aggregatedWarnings, aggregatedErrors
}

func (f *Framework) RunPodNetworkPlugins(info *runtime.Info, trainJob *trainer.TrainJob) error {
	for _, plugin := range f.podNetworkPlugins {
		start := time.Now()
		err := plugin.IdentifyPodNetwork(info, trainJob)
		metrics.ObservePlugin(plugin.Name(), "pod_network", time.Since(start), err)
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *Framework) RunComponentBuilderPlugins(ctx context.Context, info *runtime.Info, trainJob *trainer.TrainJob) ([]apiruntime.ApplyConfiguration, error) {
	var objs []apiruntime.ApplyConfiguration
	for _, plugin := range f.componentBuilderPlugins {
		start := time.Now()
		components, err := plugin.Build(ctx, info, trainJob)
		metrics.ObservePlugin(plugin.Name(), "build", time.Since(start), err)
		if err != nil {
			return nil, err
		}
		objs = append(objs, components...)
	}
	return objs, nil
}

func (f *Framework) RunTrainJobStatusPlugin(ctx context.Context, trainJob *trainer.TrainJob) (*trainer.TrainJobStatus, error) {
	if f.trainJobStatusPlugin != nil {
		start := time.Now()
		status, err := f.trainJobStatusPlugin.Status(ctx, trainJob)
		metrics.ObservePlugin(f.trainJobStatusPlugin.Name(), "status", time.Since(start), err)
		return status, err
	}
	return nil, nil
}

func (f *Framework) WatchExtensionPlugins() []framework.WatchExtensionPlugin {
	return f.watchExtensionPlugins
}

func (f *Framework) SetupRuntimeClassIndexer(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &trainer.TrainingRuntime{},
		index.TrainingRuntimeContainerRuntimeClassKey,
		index.IndexTrainingRuntimeContainerRuntimeClass); err != nil {
		return index.ErrorCanNotSetupTrainingRuntimeRuntimeClassIndexer
	}
	if err := indexer.IndexField(ctx, &trainer.ClusterTrainingRuntime{},
		index.ClusterTrainingRuntimeContainerRuntimeClassKey,
		index.IndexClusterTrainingRuntimeContainerRuntimeClass); err != nil {
		return index.ErrorCanNotSetupClusterTrainingRuntimeRuntimeClassIndexer
	}
	return nil
}
