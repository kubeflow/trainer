# IoK8sApiCoreV1TCPSocketAction

TCPSocketAction describes an action based on opening a socket

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**host** | **str** | Optional: Host name to connect to, defaults to the pod IP. | [optional] 
**port** | **str** | IntOrString is a type that can hold an int32 or a string.  When used in JSON or YAML marshalling and unmarshalling, it produces or consumes the inner type.  This allows you to have, for example, a JSON field that can accept a name or number. | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_tcp_socket_action import IoK8sApiCoreV1TCPSocketAction

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1TCPSocketAction from a JSON string
io_k8s_api_core_v1_tcp_socket_action_instance = IoK8sApiCoreV1TCPSocketAction.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1TCPSocketAction.to_json())

# convert the object into a dict
io_k8s_api_core_v1_tcp_socket_action_dict = io_k8s_api_core_v1_tcp_socket_action_instance.to_dict()
# create an instance of IoK8sApiCoreV1TCPSocketAction from a dict
io_k8s_api_core_v1_tcp_socket_action_from_dict = IoK8sApiCoreV1TCPSocketAction.from_dict(io_k8s_api_core_v1_tcp_socket_action_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


