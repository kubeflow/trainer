# IoK8sApiCoreV1DownwardAPIVolumeSource

DownwardAPIVolumeSource represents a volume containing downward API info. Downward API volumes support ownership management and SELinux relabeling.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**default_mode** | **int** | Optional: mode bits to use on created files by default. Must be a Optional: mode bits used to set permissions on created files by default. Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511. YAML accepts both octal and decimal values, JSON requires decimal values for mode bits. Defaults to 0644. Directories within the path are not affected by this setting. This might be in conflict with other options that affect the file mode, like fsGroup, and the result can be other mode bits set. | [optional] 
**items** | [**List[IoK8sApiCoreV1DownwardAPIVolumeFile]**](IoK8sApiCoreV1DownwardAPIVolumeFile.md) | Items is a list of downward API volume file | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_downward_api_volume_source import IoK8sApiCoreV1DownwardAPIVolumeSource

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1DownwardAPIVolumeSource from a JSON string
io_k8s_api_core_v1_downward_api_volume_source_instance = IoK8sApiCoreV1DownwardAPIVolumeSource.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1DownwardAPIVolumeSource.to_json())

# convert the object into a dict
io_k8s_api_core_v1_downward_api_volume_source_dict = io_k8s_api_core_v1_downward_api_volume_source_instance.to_dict()
# create an instance of IoK8sApiCoreV1DownwardAPIVolumeSource from a dict
io_k8s_api_core_v1_downward_api_volume_source_from_dict = IoK8sApiCoreV1DownwardAPIVolumeSource.from_dict(io_k8s_api_core_v1_downward_api_volume_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


