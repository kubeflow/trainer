# IoK8sApiCoreV1PersistentVolumeClaimTemplate

PersistentVolumeClaimTemplate is used to produce PersistentVolumeClaim objects as part of an EphemeralVolumeSource.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**metadata** | [**IoK8sApimachineryPkgApisMetaV1ObjectMeta**](IoK8sApimachineryPkgApisMetaV1ObjectMeta.md) |  | [optional] 
**spec** | [**IoK8sApiCoreV1PersistentVolumeClaimSpec**](IoK8sApiCoreV1PersistentVolumeClaimSpec.md) |  | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_persistent_volume_claim_template import IoK8sApiCoreV1PersistentVolumeClaimTemplate

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1PersistentVolumeClaimTemplate from a JSON string
io_k8s_api_core_v1_persistent_volume_claim_template_instance = IoK8sApiCoreV1PersistentVolumeClaimTemplate.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1PersistentVolumeClaimTemplate.to_json())

# convert the object into a dict
io_k8s_api_core_v1_persistent_volume_claim_template_dict = io_k8s_api_core_v1_persistent_volume_claim_template_instance.to_dict()
# create an instance of IoK8sApiCoreV1PersistentVolumeClaimTemplate from a dict
io_k8s_api_core_v1_persistent_volume_claim_template_from_dict = IoK8sApiCoreV1PersistentVolumeClaimTemplate.from_dict(io_k8s_api_core_v1_persistent_volume_claim_template_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


