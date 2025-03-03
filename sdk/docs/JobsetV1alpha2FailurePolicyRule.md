# JobsetV1alpha2FailurePolicyRule

FailurePolicyRule defines a FailurePolicyAction to be executed if a child job fails due to a reason listed in OnJobFailureReasons.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**action** | **str** | The action to take if the rule is matched. | [default to '']
**name** | **str** | The name of the failure policy rule. The name is defaulted to &#39;failurePolicyRuleN&#39; where N is the index of the failure policy rule. The name must match the regular expression \&quot;^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$\&quot;. | [default to '']
**on_job_failure_reasons** | **List[str]** | The requirement on the job failure reasons. The requirement is satisfied if at least one reason matches the list. The rules are evaluated in order, and the first matching rule is executed. An empty list applies the rule to any job failure reason. | [optional] 
**target_replicated_jobs** | **List[str]** | TargetReplicatedJobs are the names of the replicated jobs the operator applies to. An empty list will apply to all replicatedJobs. | [optional] 

## Example

```python
from kubeflow.trainer.models.jobset_v1alpha2_failure_policy_rule import JobsetV1alpha2FailurePolicyRule

# TODO update the JSON string below
json = "{}"
# create an instance of JobsetV1alpha2FailurePolicyRule from a JSON string
jobset_v1alpha2_failure_policy_rule_instance = JobsetV1alpha2FailurePolicyRule.from_json(json)
# print the JSON string representation of the object
print(JobsetV1alpha2FailurePolicyRule.to_json())

# convert the object into a dict
jobset_v1alpha2_failure_policy_rule_dict = jobset_v1alpha2_failure_policy_rule_instance.to_dict()
# create an instance of JobsetV1alpha2FailurePolicyRule from a dict
jobset_v1alpha2_failure_policy_rule_from_dict = JobsetV1alpha2FailurePolicyRule.from_dict(jobset_v1alpha2_failure_policy_rule_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


