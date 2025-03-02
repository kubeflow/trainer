# TrainerV1alpha1TrainingRuntimeSpec

TrainingRuntimeSpec represents a specification of the desired training runtime.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**ml_policy** | [**TrainerV1alpha1MLPolicy**](TrainerV1alpha1MLPolicy.md) |  | [optional] 
**pod_group_policy** | [**TrainerV1alpha1PodGroupPolicy**](TrainerV1alpha1PodGroupPolicy.md) |  | [optional] 
**template** | [**TrainerV1alpha1JobSetTemplateSpec**](TrainerV1alpha1JobSetTemplateSpec.md) |  | 

## Example

```python
from kubeflow.trainer.models.trainer_v1alpha1_training_runtime_spec import TrainerV1alpha1TrainingRuntimeSpec

# TODO update the JSON string below
json = "{}"
# create an instance of TrainerV1alpha1TrainingRuntimeSpec from a JSON string
trainer_v1alpha1_training_runtime_spec_instance = TrainerV1alpha1TrainingRuntimeSpec.from_json(json)
# print the JSON string representation of the object
print(TrainerV1alpha1TrainingRuntimeSpec.to_json())

# convert the object into a dict
trainer_v1alpha1_training_runtime_spec_dict = trainer_v1alpha1_training_runtime_spec_instance.to_dict()
# create an instance of TrainerV1alpha1TrainingRuntimeSpec from a dict
trainer_v1alpha1_training_runtime_spec_from_dict = TrainerV1alpha1TrainingRuntimeSpec.from_dict(trainer_v1alpha1_training_runtime_spec_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


