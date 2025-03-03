# IoK8sApiCoreV1LocalObjectReference

LocalObjectReference contains enough information to let you locate the referenced object inside the same namespace.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**name** | **str** | Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_local_object_reference import IoK8sApiCoreV1LocalObjectReference

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1LocalObjectReference from a JSON string
io_k8s_api_core_v1_local_object_reference_instance = IoK8sApiCoreV1LocalObjectReference.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1LocalObjectReference.to_json())

# convert the object into a dict
io_k8s_api_core_v1_local_object_reference_dict = io_k8s_api_core_v1_local_object_reference_instance.to_dict()
# create an instance of IoK8sApiCoreV1LocalObjectReference from a dict
io_k8s_api_core_v1_local_object_reference_from_dict = IoK8sApiCoreV1LocalObjectReference.from_dict(io_k8s_api_core_v1_local_object_reference_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


