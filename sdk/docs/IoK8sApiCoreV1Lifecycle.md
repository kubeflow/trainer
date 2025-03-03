# IoK8sApiCoreV1Lifecycle

Lifecycle describes actions that the management system should take in response to container lifecycle events. For the PostStart and PreStop lifecycle handlers, management of the container blocks until the action is complete, unless the container process fails, in which case the handler is aborted.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**post_start** | [**IoK8sApiCoreV1LifecycleHandler**](IoK8sApiCoreV1LifecycleHandler.md) |  | [optional] 
**pre_stop** | [**IoK8sApiCoreV1LifecycleHandler**](IoK8sApiCoreV1LifecycleHandler.md) |  | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_lifecycle import IoK8sApiCoreV1Lifecycle

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1Lifecycle from a JSON string
io_k8s_api_core_v1_lifecycle_instance = IoK8sApiCoreV1Lifecycle.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1Lifecycle.to_json())

# convert the object into a dict
io_k8s_api_core_v1_lifecycle_dict = io_k8s_api_core_v1_lifecycle_instance.to_dict()
# create an instance of IoK8sApiCoreV1Lifecycle from a dict
io_k8s_api_core_v1_lifecycle_from_dict = IoK8sApiCoreV1Lifecycle.from_dict(io_k8s_api_core_v1_lifecycle_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


