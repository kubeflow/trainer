# IoK8sApiCoreV1RBDVolumeSource

Represents a Rados Block Device mount that lasts the lifetime of a pod. RBD volumes support ownership management and SELinux relabeling.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**fs_type** | **str** | fsType is the filesystem type of the volume that you want to mount. Tip: Ensure that the filesystem type is supported by the host operating system. Examples: \&quot;ext4\&quot;, \&quot;xfs\&quot;, \&quot;ntfs\&quot;. Implicitly inferred to be \&quot;ext4\&quot; if unspecified. More info: https://kubernetes.io/docs/concepts/storage/volumes#rbd | [optional] 
**image** | **str** | image is the rados image name. More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it | 
**keyring** | **str** | keyring is the path to key ring for RBDUser. Default is /etc/ceph/keyring. More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it | [optional] 
**monitors** | **List[str]** | monitors is a collection of Ceph monitors. More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it | 
**pool** | **str** | pool is the rados pool name. Default is rbd. More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it | [optional] 
**read_only** | **bool** | readOnly here will force the ReadOnly setting in VolumeMounts. Defaults to false. More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it | [optional] 
**secret_ref** | [**IoK8sApiCoreV1LocalObjectReference**](IoK8sApiCoreV1LocalObjectReference.md) |  | [optional] 
**user** | **str** | user is the rados user name. Default is admin. More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_rbd_volume_source import IoK8sApiCoreV1RBDVolumeSource

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1RBDVolumeSource from a JSON string
io_k8s_api_core_v1_rbd_volume_source_instance = IoK8sApiCoreV1RBDVolumeSource.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1RBDVolumeSource.to_json())

# convert the object into a dict
io_k8s_api_core_v1_rbd_volume_source_dict = io_k8s_api_core_v1_rbd_volume_source_instance.to_dict()
# create an instance of IoK8sApiCoreV1RBDVolumeSource from a dict
io_k8s_api_core_v1_rbd_volume_source_from_dict = IoK8sApiCoreV1RBDVolumeSource.from_dict(io_k8s_api_core_v1_rbd_volume_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


