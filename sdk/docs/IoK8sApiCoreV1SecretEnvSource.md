# IoK8sApiCoreV1SecretEnvSource

SecretEnvSource selects a Secret to populate the environment variables with.  The contents of the target Secret's Data field will represent the key-value pairs as environment variables.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**name** | **str** | Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names | [optional] 
**optional** | **bool** | Specify whether the Secret must be defined | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_secret_env_source import IoK8sApiCoreV1SecretEnvSource

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1SecretEnvSource from a JSON string
io_k8s_api_core_v1_secret_env_source_instance = IoK8sApiCoreV1SecretEnvSource.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1SecretEnvSource.to_json())

# convert the object into a dict
io_k8s_api_core_v1_secret_env_source_dict = io_k8s_api_core_v1_secret_env_source_instance.to_dict()
# create an instance of IoK8sApiCoreV1SecretEnvSource from a dict
io_k8s_api_core_v1_secret_env_source_from_dict = IoK8sApiCoreV1SecretEnvSource.from_dict(io_k8s_api_core_v1_secret_env_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


