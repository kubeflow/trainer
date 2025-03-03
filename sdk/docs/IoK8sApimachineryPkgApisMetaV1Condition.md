# IoK8sApimachineryPkgApisMetaV1Condition

Condition contains details for one aspect of the current state of this API Resource.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**last_transition_time** | **datetime** | Time is a wrapper around time.Time which supports correct marshaling to YAML and JSON.  Wrappers are provided for many of the factory methods that the time package offers. | 
**message** | **str** | message is a human readable message indicating details about the transition. This may be an empty string. | 
**observed_generation** | **int** | observedGeneration represents the .metadata.generation that the condition was set based upon. For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date with respect to the current state of the instance. | [optional] 
**reason** | **str** | reason contains a programmatic identifier indicating the reason for the condition&#39;s last transition. Producers of specific condition types may define expected values and meanings for this field, and whether the values are considered a guaranteed API. The value should be a CamelCase string. This field may not be empty. | 
**status** | **str** | status of the condition, one of True, False, Unknown. | 
**type** | **str** | type of condition in CamelCase or in foo.example.com/CamelCase. | 

## Example

```python
from kubeflow.trainer.models.io_k8s_apimachinery_pkg_apis_meta_v1_condition import IoK8sApimachineryPkgApisMetaV1Condition

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApimachineryPkgApisMetaV1Condition from a JSON string
io_k8s_apimachinery_pkg_apis_meta_v1_condition_instance = IoK8sApimachineryPkgApisMetaV1Condition.from_json(json)
# print the JSON string representation of the object
print(IoK8sApimachineryPkgApisMetaV1Condition.to_json())

# convert the object into a dict
io_k8s_apimachinery_pkg_apis_meta_v1_condition_dict = io_k8s_apimachinery_pkg_apis_meta_v1_condition_instance.to_dict()
# create an instance of IoK8sApimachineryPkgApisMetaV1Condition from a dict
io_k8s_apimachinery_pkg_apis_meta_v1_condition_from_dict = IoK8sApimachineryPkgApisMetaV1Condition.from_dict(io_k8s_apimachinery_pkg_apis_meta_v1_condition_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


