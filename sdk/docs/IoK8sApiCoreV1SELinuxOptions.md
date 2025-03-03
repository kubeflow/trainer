# IoK8sApiCoreV1SELinuxOptions

SELinuxOptions are the labels to be applied to the container

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**level** | **str** | Level is SELinux level label that applies to the container. | [optional] 
**role** | **str** | Role is a SELinux role label that applies to the container. | [optional] 
**type** | **str** | Type is a SELinux type label that applies to the container. | [optional] 
**user** | **str** | User is a SELinux user label that applies to the container. | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_se_linux_options import IoK8sApiCoreV1SELinuxOptions

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1SELinuxOptions from a JSON string
io_k8s_api_core_v1_se_linux_options_instance = IoK8sApiCoreV1SELinuxOptions.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1SELinuxOptions.to_json())

# convert the object into a dict
io_k8s_api_core_v1_se_linux_options_dict = io_k8s_api_core_v1_se_linux_options_instance.to_dict()
# create an instance of IoK8sApiCoreV1SELinuxOptions from a dict
io_k8s_api_core_v1_se_linux_options_from_dict = IoK8sApiCoreV1SELinuxOptions.from_dict(io_k8s_api_core_v1_se_linux_options_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


