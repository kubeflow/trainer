# IoK8sApiBatchV1PodFailurePolicyOnPodConditionsPattern

PodFailurePolicyOnPodConditionsPattern describes a pattern for matching an actual pod condition type.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**status** | **str** | Specifies the required Pod condition status. To match a pod condition it is required that the specified status equals the pod condition status. Defaults to True. | 
**type** | **str** | Specifies the required Pod condition type. To match a pod condition it is required that specified type equals the pod condition type. | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_batch_v1_pod_failure_policy_on_pod_conditions_pattern import IoK8sApiBatchV1PodFailurePolicyOnPodConditionsPattern

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiBatchV1PodFailurePolicyOnPodConditionsPattern from a JSON string
io_k8s_api_batch_v1_pod_failure_policy_on_pod_conditions_pattern_instance = IoK8sApiBatchV1PodFailurePolicyOnPodConditionsPattern.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiBatchV1PodFailurePolicyOnPodConditionsPattern.to_json())

# convert the object into a dict
io_k8s_api_batch_v1_pod_failure_policy_on_pod_conditions_pattern_dict = io_k8s_api_batch_v1_pod_failure_policy_on_pod_conditions_pattern_instance.to_dict()
# create an instance of IoK8sApiBatchV1PodFailurePolicyOnPodConditionsPattern from a dict
io_k8s_api_batch_v1_pod_failure_policy_on_pod_conditions_pattern_from_dict = IoK8sApiBatchV1PodFailurePolicyOnPodConditionsPattern.from_dict(io_k8s_api_batch_v1_pod_failure_policy_on_pod_conditions_pattern_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


