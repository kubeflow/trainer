# IoK8sApiAutoscalingV2MetricIdentifier

MetricIdentifier defines the name and optionally selector for a metric

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**name** | **str** | name is the name of the given metric | 
**selector** | [**IoK8sApimachineryPkgApisMetaV1LabelSelector**](IoK8sApimachineryPkgApisMetaV1LabelSelector.md) |  | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_autoscaling_v2_metric_identifier import IoK8sApiAutoscalingV2MetricIdentifier

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiAutoscalingV2MetricIdentifier from a JSON string
io_k8s_api_autoscaling_v2_metric_identifier_instance = IoK8sApiAutoscalingV2MetricIdentifier.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiAutoscalingV2MetricIdentifier.to_json())

# convert the object into a dict
io_k8s_api_autoscaling_v2_metric_identifier_dict = io_k8s_api_autoscaling_v2_metric_identifier_instance.to_dict()
# create an instance of IoK8sApiAutoscalingV2MetricIdentifier from a dict
io_k8s_api_autoscaling_v2_metric_identifier_from_dict = IoK8sApiAutoscalingV2MetricIdentifier.from_dict(io_k8s_api_autoscaling_v2_metric_identifier_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


