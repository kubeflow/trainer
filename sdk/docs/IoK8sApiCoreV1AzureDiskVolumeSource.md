# IoK8sApiCoreV1AzureDiskVolumeSource

AzureDisk represents an Azure Data Disk mount on the host and bind mount to the pod.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**caching_mode** | **str** | cachingMode is the Host Caching mode: None, Read Only, Read Write. | [optional] 
**disk_name** | **str** | diskName is the Name of the data disk in the blob storage | 
**disk_uri** | **str** | diskURI is the URI of data disk in the blob storage | 
**fs_type** | **str** | fsType is Filesystem type to mount. Must be a filesystem type supported by the host operating system. Ex. \&quot;ext4\&quot;, \&quot;xfs\&quot;, \&quot;ntfs\&quot;. Implicitly inferred to be \&quot;ext4\&quot; if unspecified. | [optional] 
**kind** | **str** | kind expected values are Shared: multiple blob disks per storage account  Dedicated: single blob disk per storage account  Managed: azure managed data disk (only in managed availability set). defaults to shared | [optional] 
**read_only** | **bool** | readOnly Defaults to false (read/write). ReadOnly here will force the ReadOnly setting in VolumeMounts. | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_azure_disk_volume_source import IoK8sApiCoreV1AzureDiskVolumeSource

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1AzureDiskVolumeSource from a JSON string
io_k8s_api_core_v1_azure_disk_volume_source_instance = IoK8sApiCoreV1AzureDiskVolumeSource.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1AzureDiskVolumeSource.to_json())

# convert the object into a dict
io_k8s_api_core_v1_azure_disk_volume_source_dict = io_k8s_api_core_v1_azure_disk_volume_source_instance.to_dict()
# create an instance of IoK8sApiCoreV1AzureDiskVolumeSource from a dict
io_k8s_api_core_v1_azure_disk_volume_source_from_dict = IoK8sApiCoreV1AzureDiskVolumeSource.from_dict(io_k8s_api_core_v1_azure_disk_volume_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


