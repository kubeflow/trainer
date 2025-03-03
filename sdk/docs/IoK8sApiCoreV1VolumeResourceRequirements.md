# IoK8sApiCoreV1VolumeResourceRequirements

VolumeResourceRequirements describes the storage resource requirements for a volume.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**limits** | **Dict[str, str]** | Limits describes the maximum amount of compute resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/ | [optional] 
**requests** | **Dict[str, str]** | Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. Requests cannot exceed Limits. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/ | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_volume_resource_requirements import IoK8sApiCoreV1VolumeResourceRequirements

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1VolumeResourceRequirements from a JSON string
io_k8s_api_core_v1_volume_resource_requirements_instance = IoK8sApiCoreV1VolumeResourceRequirements.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1VolumeResourceRequirements.to_json())

# convert the object into a dict
io_k8s_api_core_v1_volume_resource_requirements_dict = io_k8s_api_core_v1_volume_resource_requirements_instance.to_dict()
# create an instance of IoK8sApiCoreV1VolumeResourceRequirements from a dict
io_k8s_api_core_v1_volume_resource_requirements_from_dict = IoK8sApiCoreV1VolumeResourceRequirements.from_dict(io_k8s_api_core_v1_volume_resource_requirements_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


