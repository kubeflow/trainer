# IoK8sApiCoreV1HTTPGetAction

HTTPGetAction describes an action based on HTTP Get requests.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**host** | **str** | Host name to connect to, defaults to the pod IP. You probably want to set \&quot;Host\&quot; in httpHeaders instead. | [optional] 
**http_headers** | [**List[IoK8sApiCoreV1HTTPHeader]**](IoK8sApiCoreV1HTTPHeader.md) | Custom headers to set in the request. HTTP allows repeated headers. | [optional] 
**path** | **str** | Path to access on the HTTP server. | [optional] 
**port** | **str** | IntOrString is a type that can hold an int32 or a string.  When used in JSON or YAML marshalling and unmarshalling, it produces or consumes the inner type.  This allows you to have, for example, a JSON field that can accept a name or number. | 
**scheme** | **str** | Scheme to use for connecting to the host. Defaults to HTTP. | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_http_get_action import IoK8sApiCoreV1HTTPGetAction

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1HTTPGetAction from a JSON string
io_k8s_api_core_v1_http_get_action_instance = IoK8sApiCoreV1HTTPGetAction.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1HTTPGetAction.to_json())

# convert the object into a dict
io_k8s_api_core_v1_http_get_action_dict = io_k8s_api_core_v1_http_get_action_instance.to_dict()
# create an instance of IoK8sApiCoreV1HTTPGetAction from a dict
io_k8s_api_core_v1_http_get_action_from_dict = IoK8sApiCoreV1HTTPGetAction.from_dict(io_k8s_api_core_v1_http_get_action_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


