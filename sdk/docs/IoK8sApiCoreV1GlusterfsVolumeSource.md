# IoK8sApiCoreV1GlusterfsVolumeSource

Represents a Glusterfs mount that lasts the lifetime of a pod. Glusterfs volumes do not support ownership management or SELinux relabeling.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**endpoints** | **str** | endpoints is the endpoint name that details Glusterfs topology. More info: https://examples.k8s.io/volumes/glusterfs/README.md#create-a-pod | 
**path** | **str** | path is the Glusterfs volume path. More info: https://examples.k8s.io/volumes/glusterfs/README.md#create-a-pod | 
**read_only** | **bool** | readOnly here will force the Glusterfs volume to be mounted with read-only permissions. Defaults to false. More info: https://examples.k8s.io/volumes/glusterfs/README.md#create-a-pod | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_glusterfs_volume_source import IoK8sApiCoreV1GlusterfsVolumeSource

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1GlusterfsVolumeSource from a JSON string
io_k8s_api_core_v1_glusterfs_volume_source_instance = IoK8sApiCoreV1GlusterfsVolumeSource.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1GlusterfsVolumeSource.to_json())

# convert the object into a dict
io_k8s_api_core_v1_glusterfs_volume_source_dict = io_k8s_api_core_v1_glusterfs_volume_source_instance.to_dict()
# create an instance of IoK8sApiCoreV1GlusterfsVolumeSource from a dict
io_k8s_api_core_v1_glusterfs_volume_source_from_dict = IoK8sApiCoreV1GlusterfsVolumeSource.from_dict(io_k8s_api_core_v1_glusterfs_volume_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


