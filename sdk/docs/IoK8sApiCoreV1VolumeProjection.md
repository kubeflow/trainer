# IoK8sApiCoreV1VolumeProjection

Projection that may be projected along with other supported volume types. Exactly one of these fields must be set.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**cluster_trust_bundle** | [**IoK8sApiCoreV1ClusterTrustBundleProjection**](IoK8sApiCoreV1ClusterTrustBundleProjection.md) |  | [optional] 
**config_map** | [**IoK8sApiCoreV1ConfigMapProjection**](IoK8sApiCoreV1ConfigMapProjection.md) |  | [optional] 
**downward_api** | [**IoK8sApiCoreV1DownwardAPIProjection**](IoK8sApiCoreV1DownwardAPIProjection.md) |  | [optional] 
**secret** | [**IoK8sApiCoreV1SecretProjection**](IoK8sApiCoreV1SecretProjection.md) |  | [optional] 
**service_account_token** | [**IoK8sApiCoreV1ServiceAccountTokenProjection**](IoK8sApiCoreV1ServiceAccountTokenProjection.md) |  | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_volume_projection import IoK8sApiCoreV1VolumeProjection

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1VolumeProjection from a JSON string
io_k8s_api_core_v1_volume_projection_instance = IoK8sApiCoreV1VolumeProjection.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1VolumeProjection.to_json())

# convert the object into a dict
io_k8s_api_core_v1_volume_projection_dict = io_k8s_api_core_v1_volume_projection_instance.to_dict()
# create an instance of IoK8sApiCoreV1VolumeProjection from a dict
io_k8s_api_core_v1_volume_projection_from_dict = IoK8sApiCoreV1VolumeProjection.from_dict(io_k8s_api_core_v1_volume_projection_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


