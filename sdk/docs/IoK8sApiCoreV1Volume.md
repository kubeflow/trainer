# IoK8sApiCoreV1Volume

Volume represents a named volume in a pod that may be accessed by any container in the pod.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**aws_elastic_block_store** | [**IoK8sApiCoreV1AWSElasticBlockStoreVolumeSource**](IoK8sApiCoreV1AWSElasticBlockStoreVolumeSource.md) |  | [optional] 
**azure_disk** | [**IoK8sApiCoreV1AzureDiskVolumeSource**](IoK8sApiCoreV1AzureDiskVolumeSource.md) |  | [optional] 
**azure_file** | [**IoK8sApiCoreV1AzureFileVolumeSource**](IoK8sApiCoreV1AzureFileVolumeSource.md) |  | [optional] 
**cephfs** | [**IoK8sApiCoreV1CephFSVolumeSource**](IoK8sApiCoreV1CephFSVolumeSource.md) |  | [optional] 
**cinder** | [**IoK8sApiCoreV1CinderVolumeSource**](IoK8sApiCoreV1CinderVolumeSource.md) |  | [optional] 
**config_map** | [**IoK8sApiCoreV1ConfigMapVolumeSource**](IoK8sApiCoreV1ConfigMapVolumeSource.md) |  | [optional] 
**csi** | [**IoK8sApiCoreV1CSIVolumeSource**](IoK8sApiCoreV1CSIVolumeSource.md) |  | [optional] 
**downward_api** | [**IoK8sApiCoreV1DownwardAPIVolumeSource**](IoK8sApiCoreV1DownwardAPIVolumeSource.md) |  | [optional] 
**empty_dir** | [**IoK8sApiCoreV1EmptyDirVolumeSource**](IoK8sApiCoreV1EmptyDirVolumeSource.md) |  | [optional] 
**ephemeral** | [**IoK8sApiCoreV1EphemeralVolumeSource**](IoK8sApiCoreV1EphemeralVolumeSource.md) |  | [optional] 
**fc** | [**IoK8sApiCoreV1FCVolumeSource**](IoK8sApiCoreV1FCVolumeSource.md) |  | [optional] 
**flex_volume** | [**IoK8sApiCoreV1FlexVolumeSource**](IoK8sApiCoreV1FlexVolumeSource.md) |  | [optional] 
**flocker** | [**IoK8sApiCoreV1FlockerVolumeSource**](IoK8sApiCoreV1FlockerVolumeSource.md) |  | [optional] 
**gce_persistent_disk** | [**IoK8sApiCoreV1GCEPersistentDiskVolumeSource**](IoK8sApiCoreV1GCEPersistentDiskVolumeSource.md) |  | [optional] 
**git_repo** | [**IoK8sApiCoreV1GitRepoVolumeSource**](IoK8sApiCoreV1GitRepoVolumeSource.md) |  | [optional] 
**glusterfs** | [**IoK8sApiCoreV1GlusterfsVolumeSource**](IoK8sApiCoreV1GlusterfsVolumeSource.md) |  | [optional] 
**host_path** | [**IoK8sApiCoreV1HostPathVolumeSource**](IoK8sApiCoreV1HostPathVolumeSource.md) |  | [optional] 
**image** | [**IoK8sApiCoreV1ImageVolumeSource**](IoK8sApiCoreV1ImageVolumeSource.md) |  | [optional] 
**iscsi** | [**IoK8sApiCoreV1ISCSIVolumeSource**](IoK8sApiCoreV1ISCSIVolumeSource.md) |  | [optional] 
**name** | **str** | name of the volume. Must be a DNS_LABEL and unique within the pod. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names | 
**nfs** | [**IoK8sApiCoreV1NFSVolumeSource**](IoK8sApiCoreV1NFSVolumeSource.md) |  | [optional] 
**persistent_volume_claim** | [**IoK8sApiCoreV1PersistentVolumeClaimVolumeSource**](IoK8sApiCoreV1PersistentVolumeClaimVolumeSource.md) |  | [optional] 
**photon_persistent_disk** | [**IoK8sApiCoreV1PhotonPersistentDiskVolumeSource**](IoK8sApiCoreV1PhotonPersistentDiskVolumeSource.md) |  | [optional] 
**portworx_volume** | [**IoK8sApiCoreV1PortworxVolumeSource**](IoK8sApiCoreV1PortworxVolumeSource.md) |  | [optional] 
**projected** | [**IoK8sApiCoreV1ProjectedVolumeSource**](IoK8sApiCoreV1ProjectedVolumeSource.md) |  | [optional] 
**quobyte** | [**IoK8sApiCoreV1QuobyteVolumeSource**](IoK8sApiCoreV1QuobyteVolumeSource.md) |  | [optional] 
**rbd** | [**IoK8sApiCoreV1RBDVolumeSource**](IoK8sApiCoreV1RBDVolumeSource.md) |  | [optional] 
**scale_io** | [**IoK8sApiCoreV1ScaleIOVolumeSource**](IoK8sApiCoreV1ScaleIOVolumeSource.md) |  | [optional] 
**secret** | [**IoK8sApiCoreV1SecretVolumeSource**](IoK8sApiCoreV1SecretVolumeSource.md) |  | [optional] 
**storageos** | [**IoK8sApiCoreV1StorageOSVolumeSource**](IoK8sApiCoreV1StorageOSVolumeSource.md) |  | [optional] 
**vsphere_volume** | [**IoK8sApiCoreV1VsphereVirtualDiskVolumeSource**](IoK8sApiCoreV1VsphereVirtualDiskVolumeSource.md) |  | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_volume import IoK8sApiCoreV1Volume

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1Volume from a JSON string
io_k8s_api_core_v1_volume_instance = IoK8sApiCoreV1Volume.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1Volume.to_json())

# convert the object into a dict
io_k8s_api_core_v1_volume_dict = io_k8s_api_core_v1_volume_instance.to_dict()
# create an instance of IoK8sApiCoreV1Volume from a dict
io_k8s_api_core_v1_volume_from_dict = IoK8sApiCoreV1Volume.from_dict(io_k8s_api_core_v1_volume_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


