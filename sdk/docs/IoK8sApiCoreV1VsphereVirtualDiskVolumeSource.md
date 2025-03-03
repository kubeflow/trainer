# IoK8sApiCoreV1VsphereVirtualDiskVolumeSource

Represents a vSphere volume resource.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**fs_type** | **str** | fsType is filesystem type to mount. Must be a filesystem type supported by the host operating system. Ex. \&quot;ext4\&quot;, \&quot;xfs\&quot;, \&quot;ntfs\&quot;. Implicitly inferred to be \&quot;ext4\&quot; if unspecified. | [optional] 
**storage_policy_id** | **str** | storagePolicyID is the storage Policy Based Management (SPBM) profile ID associated with the StoragePolicyName. | [optional] 
**storage_policy_name** | **str** | storagePolicyName is the storage Policy Based Management (SPBM) profile name. | [optional] 
**volume_path** | **str** | volumePath is the path that identifies vSphere volume vmdk | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_vsphere_virtual_disk_volume_source import IoK8sApiCoreV1VsphereVirtualDiskVolumeSource

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1VsphereVirtualDiskVolumeSource from a JSON string
io_k8s_api_core_v1_vsphere_virtual_disk_volume_source_instance = IoK8sApiCoreV1VsphereVirtualDiskVolumeSource.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1VsphereVirtualDiskVolumeSource.to_json())

# convert the object into a dict
io_k8s_api_core_v1_vsphere_virtual_disk_volume_source_dict = io_k8s_api_core_v1_vsphere_virtual_disk_volume_source_instance.to_dict()
# create an instance of IoK8sApiCoreV1VsphereVirtualDiskVolumeSource from a dict
io_k8s_api_core_v1_vsphere_virtual_disk_volume_source_from_dict = IoK8sApiCoreV1VsphereVirtualDiskVolumeSource.from_dict(io_k8s_api_core_v1_vsphere_virtual_disk_volume_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


