# IoK8sApiCoreV1ConfigMapEnvSource

ConfigMapEnvSource selects a ConfigMap to populate the environment variables with.  The contents of the target ConfigMap's Data field will represent the key-value pairs as environment variables.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**name** | **str** | Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names | [optional] 
**optional** | **bool** | Specify whether the ConfigMap must be defined | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_config_map_env_source import IoK8sApiCoreV1ConfigMapEnvSource

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1ConfigMapEnvSource from a JSON string
io_k8s_api_core_v1_config_map_env_source_instance = IoK8sApiCoreV1ConfigMapEnvSource.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1ConfigMapEnvSource.to_json())

# convert the object into a dict
io_k8s_api_core_v1_config_map_env_source_dict = io_k8s_api_core_v1_config_map_env_source_instance.to_dict()
# create an instance of IoK8sApiCoreV1ConfigMapEnvSource from a dict
io_k8s_api_core_v1_config_map_env_source_from_dict = IoK8sApiCoreV1ConfigMapEnvSource.from_dict(io_k8s_api_core_v1_config_map_env_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


