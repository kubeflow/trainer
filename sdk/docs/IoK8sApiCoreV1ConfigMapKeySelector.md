# IoK8sApiCoreV1ConfigMapKeySelector

Selects a key from a ConfigMap.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**key** | **str** | The key to select. | 
**name** | **str** | Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names | [optional] 
**optional** | **bool** | Specify whether the ConfigMap or its key must be defined | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_config_map_key_selector import IoK8sApiCoreV1ConfigMapKeySelector

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1ConfigMapKeySelector from a JSON string
io_k8s_api_core_v1_config_map_key_selector_instance = IoK8sApiCoreV1ConfigMapKeySelector.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1ConfigMapKeySelector.to_json())

# convert the object into a dict
io_k8s_api_core_v1_config_map_key_selector_dict = io_k8s_api_core_v1_config_map_key_selector_instance.to_dict()
# create an instance of IoK8sApiCoreV1ConfigMapKeySelector from a dict
io_k8s_api_core_v1_config_map_key_selector_from_dict = IoK8sApiCoreV1ConfigMapKeySelector.from_dict(io_k8s_api_core_v1_config_map_key_selector_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


