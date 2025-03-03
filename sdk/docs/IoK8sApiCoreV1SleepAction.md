# IoK8sApiCoreV1SleepAction

SleepAction describes a \"sleep\" action.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**seconds** | **int** | Seconds is the number of seconds to sleep. | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_sleep_action import IoK8sApiCoreV1SleepAction

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1SleepAction from a JSON string
io_k8s_api_core_v1_sleep_action_instance = IoK8sApiCoreV1SleepAction.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1SleepAction.to_json())

# convert the object into a dict
io_k8s_api_core_v1_sleep_action_dict = io_k8s_api_core_v1_sleep_action_instance.to_dict()
# create an instance of IoK8sApiCoreV1SleepAction from a dict
io_k8s_api_core_v1_sleep_action_from_dict = IoK8sApiCoreV1SleepAction.from_dict(io_k8s_api_core_v1_sleep_action_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


