# IoK8sApiCoreV1ResourceRequirements

ResourceRequirements describes the compute resource requirements.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**claims** | [**List[IoK8sApiCoreV1ResourceClaim]**](IoK8sApiCoreV1ResourceClaim.md) | Claims lists the names of resources, defined in spec.resourceClaims, that are used by this container.  This is an alpha field and requires enabling the DynamicResourceAllocation feature gate.  This field is immutable. It can only be set for containers. | [optional] 
**limits** | **Dict[str, str]** | Limits describes the maximum amount of compute resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/ | [optional] 
**requests** | **Dict[str, str]** | Requests describes the minimum amount of compute resources required. If Requests is omitted for a container, it defaults to Limits if that is explicitly specified, otherwise to an implementation-defined value. Requests cannot exceed Limits. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/ | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_resource_requirements import IoK8sApiCoreV1ResourceRequirements

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1ResourceRequirements from a JSON string
io_k8s_api_core_v1_resource_requirements_instance = IoK8sApiCoreV1ResourceRequirements.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1ResourceRequirements.to_json())

# convert the object into a dict
io_k8s_api_core_v1_resource_requirements_dict = io_k8s_api_core_v1_resource_requirements_instance.to_dict()
# create an instance of IoK8sApiCoreV1ResourceRequirements from a dict
io_k8s_api_core_v1_resource_requirements_from_dict = IoK8sApiCoreV1ResourceRequirements.from_dict(io_k8s_api_core_v1_resource_requirements_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


