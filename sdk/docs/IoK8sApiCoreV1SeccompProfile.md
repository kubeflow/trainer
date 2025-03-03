# IoK8sApiCoreV1SeccompProfile

SeccompProfile defines a pod/container's seccomp profile settings. Only one profile source may be set.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**localhost_profile** | **str** | localhostProfile indicates a profile defined in a file on the node should be used. The profile must be preconfigured on the node to work. Must be a descending path, relative to the kubelet&#39;s configured seccomp profile location. Must be set if type is \&quot;Localhost\&quot;. Must NOT be set for any other type. | [optional] 
**type** | **str** | type indicates which kind of seccomp profile will be applied. Valid options are:  Localhost - a profile defined in a file on the node should be used. RuntimeDefault - the container runtime default profile should be used. Unconfined - no profile should be applied. | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_seccomp_profile import IoK8sApiCoreV1SeccompProfile

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1SeccompProfile from a JSON string
io_k8s_api_core_v1_seccomp_profile_instance = IoK8sApiCoreV1SeccompProfile.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1SeccompProfile.to_json())

# convert the object into a dict
io_k8s_api_core_v1_seccomp_profile_dict = io_k8s_api_core_v1_seccomp_profile_instance.to_dict()
# create an instance of IoK8sApiCoreV1SeccompProfile from a dict
io_k8s_api_core_v1_seccomp_profile_from_dict = IoK8sApiCoreV1SeccompProfile.from_dict(io_k8s_api_core_v1_seccomp_profile_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


