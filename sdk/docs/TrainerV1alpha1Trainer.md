# TrainerV1alpha1Trainer

Trainer represents the desired trainer configuration. Every training runtime contains `trainer` container which represents Trainer.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**args** | **List[str]** | Arguments to the entrypoint for the training container. | [optional] 
**command** | **List[str]** | Entrypoint commands for the training container. | [optional] 
**env** | [**List[IoK8sApiCoreV1EnvVar]**](IoK8sApiCoreV1EnvVar.md) | List of environment variables to set in the training container. These values will be merged with the TrainingRuntime&#39;s trainer environments. | [optional] 
**image** | **str** | Docker image for the training container. | [optional] 
**num_nodes** | **int** | Number of training nodes. | [optional] 
**num_proc_per_node** | **str** | IntOrString is a type that can hold an int32 or a string.  When used in JSON or YAML marshalling and unmarshalling, it produces or consumes the inner type.  This allows you to have, for example, a JSON field that can accept a name or number. | [optional] 
**resources_per_node** | [**IoK8sApiCoreV1ResourceRequirements**](IoK8sApiCoreV1ResourceRequirements.md) |  | [optional] 

## Example

```python
from kubeflow.trainer.models.trainer_v1alpha1_trainer import TrainerV1alpha1Trainer

# TODO update the JSON string below
json = "{}"
# create an instance of TrainerV1alpha1Trainer from a JSON string
trainer_v1alpha1_trainer_instance = TrainerV1alpha1Trainer.from_json(json)
# print the JSON string representation of the object
print(TrainerV1alpha1Trainer.to_json())

# convert the object into a dict
trainer_v1alpha1_trainer_dict = trainer_v1alpha1_trainer_instance.to_dict()
# create an instance of TrainerV1alpha1Trainer from a dict
trainer_v1alpha1_trainer_from_dict = TrainerV1alpha1Trainer.from_dict(trainer_v1alpha1_trainer_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


