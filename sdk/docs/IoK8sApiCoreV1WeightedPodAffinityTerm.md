# IoK8sApiCoreV1WeightedPodAffinityTerm

The weights of all of the matched WeightedPodAffinityTerm fields are added per-node to find the most preferred node(s)

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**pod_affinity_term** | [**IoK8sApiCoreV1PodAffinityTerm**](IoK8sApiCoreV1PodAffinityTerm.md) |  | 
**weight** | **int** | weight associated with matching the corresponding podAffinityTerm, in the range 1-100. | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_weighted_pod_affinity_term import IoK8sApiCoreV1WeightedPodAffinityTerm

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1WeightedPodAffinityTerm from a JSON string
io_k8s_api_core_v1_weighted_pod_affinity_term_instance = IoK8sApiCoreV1WeightedPodAffinityTerm.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1WeightedPodAffinityTerm.to_json())

# convert the object into a dict
io_k8s_api_core_v1_weighted_pod_affinity_term_dict = io_k8s_api_core_v1_weighted_pod_affinity_term_instance.to_dict()
# create an instance of IoK8sApiCoreV1WeightedPodAffinityTerm from a dict
io_k8s_api_core_v1_weighted_pod_affinity_term_from_dict = IoK8sApiCoreV1WeightedPodAffinityTerm.from_dict(io_k8s_api_core_v1_weighted_pod_affinity_term_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


