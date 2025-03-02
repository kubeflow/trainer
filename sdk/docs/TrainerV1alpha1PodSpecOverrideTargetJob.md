# TrainerV1alpha1PodSpecOverrideTargetJob


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**name** | **str** | Name is the target training job name for which the PodSpec is overridden. | [default to '']

## Example

```python
from kubeflow.trainer.models.trainer_v1alpha1_pod_spec_override_target_job import TrainerV1alpha1PodSpecOverrideTargetJob

# TODO update the JSON string below
json = "{}"
# create an instance of TrainerV1alpha1PodSpecOverrideTargetJob from a JSON string
trainer_v1alpha1_pod_spec_override_target_job_instance = TrainerV1alpha1PodSpecOverrideTargetJob.from_json(json)
# print the JSON string representation of the object
print(TrainerV1alpha1PodSpecOverrideTargetJob.to_json())

# convert the object into a dict
trainer_v1alpha1_pod_spec_override_target_job_dict = trainer_v1alpha1_pod_spec_override_target_job_instance.to_dict()
# create an instance of TrainerV1alpha1PodSpecOverrideTargetJob from a dict
trainer_v1alpha1_pod_spec_override_target_job_from_dict = TrainerV1alpha1PodSpecOverrideTargetJob.from_dict(trainer_v1alpha1_pod_spec_override_target_job_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


