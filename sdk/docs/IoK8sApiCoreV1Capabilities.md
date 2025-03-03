# IoK8sApiCoreV1Capabilities

Adds and removes POSIX capabilities from running containers.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**add** | **List[str]** | Added capabilities | [optional] 
**drop** | **List[str]** | Removed capabilities | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_capabilities import IoK8sApiCoreV1Capabilities

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1Capabilities from a JSON string
io_k8s_api_core_v1_capabilities_instance = IoK8sApiCoreV1Capabilities.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1Capabilities.to_json())

# convert the object into a dict
io_k8s_api_core_v1_capabilities_dict = io_k8s_api_core_v1_capabilities_instance.to_dict()
# create an instance of IoK8sApiCoreV1Capabilities from a dict
io_k8s_api_core_v1_capabilities_from_dict = IoK8sApiCoreV1Capabilities.from_dict(io_k8s_api_core_v1_capabilities_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


