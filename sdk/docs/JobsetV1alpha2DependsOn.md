# JobsetV1alpha2DependsOn

DependsOn defines the dependency on the previous ReplicatedJob status.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**name** | **str** | Name of the previous ReplicatedJob. | [default to '']
**status** | **str** | Status defines the condition for the ReplicatedJob. Only Ready or Complete status can be set. | [default to '']

## Example

```python
from kubeflow.trainer.models.jobset_v1alpha2_depends_on import JobsetV1alpha2DependsOn

# TODO update the JSON string below
json = "{}"
# create an instance of JobsetV1alpha2DependsOn from a JSON string
jobset_v1alpha2_depends_on_instance = JobsetV1alpha2DependsOn.from_json(json)
# print the JSON string representation of the object
print(JobsetV1alpha2DependsOn.to_json())

# convert the object into a dict
jobset_v1alpha2_depends_on_dict = jobset_v1alpha2_depends_on_instance.to_dict()
# create an instance of JobsetV1alpha2DependsOn from a dict
jobset_v1alpha2_depends_on_from_dict = JobsetV1alpha2DependsOn.from_dict(jobset_v1alpha2_depends_on_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


