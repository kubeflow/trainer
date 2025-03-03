# JobsetV1alpha2StartupPolicy


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**startup_policy_order** | **str** | StartupPolicyOrder determines the startup order of the ReplicatedJobs. AnyOrder means to start replicated jobs in any order. InOrder means to start them as they are listed in the JobSet. A ReplicatedJob is started only when all the jobs of the previous one are ready. | [default to '']

## Example

```python
from kubeflow.trainer.models.jobset_v1alpha2_startup_policy import JobsetV1alpha2StartupPolicy

# TODO update the JSON string below
json = "{}"
# create an instance of JobsetV1alpha2StartupPolicy from a JSON string
jobset_v1alpha2_startup_policy_instance = JobsetV1alpha2StartupPolicy.from_json(json)
# print the JSON string representation of the object
print(JobsetV1alpha2StartupPolicy.to_json())

# convert the object into a dict
jobset_v1alpha2_startup_policy_dict = jobset_v1alpha2_startup_policy_instance.to_dict()
# create an instance of JobsetV1alpha2StartupPolicy from a dict
jobset_v1alpha2_startup_policy_from_dict = JobsetV1alpha2StartupPolicy.from_dict(jobset_v1alpha2_startup_policy_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


