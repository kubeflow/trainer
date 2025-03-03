# IoK8sApiCoreV1NodeSelectorRequirement

A node selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**key** | **str** | The label key that the selector applies to. | 
**operator** | **str** | Represents a key&#39;s relationship to a set of values. Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt. | 
**values** | **List[str]** | An array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. If the operator is Gt or Lt, the values array must have a single element, which will be interpreted as an integer. This array is replaced during a strategic merge patch. | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_node_selector_requirement import IoK8sApiCoreV1NodeSelectorRequirement

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1NodeSelectorRequirement from a JSON string
io_k8s_api_core_v1_node_selector_requirement_instance = IoK8sApiCoreV1NodeSelectorRequirement.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1NodeSelectorRequirement.to_json())

# convert the object into a dict
io_k8s_api_core_v1_node_selector_requirement_dict = io_k8s_api_core_v1_node_selector_requirement_instance.to_dict()
# create an instance of IoK8sApiCoreV1NodeSelectorRequirement from a dict
io_k8s_api_core_v1_node_selector_requirement_from_dict = IoK8sApiCoreV1NodeSelectorRequirement.from_dict(io_k8s_api_core_v1_node_selector_requirement_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


