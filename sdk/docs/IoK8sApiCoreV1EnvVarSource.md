# IoK8sApiCoreV1EnvVarSource

EnvVarSource represents a source for the value of an EnvVar.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**config_map_key_ref** | [**IoK8sApiCoreV1ConfigMapKeySelector**](IoK8sApiCoreV1ConfigMapKeySelector.md) |  | [optional] 
**field_ref** | [**IoK8sApiCoreV1ObjectFieldSelector**](IoK8sApiCoreV1ObjectFieldSelector.md) |  | [optional] 
**resource_field_ref** | [**IoK8sApiCoreV1ResourceFieldSelector**](IoK8sApiCoreV1ResourceFieldSelector.md) |  | [optional] 
**secret_key_ref** | [**IoK8sApiCoreV1SecretKeySelector**](IoK8sApiCoreV1SecretKeySelector.md) |  | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_env_var_source import IoK8sApiCoreV1EnvVarSource

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1EnvVarSource from a JSON string
io_k8s_api_core_v1_env_var_source_instance = IoK8sApiCoreV1EnvVarSource.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1EnvVarSource.to_json())

# convert the object into a dict
io_k8s_api_core_v1_env_var_source_dict = io_k8s_api_core_v1_env_var_source_instance.to_dict()
# create an instance of IoK8sApiCoreV1EnvVarSource from a dict
io_k8s_api_core_v1_env_var_source_from_dict = IoK8sApiCoreV1EnvVarSource.from_dict(io_k8s_api_core_v1_env_var_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


