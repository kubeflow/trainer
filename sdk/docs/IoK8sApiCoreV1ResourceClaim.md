# IoK8sApiCoreV1ResourceClaim

ResourceClaim references one entry in PodSpec.ResourceClaims.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**name** | **str** | Name must match the name of one entry in pod.spec.resourceClaims of the Pod where this field is used. It makes that resource available inside a container. | 
**request** | **str** | Request is the name chosen for a request in the referenced claim. If empty, everything from the claim is made available, otherwise only the result of this request. | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_resource_claim import IoK8sApiCoreV1ResourceClaim

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1ResourceClaim from a JSON string
io_k8s_api_core_v1_resource_claim_instance = IoK8sApiCoreV1ResourceClaim.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1ResourceClaim.to_json())

# convert the object into a dict
io_k8s_api_core_v1_resource_claim_dict = io_k8s_api_core_v1_resource_claim_instance.to_dict()
# create an instance of IoK8sApiCoreV1ResourceClaim from a dict
io_k8s_api_core_v1_resource_claim_from_dict = IoK8sApiCoreV1ResourceClaim.from_dict(io_k8s_api_core_v1_resource_claim_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


