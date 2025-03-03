# IoK8sApiCoreV1EnvVar

EnvVar represents an environment variable present in a Container.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**name** | **str** | Name of the environment variable. Must be a C_IDENTIFIER. | 
**value** | **str** | Variable references $(VAR_NAME) are expanded using the previously defined environment variables in the container and any service environment variables. If a variable cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. \&quot;$$(VAR_NAME)\&quot; will produce the string literal \&quot;$(VAR_NAME)\&quot;. Escaped references will never be expanded, regardless of whether the variable exists or not. Defaults to \&quot;\&quot;. | [optional] 
**value_from** | [**IoK8sApiCoreV1EnvVarSource**](IoK8sApiCoreV1EnvVarSource.md) |  | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_env_var import IoK8sApiCoreV1EnvVar

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1EnvVar from a JSON string
io_k8s_api_core_v1_env_var_instance = IoK8sApiCoreV1EnvVar.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1EnvVar.to_json())

# convert the object into a dict
io_k8s_api_core_v1_env_var_dict = io_k8s_api_core_v1_env_var_instance.to_dict()
# create an instance of IoK8sApiCoreV1EnvVar from a dict
io_k8s_api_core_v1_env_var_from_dict = IoK8sApiCoreV1EnvVar.from_dict(io_k8s_api_core_v1_env_var_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


