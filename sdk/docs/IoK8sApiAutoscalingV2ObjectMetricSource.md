# IoK8sApiAutoscalingV2ObjectMetricSource

ObjectMetricSource indicates how to scale on a metric describing a kubernetes object (for example, hits-per-second on an Ingress object).

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**described_object** | [**IoK8sApiAutoscalingV2CrossVersionObjectReference**](IoK8sApiAutoscalingV2CrossVersionObjectReference.md) |  | 
**metric** | [**IoK8sApiAutoscalingV2MetricIdentifier**](IoK8sApiAutoscalingV2MetricIdentifier.md) |  | 
**target** | [**IoK8sApiAutoscalingV2MetricTarget**](IoK8sApiAutoscalingV2MetricTarget.md) |  | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_autoscaling_v2_object_metric_source import IoK8sApiAutoscalingV2ObjectMetricSource

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiAutoscalingV2ObjectMetricSource from a JSON string
io_k8s_api_autoscaling_v2_object_metric_source_instance = IoK8sApiAutoscalingV2ObjectMetricSource.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiAutoscalingV2ObjectMetricSource.to_json())

# convert the object into a dict
io_k8s_api_autoscaling_v2_object_metric_source_dict = io_k8s_api_autoscaling_v2_object_metric_source_instance.to_dict()
# create an instance of IoK8sApiAutoscalingV2ObjectMetricSource from a dict
io_k8s_api_autoscaling_v2_object_metric_source_from_dict = IoK8sApiAutoscalingV2ObjectMetricSource.from_dict(io_k8s_api_autoscaling_v2_object_metric_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


