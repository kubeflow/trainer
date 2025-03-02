# TrainerV1alpha1RuntimeRef

RuntimeRef represents the reference to the existing training runtime.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**api_group** | **str** | APIGroup of the runtime being referenced. Defaults to &#x60;trainer.kubeflow.org&#x60;. | [optional] 
**kind** | **str** | Kind of the runtime being referenced. Defaults to ClusterTrainingRuntime. | [optional] 
**name** | **str** | Name of the runtime being referenced. When namespaced-scoped TrainingRuntime is used, the TrainJob must have the same namespace as the deployed runtime. | [default to '']

## Example

```python
from kubeflow.trainer.models.trainer_v1alpha1_runtime_ref import TrainerV1alpha1RuntimeRef

# TODO update the JSON string below
json = "{}"
# create an instance of TrainerV1alpha1RuntimeRef from a JSON string
trainer_v1alpha1_runtime_ref_instance = TrainerV1alpha1RuntimeRef.from_json(json)
# print the JSON string representation of the object
print(TrainerV1alpha1RuntimeRef.to_json())

# convert the object into a dict
trainer_v1alpha1_runtime_ref_dict = trainer_v1alpha1_runtime_ref_instance.to_dict()
# create an instance of TrainerV1alpha1RuntimeRef from a dict
trainer_v1alpha1_runtime_ref_from_dict = TrainerV1alpha1RuntimeRef.from_dict(trainer_v1alpha1_runtime_ref_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


