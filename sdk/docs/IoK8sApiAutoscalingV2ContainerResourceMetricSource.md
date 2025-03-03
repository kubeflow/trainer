# IoK8sApiAutoscalingV2ContainerResourceMetricSource

ContainerResourceMetricSource indicates how to scale on a resource metric known to Kubernetes, as specified in requests and limits, describing each pod in the current scale target (e.g. CPU or memory).  The values will be averaged together before being compared to the target.  Such metrics are built in to Kubernetes, and have special scaling options on top of those available to normal per-pod metrics using the \"pods\" source.  Only one \"target\" type should be set.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**container** | **str** | container is the name of the container in the pods of the scaling target | 
**name** | **str** | name is the name of the resource in question. | 
**target** | [**IoK8sApiAutoscalingV2MetricTarget**](IoK8sApiAutoscalingV2MetricTarget.md) |  | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_autoscaling_v2_container_resource_metric_source import IoK8sApiAutoscalingV2ContainerResourceMetricSource

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiAutoscalingV2ContainerResourceMetricSource from a JSON string
io_k8s_api_autoscaling_v2_container_resource_metric_source_instance = IoK8sApiAutoscalingV2ContainerResourceMetricSource.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiAutoscalingV2ContainerResourceMetricSource.to_json())

# convert the object into a dict
io_k8s_api_autoscaling_v2_container_resource_metric_source_dict = io_k8s_api_autoscaling_v2_container_resource_metric_source_instance.to_dict()
# create an instance of IoK8sApiAutoscalingV2ContainerResourceMetricSource from a dict
io_k8s_api_autoscaling_v2_container_resource_metric_source_from_dict = IoK8sApiAutoscalingV2ContainerResourceMetricSource.from_dict(io_k8s_api_autoscaling_v2_container_resource_metric_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


