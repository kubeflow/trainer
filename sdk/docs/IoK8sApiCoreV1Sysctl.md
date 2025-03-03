# IoK8sApiCoreV1Sysctl

Sysctl defines a kernel parameter to be set

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**name** | **str** | Name of a property to set | 
**value** | **str** | Value of a property to set | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_sysctl import IoK8sApiCoreV1Sysctl

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1Sysctl from a JSON string
io_k8s_api_core_v1_sysctl_instance = IoK8sApiCoreV1Sysctl.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1Sysctl.to_json())

# convert the object into a dict
io_k8s_api_core_v1_sysctl_dict = io_k8s_api_core_v1_sysctl_instance.to_dict()
# create an instance of IoK8sApiCoreV1Sysctl from a dict
io_k8s_api_core_v1_sysctl_from_dict = IoK8sApiCoreV1Sysctl.from_dict(io_k8s_api_core_v1_sysctl_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


