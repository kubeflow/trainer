# TrainerV1alpha1MLPolicy

MLPolicy represents configuration for the model trining with ML-specific parameters.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**mpi** | [**TrainerV1alpha1MPIMLPolicySource**](TrainerV1alpha1MPIMLPolicySource.md) |  | [optional] 
**num_nodes** | **int** | Number of training nodes. Defaults to 1. | [optional] 
**torch** | [**TrainerV1alpha1TorchMLPolicySource**](TrainerV1alpha1TorchMLPolicySource.md) |  | [optional] 

## Example

```python
from kubeflow.trainer.models.trainer_v1alpha1_ml_policy import TrainerV1alpha1MLPolicy

# TODO update the JSON string below
json = "{}"
# create an instance of TrainerV1alpha1MLPolicy from a JSON string
trainer_v1alpha1_ml_policy_instance = TrainerV1alpha1MLPolicy.from_json(json)
# print the JSON string representation of the object
print(TrainerV1alpha1MLPolicy.to_json())

# convert the object into a dict
trainer_v1alpha1_ml_policy_dict = trainer_v1alpha1_ml_policy_instance.to_dict()
# create an instance of TrainerV1alpha1MLPolicy from a dict
trainer_v1alpha1_ml_policy_from_dict = TrainerV1alpha1MLPolicy.from_dict(trainer_v1alpha1_ml_policy_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


