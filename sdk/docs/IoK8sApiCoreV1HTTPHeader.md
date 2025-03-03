# IoK8sApiCoreV1HTTPHeader

HTTPHeader describes a custom header to be used in HTTP probes

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**name** | **str** | The header field name. This will be canonicalized upon output, so case-variant names will be understood as the same header. | 
**value** | **str** | The header field value | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_http_header import IoK8sApiCoreV1HTTPHeader

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1HTTPHeader from a JSON string
io_k8s_api_core_v1_http_header_instance = IoK8sApiCoreV1HTTPHeader.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1HTTPHeader.to_json())

# convert the object into a dict
io_k8s_api_core_v1_http_header_dict = io_k8s_api_core_v1_http_header_instance.to_dict()
# create an instance of IoK8sApiCoreV1HTTPHeader from a dict
io_k8s_api_core_v1_http_header_from_dict = IoK8sApiCoreV1HTTPHeader.from_dict(io_k8s_api_core_v1_http_header_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


