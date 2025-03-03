# JobsetV1alpha2Coordinator

Coordinator defines which pod can be marked as the coordinator for the JobSet workload.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**job_index** | **int** | JobIndex is the index of Job which contains the coordinator pod (i.e., for a ReplicatedJob with N replicas, there are Job indexes 0 to N-1). | [optional] 
**pod_index** | **int** | PodIndex is the Job completion index of the coordinator pod. | [optional] 
**replicated_job** | **str** | ReplicatedJob is the name of the ReplicatedJob which contains the coordinator pod. | [default to '']

## Example

```python
from kubeflow.trainer.models.jobset_v1alpha2_coordinator import JobsetV1alpha2Coordinator

# TODO update the JSON string below
json = "{}"
# create an instance of JobsetV1alpha2Coordinator from a JSON string
jobset_v1alpha2_coordinator_instance = JobsetV1alpha2Coordinator.from_json(json)
# print the JSON string representation of the object
print(JobsetV1alpha2Coordinator.to_json())

# convert the object into a dict
jobset_v1alpha2_coordinator_dict = jobset_v1alpha2_coordinator_instance.to_dict()
# create an instance of JobsetV1alpha2Coordinator from a dict
jobset_v1alpha2_coordinator_from_dict = JobsetV1alpha2Coordinator.from_dict(jobset_v1alpha2_coordinator_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


