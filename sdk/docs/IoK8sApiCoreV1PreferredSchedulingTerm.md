# IoK8sApiCoreV1PreferredSchedulingTerm

An empty preferred scheduling term matches all objects with implicit weight 0 (i.e. it's a no-op). A null preferred scheduling term matches no objects (i.e. is also a no-op).

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**preference** | [**IoK8sApiCoreV1NodeSelectorTerm**](IoK8sApiCoreV1NodeSelectorTerm.md) |  | 
**weight** | **int** | Weight associated with matching the corresponding nodeSelectorTerm, in the range 1-100. | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_preferred_scheduling_term import IoK8sApiCoreV1PreferredSchedulingTerm

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1PreferredSchedulingTerm from a JSON string
io_k8s_api_core_v1_preferred_scheduling_term_instance = IoK8sApiCoreV1PreferredSchedulingTerm.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1PreferredSchedulingTerm.to_json())

# convert the object into a dict
io_k8s_api_core_v1_preferred_scheduling_term_dict = io_k8s_api_core_v1_preferred_scheduling_term_instance.to_dict()
# create an instance of IoK8sApiCoreV1PreferredSchedulingTerm from a dict
io_k8s_api_core_v1_preferred_scheduling_term_from_dict = IoK8sApiCoreV1PreferredSchedulingTerm.from_dict(io_k8s_api_core_v1_preferred_scheduling_term_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


