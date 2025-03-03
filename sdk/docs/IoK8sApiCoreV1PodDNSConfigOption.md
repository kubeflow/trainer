# IoK8sApiCoreV1PodDNSConfigOption

PodDNSConfigOption defines DNS resolver options of a pod.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**name** | **str** | Name is this DNS resolver option&#39;s name. Required. | [optional] 
**value** | **str** | Value is this DNS resolver option&#39;s value. | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_pod_dns_config_option import IoK8sApiCoreV1PodDNSConfigOption

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1PodDNSConfigOption from a JSON string
io_k8s_api_core_v1_pod_dns_config_option_instance = IoK8sApiCoreV1PodDNSConfigOption.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1PodDNSConfigOption.to_json())

# convert the object into a dict
io_k8s_api_core_v1_pod_dns_config_option_dict = io_k8s_api_core_v1_pod_dns_config_option_instance.to_dict()
# create an instance of IoK8sApiCoreV1PodDNSConfigOption from a dict
io_k8s_api_core_v1_pod_dns_config_option_from_dict = IoK8sApiCoreV1PodDNSConfigOption.from_dict(io_k8s_api_core_v1_pod_dns_config_option_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


