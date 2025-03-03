# TrainerV1alpha1TrainJobStatus

TrainJobStatus represents the current status of TrainJob.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**conditions** | [**List[IoK8sApimachineryPkgApisMetaV1Condition]**](IoK8sApimachineryPkgApisMetaV1Condition.md) | Conditions for the TrainJob. | [optional] 
**jobs_status** | [**List[TrainerV1alpha1JobStatus]**](TrainerV1alpha1JobStatus.md) | JobsStatus tracks the child Jobs in TrainJob. | [optional] 

## Example

```python
from kubeflow.trainer.models.trainer_v1alpha1_train_job_status import TrainerV1alpha1TrainJobStatus

# TODO update the JSON string below
json = "{}"
# create an instance of TrainerV1alpha1TrainJobStatus from a JSON string
trainer_v1alpha1_train_job_status_instance = TrainerV1alpha1TrainJobStatus.from_json(json)
# print the JSON string representation of the object
print(TrainerV1alpha1TrainJobStatus.to_json())

# convert the object into a dict
trainer_v1alpha1_train_job_status_dict = trainer_v1alpha1_train_job_status_instance.to_dict()
# create an instance of TrainerV1alpha1TrainJobStatus from a dict
trainer_v1alpha1_train_job_status_from_dict = TrainerV1alpha1TrainJobStatus.from_dict(trainer_v1alpha1_train_job_status_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


