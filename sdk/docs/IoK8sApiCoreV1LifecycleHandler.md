# IoK8sApiCoreV1LifecycleHandler

LifecycleHandler defines a specific action that should be taken in a lifecycle hook. One and only one of the fields, except TCPSocket must be specified.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**var_exec** | [**IoK8sApiCoreV1ExecAction**](IoK8sApiCoreV1ExecAction.md) |  | [optional] 
**http_get** | [**IoK8sApiCoreV1HTTPGetAction**](IoK8sApiCoreV1HTTPGetAction.md) |  | [optional] 
**sleep** | [**IoK8sApiCoreV1SleepAction**](IoK8sApiCoreV1SleepAction.md) |  | [optional] 
**tcp_socket** | [**IoK8sApiCoreV1TCPSocketAction**](IoK8sApiCoreV1TCPSocketAction.md) |  | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_lifecycle_handler import IoK8sApiCoreV1LifecycleHandler

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1LifecycleHandler from a JSON string
io_k8s_api_core_v1_lifecycle_handler_instance = IoK8sApiCoreV1LifecycleHandler.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1LifecycleHandler.to_json())

# convert the object into a dict
io_k8s_api_core_v1_lifecycle_handler_dict = io_k8s_api_core_v1_lifecycle_handler_instance.to_dict()
# create an instance of IoK8sApiCoreV1LifecycleHandler from a dict
io_k8s_api_core_v1_lifecycle_handler_from_dict = IoK8sApiCoreV1LifecycleHandler.from_dict(io_k8s_api_core_v1_lifecycle_handler_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


