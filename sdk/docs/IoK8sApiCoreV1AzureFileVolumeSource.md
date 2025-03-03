# IoK8sApiCoreV1AzureFileVolumeSource

AzureFile represents an Azure File Service mount on the host and bind mount to the pod.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**read_only** | **bool** | readOnly defaults to false (read/write). ReadOnly here will force the ReadOnly setting in VolumeMounts. | [optional] 
**secret_name** | **str** | secretName is the  name of secret that contains Azure Storage Account Name and Key | 
**share_name** | **str** | shareName is the azure share Name | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_azure_file_volume_source import IoK8sApiCoreV1AzureFileVolumeSource

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1AzureFileVolumeSource from a JSON string
io_k8s_api_core_v1_azure_file_volume_source_instance = IoK8sApiCoreV1AzureFileVolumeSource.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1AzureFileVolumeSource.to_json())

# convert the object into a dict
io_k8s_api_core_v1_azure_file_volume_source_dict = io_k8s_api_core_v1_azure_file_volume_source_instance.to_dict()
# create an instance of IoK8sApiCoreV1AzureFileVolumeSource from a dict
io_k8s_api_core_v1_azure_file_volume_source_from_dict = IoK8sApiCoreV1AzureFileVolumeSource.from_dict(io_k8s_api_core_v1_azure_file_volume_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


