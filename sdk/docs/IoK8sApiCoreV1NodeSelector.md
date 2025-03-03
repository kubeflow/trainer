# IoK8sApiCoreV1NodeSelector

A node selector represents the union of the results of one or more label queries over a set of nodes; that is, it represents the OR of the selectors represented by the node selector terms.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**node_selector_terms** | [**List[IoK8sApiCoreV1NodeSelectorTerm]**](IoK8sApiCoreV1NodeSelectorTerm.md) | Required. A list of node selector terms. The terms are ORed. | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_node_selector import IoK8sApiCoreV1NodeSelector

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1NodeSelector from a JSON string
io_k8s_api_core_v1_node_selector_instance = IoK8sApiCoreV1NodeSelector.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1NodeSelector.to_json())

# convert the object into a dict
io_k8s_api_core_v1_node_selector_dict = io_k8s_api_core_v1_node_selector_instance.to_dict()
# create an instance of IoK8sApiCoreV1NodeSelector from a dict
io_k8s_api_core_v1_node_selector_from_dict = IoK8sApiCoreV1NodeSelector.from_dict(io_k8s_api_core_v1_node_selector_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


