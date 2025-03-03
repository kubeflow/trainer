# IoK8sApiCoreV1DownwardAPIProjection

Represents downward API info for projecting into a projected volume. Note that this is identical to a downwardAPI volume source without the default mode.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**items** | [**List[IoK8sApiCoreV1DownwardAPIVolumeFile]**](IoK8sApiCoreV1DownwardAPIVolumeFile.md) | Items is a list of DownwardAPIVolume file | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_downward_api_projection import IoK8sApiCoreV1DownwardAPIProjection

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1DownwardAPIProjection from a JSON string
io_k8s_api_core_v1_downward_api_projection_instance = IoK8sApiCoreV1DownwardAPIProjection.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1DownwardAPIProjection.to_json())

# convert the object into a dict
io_k8s_api_core_v1_downward_api_projection_dict = io_k8s_api_core_v1_downward_api_projection_instance.to_dict()
# create an instance of IoK8sApiCoreV1DownwardAPIProjection from a dict
io_k8s_api_core_v1_downward_api_projection_from_dict = IoK8sApiCoreV1DownwardAPIProjection.from_dict(io_k8s_api_core_v1_downward_api_projection_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


