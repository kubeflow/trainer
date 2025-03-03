# IoK8sApiCoreV1EnvFromSource

EnvFromSource represents the source of a set of ConfigMaps

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**config_map_ref** | [**IoK8sApiCoreV1ConfigMapEnvSource**](IoK8sApiCoreV1ConfigMapEnvSource.md) |  | [optional] 
**prefix** | **str** | An optional identifier to prepend to each key in the ConfigMap. Must be a C_IDENTIFIER. | [optional] 
**secret_ref** | [**IoK8sApiCoreV1SecretEnvSource**](IoK8sApiCoreV1SecretEnvSource.md) |  | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_env_from_source import IoK8sApiCoreV1EnvFromSource

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1EnvFromSource from a JSON string
io_k8s_api_core_v1_env_from_source_instance = IoK8sApiCoreV1EnvFromSource.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1EnvFromSource.to_json())

# convert the object into a dict
io_k8s_api_core_v1_env_from_source_dict = io_k8s_api_core_v1_env_from_source_instance.to_dict()
# create an instance of IoK8sApiCoreV1EnvFromSource from a dict
io_k8s_api_core_v1_env_from_source_from_dict = IoK8sApiCoreV1EnvFromSource.from_dict(io_k8s_api_core_v1_env_from_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


