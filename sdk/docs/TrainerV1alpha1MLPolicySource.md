# TrainerV1alpha1MLPolicySource

MLPolicySource represents the runtime-specific configuration for various technologies. One of the following specs can be set.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**mpi** | [**TrainerV1alpha1MPIMLPolicySource**](TrainerV1alpha1MPIMLPolicySource.md) |  | [optional] 
**torch** | [**TrainerV1alpha1TorchMLPolicySource**](TrainerV1alpha1TorchMLPolicySource.md) |  | [optional] 

## Example

```python
from kubeflow.trainer.models.trainer_v1alpha1_ml_policy_source import TrainerV1alpha1MLPolicySource

# TODO update the JSON string below
json = "{}"
# create an instance of TrainerV1alpha1MLPolicySource from a JSON string
trainer_v1alpha1_ml_policy_source_instance = TrainerV1alpha1MLPolicySource.from_json(json)
# print the JSON string representation of the object
print(TrainerV1alpha1MLPolicySource.to_json())

# convert the object into a dict
trainer_v1alpha1_ml_policy_source_dict = trainer_v1alpha1_ml_policy_source_instance.to_dict()
# create an instance of TrainerV1alpha1MLPolicySource from a dict
trainer_v1alpha1_ml_policy_source_from_dict = TrainerV1alpha1MLPolicySource.from_dict(trainer_v1alpha1_ml_policy_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


