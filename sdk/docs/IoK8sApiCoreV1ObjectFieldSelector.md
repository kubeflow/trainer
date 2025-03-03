# IoK8sApiCoreV1ObjectFieldSelector

ObjectFieldSelector selects an APIVersioned field of an object.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**api_version** | **str** | Version of the schema the FieldPath is written in terms of, defaults to \&quot;v1\&quot;. | [optional] 
**field_path** | **str** | Path of the field to select in the specified API version. | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_object_field_selector import IoK8sApiCoreV1ObjectFieldSelector

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1ObjectFieldSelector from a JSON string
io_k8s_api_core_v1_object_field_selector_instance = IoK8sApiCoreV1ObjectFieldSelector.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1ObjectFieldSelector.to_json())

# convert the object into a dict
io_k8s_api_core_v1_object_field_selector_dict = io_k8s_api_core_v1_object_field_selector_instance.to_dict()
# create an instance of IoK8sApiCoreV1ObjectFieldSelector from a dict
io_k8s_api_core_v1_object_field_selector_from_dict = IoK8sApiCoreV1ObjectFieldSelector.from_dict(io_k8s_api_core_v1_object_field_selector_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


