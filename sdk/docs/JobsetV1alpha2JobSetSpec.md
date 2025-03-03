# JobsetV1alpha2JobSetSpec

JobSetSpec defines the desired state of JobSet

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**coordinator** | [**JobsetV1alpha2Coordinator**](JobsetV1alpha2Coordinator.md) |  | [optional] 
**failure_policy** | [**JobsetV1alpha2FailurePolicy**](JobsetV1alpha2FailurePolicy.md) |  | [optional] 
**managed_by** | **str** | ManagedBy is used to indicate the controller or entity that manages a JobSet. The built-in JobSet controller reconciles JobSets which don&#39;t have this field at all or the field value is the reserved string &#x60;jobset.sigs.k8s.io/jobset-controller&#x60;, but skips reconciling JobSets with a custom value for this field.  The value must be a valid domain-prefixed path (e.g. acme.io/foo) - all characters before the first \&quot;/\&quot; must be a valid subdomain as defined by RFC 1123. All characters trailing the first \&quot;/\&quot; must be valid HTTP Path characters as defined by RFC 3986. The value cannot exceed 63 characters. The field is immutable. | [optional] 
**network** | [**JobsetV1alpha2Network**](JobsetV1alpha2Network.md) |  | [optional] 
**replicated_jobs** | [**List[JobsetV1alpha2ReplicatedJob]**](JobsetV1alpha2ReplicatedJob.md) | ReplicatedJobs is the group of jobs that will form the set. | [optional] 
**startup_policy** | [**JobsetV1alpha2StartupPolicy**](JobsetV1alpha2StartupPolicy.md) |  | [optional] 
**success_policy** | [**JobsetV1alpha2SuccessPolicy**](JobsetV1alpha2SuccessPolicy.md) |  | [optional] 
**suspend** | **bool** | Suspend suspends all running child Jobs when set to true. | [optional] 
**ttl_seconds_after_finished** | **int** | TTLSecondsAfterFinished limits the lifetime of a JobSet that has finished execution (either Complete or Failed). If this field is set, TTLSecondsAfterFinished after the JobSet finishes, it is eligible to be automatically deleted. When the JobSet is being deleted, its lifecycle guarantees (e.g. finalizers) will be honored. If this field is unset, the JobSet won&#39;t be automatically deleted. If this field is set to zero, the JobSet becomes eligible to be deleted immediately after it finishes. | [optional] 

## Example

```python
from kubeflow.trainer.models.jobset_v1alpha2_job_set_spec import JobsetV1alpha2JobSetSpec

# TODO update the JSON string below
json = "{}"
# create an instance of JobsetV1alpha2JobSetSpec from a JSON string
jobset_v1alpha2_job_set_spec_instance = JobsetV1alpha2JobSetSpec.from_json(json)
# print the JSON string representation of the object
print(JobsetV1alpha2JobSetSpec.to_json())

# convert the object into a dict
jobset_v1alpha2_job_set_spec_dict = jobset_v1alpha2_job_set_spec_instance.to_dict()
# create an instance of JobsetV1alpha2JobSetSpec from a dict
jobset_v1alpha2_job_set_spec_from_dict = JobsetV1alpha2JobSetSpec.from_dict(jobset_v1alpha2_job_set_spec_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


