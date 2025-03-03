# IoK8sApiCoreV1NodeSelectorTerm

A null or empty node selector term matches no objects. The requirements of them are ANDed. The TopologySelectorTerm type implements a subset of the NodeSelectorTerm.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**match_expressions** | [**List[IoK8sApiCoreV1NodeSelectorRequirement]**](IoK8sApiCoreV1NodeSelectorRequirement.md) | A list of node selector requirements by node&#39;s labels. | [optional] 
**match_fields** | [**List[IoK8sApiCoreV1NodeSelectorRequirement]**](IoK8sApiCoreV1NodeSelectorRequirement.md) | A list of node selector requirements by node&#39;s fields. | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_node_selector_term import IoK8sApiCoreV1NodeSelectorTerm

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1NodeSelectorTerm from a JSON string
io_k8s_api_core_v1_node_selector_term_instance = IoK8sApiCoreV1NodeSelectorTerm.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1NodeSelectorTerm.to_json())

# convert the object into a dict
io_k8s_api_core_v1_node_selector_term_dict = io_k8s_api_core_v1_node_selector_term_instance.to_dict()
# create an instance of IoK8sApiCoreV1NodeSelectorTerm from a dict
io_k8s_api_core_v1_node_selector_term_from_dict = IoK8sApiCoreV1NodeSelectorTerm.from_dict(io_k8s_api_core_v1_node_selector_term_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


