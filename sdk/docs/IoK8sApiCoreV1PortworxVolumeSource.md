# IoK8sApiCoreV1PortworxVolumeSource

PortworxVolumeSource represents a Portworx volume resource.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**fs_type** | **str** | fSType represents the filesystem type to mount Must be a filesystem type supported by the host operating system. Ex. \&quot;ext4\&quot;, \&quot;xfs\&quot;. Implicitly inferred to be \&quot;ext4\&quot; if unspecified. | [optional] 
**read_only** | **bool** | readOnly defaults to false (read/write). ReadOnly here will force the ReadOnly setting in VolumeMounts. | [optional] 
**volume_id** | **str** | volumeID uniquely identifies a Portworx volume | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_portworx_volume_source import IoK8sApiCoreV1PortworxVolumeSource

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1PortworxVolumeSource from a JSON string
io_k8s_api_core_v1_portworx_volume_source_instance = IoK8sApiCoreV1PortworxVolumeSource.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1PortworxVolumeSource.to_json())

# convert the object into a dict
io_k8s_api_core_v1_portworx_volume_source_dict = io_k8s_api_core_v1_portworx_volume_source_instance.to_dict()
# create an instance of IoK8sApiCoreV1PortworxVolumeSource from a dict
io_k8s_api_core_v1_portworx_volume_source_from_dict = IoK8sApiCoreV1PortworxVolumeSource.from_dict(io_k8s_api_core_v1_portworx_volume_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


