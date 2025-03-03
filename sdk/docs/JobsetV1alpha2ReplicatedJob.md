# JobsetV1alpha2ReplicatedJob


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**depends_on** | [**List[JobsetV1alpha2DependsOn]**](JobsetV1alpha2DependsOn.md) | DependsOn is an optional list that specifies the preceding ReplicatedJobs upon which the current ReplicatedJob depends. If specified, the ReplicatedJob will be created only after the referenced ReplicatedJobs reach their desired state. The Order of ReplicatedJobs is defined by their enumeration in the slice. Note, that the first ReplicatedJob in the slice cannot use the DependsOn API. Currently, only a single item is supported in the DependsOn list. If JobSet is suspended the all active ReplicatedJobs will be suspended. When JobSet is resumed the Job sequence starts again. This API is mutually exclusive with the StartupPolicy API. | [optional] 
**name** | **str** | Name is the name of the entry and will be used as a suffix for the Job name. | [default to '']
**replicas** | **int** | Replicas is the number of jobs that will be created from this ReplicatedJob&#39;s template. Jobs names will be in the format: &lt;jobSet.name&gt;-&lt;spec.replicatedJob.name&gt;-&lt;job-index&gt; | [optional] 
**template** | [**IoK8sApiBatchV1JobTemplateSpec**](IoK8sApiBatchV1JobTemplateSpec.md) |  | 

## Example

```python
from kubeflow.trainer.models.jobset_v1alpha2_replicated_job import JobsetV1alpha2ReplicatedJob

# TODO update the JSON string below
json = "{}"
# create an instance of JobsetV1alpha2ReplicatedJob from a JSON string
jobset_v1alpha2_replicated_job_instance = JobsetV1alpha2ReplicatedJob.from_json(json)
# print the JSON string representation of the object
print(JobsetV1alpha2ReplicatedJob.to_json())

# convert the object into a dict
jobset_v1alpha2_replicated_job_dict = jobset_v1alpha2_replicated_job_instance.to_dict()
# create an instance of JobsetV1alpha2ReplicatedJob from a dict
jobset_v1alpha2_replicated_job_from_dict = JobsetV1alpha2ReplicatedJob.from_dict(jobset_v1alpha2_replicated_job_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


