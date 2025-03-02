# TrainerV1alpha1JobStatus


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**active** | **int** | Active is the number of child Jobs with at least 1 pod in a running or pending state which are not marked for deletion. | [default to 0]
**failed** | **int** | Failed is the number of failed child Jobs. | [default to 0]
**name** | **str** | Name of the child Job. | [default to '']
**ready** | **int** | Ready is the number of child Jobs where the number of ready pods and completed pods is greater than or equal to the total expected pod count for the child Job. | [default to 0]
**succeeded** | **int** | Succeeded is the number of successfully completed child Jobs. | [default to 0]
**suspended** | **int** | Suspended is the number of child Jobs which are in a suspended state. | [default to 0]

## Example

```python
from kubeflow.trainer.models.trainer_v1alpha1_job_status import TrainerV1alpha1JobStatus

# TODO update the JSON string below
json = "{}"
# create an instance of TrainerV1alpha1JobStatus from a JSON string
trainer_v1alpha1_job_status_instance = TrainerV1alpha1JobStatus.from_json(json)
# print the JSON string representation of the object
print(TrainerV1alpha1JobStatus.to_json())

# convert the object into a dict
trainer_v1alpha1_job_status_dict = trainer_v1alpha1_job_status_instance.to_dict()
# create an instance of TrainerV1alpha1JobStatus from a dict
trainer_v1alpha1_job_status_from_dict = TrainerV1alpha1JobStatus.from_dict(trainer_v1alpha1_job_status_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


