# IoK8sApiAutoscalingV2MetricSpec

MetricSpec specifies how to scale based on a single metric (only `type` and one other matching field should be set at once).

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**container_resource** | [**IoK8sApiAutoscalingV2ContainerResourceMetricSource**](IoK8sApiAutoscalingV2ContainerResourceMetricSource.md) |  | [optional] 
**external** | [**IoK8sApiAutoscalingV2ExternalMetricSource**](IoK8sApiAutoscalingV2ExternalMetricSource.md) |  | [optional] 
**object** | [**IoK8sApiAutoscalingV2ObjectMetricSource**](IoK8sApiAutoscalingV2ObjectMetricSource.md) |  | [optional] 
**pods** | [**IoK8sApiAutoscalingV2PodsMetricSource**](IoK8sApiAutoscalingV2PodsMetricSource.md) |  | [optional] 
**resource** | [**IoK8sApiAutoscalingV2ResourceMetricSource**](IoK8sApiAutoscalingV2ResourceMetricSource.md) |  | [optional] 
**type** | **str** | type is the type of metric source.  It should be one of \&quot;ContainerResource\&quot;, \&quot;External\&quot;, \&quot;Object\&quot;, \&quot;Pods\&quot; or \&quot;Resource\&quot;, each mapping to a matching field in the object. | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_autoscaling_v2_metric_spec import IoK8sApiAutoscalingV2MetricSpec

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiAutoscalingV2MetricSpec from a JSON string
io_k8s_api_autoscaling_v2_metric_spec_instance = IoK8sApiAutoscalingV2MetricSpec.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiAutoscalingV2MetricSpec.to_json())

# convert the object into a dict
io_k8s_api_autoscaling_v2_metric_spec_dict = io_k8s_api_autoscaling_v2_metric_spec_instance.to_dict()
# create an instance of IoK8sApiAutoscalingV2MetricSpec from a dict
io_k8s_api_autoscaling_v2_metric_spec_from_dict = IoK8sApiAutoscalingV2MetricSpec.from_dict(io_k8s_api_autoscaling_v2_metric_spec_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


