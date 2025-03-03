# IoK8sApiCoreV1HostPathVolumeSource

Represents a host path mapped into a pod. Host path volumes do not support ownership management or SELinux relabeling.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**path** | **str** | path of the directory on the host. If the path is a symlink, it will follow the link to the real path. More info: https://kubernetes.io/docs/concepts/storage/volumes#hostpath | 
**type** | **str** | type for HostPath Volume Defaults to \&quot;\&quot; More info: https://kubernetes.io/docs/concepts/storage/volumes#hostpath | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_host_path_volume_source import IoK8sApiCoreV1HostPathVolumeSource

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1HostPathVolumeSource from a JSON string
io_k8s_api_core_v1_host_path_volume_source_instance = IoK8sApiCoreV1HostPathVolumeSource.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1HostPathVolumeSource.to_json())

# convert the object into a dict
io_k8s_api_core_v1_host_path_volume_source_dict = io_k8s_api_core_v1_host_path_volume_source_instance.to_dict()
# create an instance of IoK8sApiCoreV1HostPathVolumeSource from a dict
io_k8s_api_core_v1_host_path_volume_source_from_dict = IoK8sApiCoreV1HostPathVolumeSource.from_dict(io_k8s_api_core_v1_host_path_volume_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


