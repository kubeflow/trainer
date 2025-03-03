# JobsetV1alpha2SuccessPolicy


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**operator** | **str** | Operator determines either All or Any of the selected jobs should succeed to consider the JobSet successful | [default to '']
**target_replicated_jobs** | **List[str]** | TargetReplicatedJobs are the names of the replicated jobs the operator will apply to. A null or empty list will apply to all replicatedJobs. | [optional] 

## Example

```python
from kubeflow.trainer.models.jobset_v1alpha2_success_policy import JobsetV1alpha2SuccessPolicy

# TODO update the JSON string below
json = "{}"
# create an instance of JobsetV1alpha2SuccessPolicy from a JSON string
jobset_v1alpha2_success_policy_instance = JobsetV1alpha2SuccessPolicy.from_json(json)
# print the JSON string representation of the object
print(JobsetV1alpha2SuccessPolicy.to_json())

# convert the object into a dict
jobset_v1alpha2_success_policy_dict = jobset_v1alpha2_success_policy_instance.to_dict()
# create an instance of JobsetV1alpha2SuccessPolicy from a dict
jobset_v1alpha2_success_policy_from_dict = JobsetV1alpha2SuccessPolicy.from_dict(jobset_v1alpha2_success_policy_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


