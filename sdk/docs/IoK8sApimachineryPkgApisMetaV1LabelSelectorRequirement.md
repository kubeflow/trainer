# IoK8sApimachineryPkgApisMetaV1LabelSelectorRequirement

A label selector requirement is a selector that contains values, a key, and an operator that relates the key and values.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**key** | **str** | key is the label key that the selector applies to. | 
**operator** | **str** | operator represents a key&#39;s relationship to a set of values. Valid operators are In, NotIn, Exists and DoesNotExist. | 
**values** | **List[str]** | values is an array of string values. If the operator is In or NotIn, the values array must be non-empty. If the operator is Exists or DoesNotExist, the values array must be empty. This array is replaced during a strategic merge patch. | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_apimachinery_pkg_apis_meta_v1_label_selector_requirement import IoK8sApimachineryPkgApisMetaV1LabelSelectorRequirement

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApimachineryPkgApisMetaV1LabelSelectorRequirement from a JSON string
io_k8s_apimachinery_pkg_apis_meta_v1_label_selector_requirement_instance = IoK8sApimachineryPkgApisMetaV1LabelSelectorRequirement.from_json(json)
# print the JSON string representation of the object
print(IoK8sApimachineryPkgApisMetaV1LabelSelectorRequirement.to_json())

# convert the object into a dict
io_k8s_apimachinery_pkg_apis_meta_v1_label_selector_requirement_dict = io_k8s_apimachinery_pkg_apis_meta_v1_label_selector_requirement_instance.to_dict()
# create an instance of IoK8sApimachineryPkgApisMetaV1LabelSelectorRequirement from a dict
io_k8s_apimachinery_pkg_apis_meta_v1_label_selector_requirement_from_dict = IoK8sApimachineryPkgApisMetaV1LabelSelectorRequirement.from_dict(io_k8s_apimachinery_pkg_apis_meta_v1_label_selector_requirement_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


