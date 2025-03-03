# IoK8sApiCoreV1EphemeralVolumeSource

Represents an ephemeral volume that is handled by a normal storage driver.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**volume_claim_template** | [**IoK8sApiCoreV1PersistentVolumeClaimTemplate**](IoK8sApiCoreV1PersistentVolumeClaimTemplate.md) |  | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_ephemeral_volume_source import IoK8sApiCoreV1EphemeralVolumeSource

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1EphemeralVolumeSource from a JSON string
io_k8s_api_core_v1_ephemeral_volume_source_instance = IoK8sApiCoreV1EphemeralVolumeSource.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1EphemeralVolumeSource.to_json())

# convert the object into a dict
io_k8s_api_core_v1_ephemeral_volume_source_dict = io_k8s_api_core_v1_ephemeral_volume_source_instance.to_dict()
# create an instance of IoK8sApiCoreV1EphemeralVolumeSource from a dict
io_k8s_api_core_v1_ephemeral_volume_source_from_dict = IoK8sApiCoreV1EphemeralVolumeSource.from_dict(io_k8s_api_core_v1_ephemeral_volume_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


