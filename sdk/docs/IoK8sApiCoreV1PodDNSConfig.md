# IoK8sApiCoreV1PodDNSConfig

PodDNSConfig defines the DNS parameters of a pod in addition to those generated from DNSPolicy.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**nameservers** | **List[str]** | A list of DNS name server IP addresses. This will be appended to the base nameservers generated from DNSPolicy. Duplicated nameservers will be removed. | [optional] 
**options** | [**List[IoK8sApiCoreV1PodDNSConfigOption]**](IoK8sApiCoreV1PodDNSConfigOption.md) | A list of DNS resolver options. This will be merged with the base options generated from DNSPolicy. Duplicated entries will be removed. Resolution options given in Options will override those that appear in the base DNSPolicy. | [optional] 
**searches** | **List[str]** | A list of DNS search domains for host-name lookup. This will be appended to the base search paths generated from DNSPolicy. Duplicated search paths will be removed. | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_pod_dns_config import IoK8sApiCoreV1PodDNSConfig

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1PodDNSConfig from a JSON string
io_k8s_api_core_v1_pod_dns_config_instance = IoK8sApiCoreV1PodDNSConfig.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1PodDNSConfig.to_json())

# convert the object into a dict
io_k8s_api_core_v1_pod_dns_config_dict = io_k8s_api_core_v1_pod_dns_config_instance.to_dict()
# create an instance of IoK8sApiCoreV1PodDNSConfig from a dict
io_k8s_api_core_v1_pod_dns_config_from_dict = IoK8sApiCoreV1PodDNSConfig.from_dict(io_k8s_api_core_v1_pod_dns_config_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


