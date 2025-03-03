# IoK8sApiCoreV1NodeAffinity

Node affinity is a group of node affinity scheduling rules.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**preferred_during_scheduling_ignored_during_execution** | [**List[IoK8sApiCoreV1PreferredSchedulingTerm]**](IoK8sApiCoreV1PreferredSchedulingTerm.md) | The scheduler will prefer to schedule pods to nodes that satisfy the affinity expressions specified by this field, but it may choose a node that violates one or more of the expressions. The node that is most preferred is the one with the greatest sum of weights, i.e. for each node that meets all of the scheduling requirements (resource request, requiredDuringScheduling affinity expressions, etc.), compute a sum by iterating through the elements of this field and adding \&quot;weight\&quot; to the sum if the node matches the corresponding matchExpressions; the node(s) with the highest sum are the most preferred. | [optional] 
**required_during_scheduling_ignored_during_execution** | [**IoK8sApiCoreV1NodeSelector**](IoK8sApiCoreV1NodeSelector.md) |  | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_node_affinity import IoK8sApiCoreV1NodeAffinity

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1NodeAffinity from a JSON string
io_k8s_api_core_v1_node_affinity_instance = IoK8sApiCoreV1NodeAffinity.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1NodeAffinity.to_json())

# convert the object into a dict
io_k8s_api_core_v1_node_affinity_dict = io_k8s_api_core_v1_node_affinity_instance.to_dict()
# create an instance of IoK8sApiCoreV1NodeAffinity from a dict
io_k8s_api_core_v1_node_affinity_from_dict = IoK8sApiCoreV1NodeAffinity.from_dict(io_k8s_api_core_v1_node_affinity_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


