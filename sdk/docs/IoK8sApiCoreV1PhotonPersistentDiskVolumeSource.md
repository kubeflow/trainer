# IoK8sApiCoreV1PhotonPersistentDiskVolumeSource

Represents a Photon Controller persistent disk resource.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**fs_type** | **str** | fsType is the filesystem type to mount. Must be a filesystem type supported by the host operating system. Ex. \&quot;ext4\&quot;, \&quot;xfs\&quot;, \&quot;ntfs\&quot;. Implicitly inferred to be \&quot;ext4\&quot; if unspecified. | [optional] 
**pd_id** | **str** | pdID is the ID that identifies Photon Controller persistent disk | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_photon_persistent_disk_volume_source import IoK8sApiCoreV1PhotonPersistentDiskVolumeSource

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1PhotonPersistentDiskVolumeSource from a JSON string
io_k8s_api_core_v1_photon_persistent_disk_volume_source_instance = IoK8sApiCoreV1PhotonPersistentDiskVolumeSource.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1PhotonPersistentDiskVolumeSource.to_json())

# convert the object into a dict
io_k8s_api_core_v1_photon_persistent_disk_volume_source_dict = io_k8s_api_core_v1_photon_persistent_disk_volume_source_instance.to_dict()
# create an instance of IoK8sApiCoreV1PhotonPersistentDiskVolumeSource from a dict
io_k8s_api_core_v1_photon_persistent_disk_volume_source_from_dict = IoK8sApiCoreV1PhotonPersistentDiskVolumeSource.from_dict(io_k8s_api_core_v1_photon_persistent_disk_volume_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


