# IoK8sApiCoreV1TypedLocalObjectReference

TypedLocalObjectReference contains enough information to let you locate the typed referenced object inside the same namespace.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**api_group** | **str** | APIGroup is the group for the resource being referenced. If APIGroup is not specified, the specified Kind must be in the core API group. For any other third-party types, APIGroup is required. | [optional] 
**kind** | **str** | Kind is the type of resource being referenced | 
**name** | **str** | Name is the name of resource being referenced | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_typed_local_object_reference import IoK8sApiCoreV1TypedLocalObjectReference

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1TypedLocalObjectReference from a JSON string
io_k8s_api_core_v1_typed_local_object_reference_instance = IoK8sApiCoreV1TypedLocalObjectReference.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1TypedLocalObjectReference.to_json())

# convert the object into a dict
io_k8s_api_core_v1_typed_local_object_reference_dict = io_k8s_api_core_v1_typed_local_object_reference_instance.to_dict()
# create an instance of IoK8sApiCoreV1TypedLocalObjectReference from a dict
io_k8s_api_core_v1_typed_local_object_reference_from_dict = IoK8sApiCoreV1TypedLocalObjectReference.from_dict(io_k8s_api_core_v1_typed_local_object_reference_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


