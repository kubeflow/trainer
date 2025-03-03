# IoK8sApiCoreV1VolumeDevice

volumeDevice describes a mapping of a raw block device within a container.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**device_path** | **str** | devicePath is the path inside of the container that the device will be mapped to. | 
**name** | **str** | name must match the name of a persistentVolumeClaim in the pod | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_volume_device import IoK8sApiCoreV1VolumeDevice

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1VolumeDevice from a JSON string
io_k8s_api_core_v1_volume_device_instance = IoK8sApiCoreV1VolumeDevice.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1VolumeDevice.to_json())

# convert the object into a dict
io_k8s_api_core_v1_volume_device_dict = io_k8s_api_core_v1_volume_device_instance.to_dict()
# create an instance of IoK8sApiCoreV1VolumeDevice from a dict
io_k8s_api_core_v1_volume_device_from_dict = IoK8sApiCoreV1VolumeDevice.from_dict(io_k8s_api_core_v1_volume_device_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


