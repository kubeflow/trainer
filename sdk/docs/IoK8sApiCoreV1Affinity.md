# IoK8sApiCoreV1Affinity

Affinity is a group of affinity scheduling rules.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**node_affinity** | [**IoK8sApiCoreV1NodeAffinity**](IoK8sApiCoreV1NodeAffinity.md) |  | [optional] 
**pod_affinity** | [**IoK8sApiCoreV1PodAffinity**](IoK8sApiCoreV1PodAffinity.md) |  | [optional] 
**pod_anti_affinity** | [**IoK8sApiCoreV1PodAntiAffinity**](IoK8sApiCoreV1PodAntiAffinity.md) |  | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_affinity import IoK8sApiCoreV1Affinity

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1Affinity from a JSON string
io_k8s_api_core_v1_affinity_instance = IoK8sApiCoreV1Affinity.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1Affinity.to_json())

# convert the object into a dict
io_k8s_api_core_v1_affinity_dict = io_k8s_api_core_v1_affinity_instance.to_dict()
# create an instance of IoK8sApiCoreV1Affinity from a dict
io_k8s_api_core_v1_affinity_from_dict = IoK8sApiCoreV1Affinity.from_dict(io_k8s_api_core_v1_affinity_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


