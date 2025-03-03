# JobsetV1alpha2FailurePolicy


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**max_restarts** | **int** | MaxRestarts defines the limit on the number of JobSet restarts. A restart is achieved by recreating all active child jobs. | [optional] 
**restart_strategy** | **str** | RestartStrategy defines the strategy to use when restarting the JobSet. Defaults to Recreate. | [optional] 
**rules** | [**List[JobsetV1alpha2FailurePolicyRule]**](JobsetV1alpha2FailurePolicyRule.md) | List of failure policy rules for this JobSet. For a given Job failure, the rules will be evaluated in order, and only the first matching rule will be executed. If no matching rule is found, the RestartJobSet action is applied. | [optional] 

## Example

```python
from kubeflow.trainer.models.jobset_v1alpha2_failure_policy import JobsetV1alpha2FailurePolicy

# TODO update the JSON string below
json = "{}"
# create an instance of JobsetV1alpha2FailurePolicy from a JSON string
jobset_v1alpha2_failure_policy_instance = JobsetV1alpha2FailurePolicy.from_json(json)
# print the JSON string representation of the object
print(JobsetV1alpha2FailurePolicy.to_json())

# convert the object into a dict
jobset_v1alpha2_failure_policy_dict = jobset_v1alpha2_failure_policy_instance.to_dict()
# create an instance of JobsetV1alpha2FailurePolicy from a dict
jobset_v1alpha2_failure_policy_from_dict = JobsetV1alpha2FailurePolicy.from_dict(jobset_v1alpha2_failure_policy_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


