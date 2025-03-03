# IoK8sApiCoreV1HostAlias

HostAlias holds the mapping between IP and hostnames that will be injected as an entry in the pod's hosts file.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**hostnames** | **List[str]** | Hostnames for the above IP address. | [optional] 
**ip** | **str** | IP address of the host file entry. | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_host_alias import IoK8sApiCoreV1HostAlias

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1HostAlias from a JSON string
io_k8s_api_core_v1_host_alias_instance = IoK8sApiCoreV1HostAlias.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1HostAlias.to_json())

# convert the object into a dict
io_k8s_api_core_v1_host_alias_dict = io_k8s_api_core_v1_host_alias_instance.to_dict()
# create an instance of IoK8sApiCoreV1HostAlias from a dict
io_k8s_api_core_v1_host_alias_from_dict = IoK8sApiCoreV1HostAlias.from_dict(io_k8s_api_core_v1_host_alias_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


