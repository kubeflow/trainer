# IoK8sApiAutoscalingV2PodsMetricSource

PodsMetricSource indicates how to scale on a metric describing each pod in the current scale target (for example, transactions-processed-per-second). The values will be averaged together before being compared to the target value.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**metric** | [**IoK8sApiAutoscalingV2MetricIdentifier**](IoK8sApiAutoscalingV2MetricIdentifier.md) |  | 
**target** | [**IoK8sApiAutoscalingV2MetricTarget**](IoK8sApiAutoscalingV2MetricTarget.md) |  | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_autoscaling_v2_pods_metric_source import IoK8sApiAutoscalingV2PodsMetricSource

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiAutoscalingV2PodsMetricSource from a JSON string
io_k8s_api_autoscaling_v2_pods_metric_source_instance = IoK8sApiAutoscalingV2PodsMetricSource.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiAutoscalingV2PodsMetricSource.to_json())

# convert the object into a dict
io_k8s_api_autoscaling_v2_pods_metric_source_dict = io_k8s_api_autoscaling_v2_pods_metric_source_instance.to_dict()
# create an instance of IoK8sApiAutoscalingV2PodsMetricSource from a dict
io_k8s_api_autoscaling_v2_pods_metric_source_from_dict = IoK8sApiAutoscalingV2PodsMetricSource.from_dict(io_k8s_api_autoscaling_v2_pods_metric_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


