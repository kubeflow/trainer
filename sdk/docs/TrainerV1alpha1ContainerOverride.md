# TrainerV1alpha1ContainerOverride

ContainerOverride represents parameters that can be overridden using PodSpecOverrides. Parameters from the Trainer, DatasetConfig, and ModelConfig will take precedence.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**args** | **List[str]** | Arguments to the entrypoint for the training container. | [optional] 
**command** | **List[str]** | Entrypoint commands for the training container. | [optional] 
**env** | [**List[V1EnvVar]**](V1EnvVar.md) | List of environment variables to set in the container. These values will be merged with the TrainingRuntime&#39;s environments. | [optional] 
**env_from** | [**List[V1EnvFromSource]**](V1EnvFromSource.md) | List of sources to populate environment variables in the container. These   values will be merged with the TrainingRuntime&#39;s environments. | [optional] 
**name** | **str** | Name for the container. TrainingRuntime must have this container. | [default to '']
**volume_mounts** | [**List[V1VolumeMount]**](V1VolumeMount.md) | Pod volumes to mount into the container&#39;s filesystem. | [optional] 

## Example

```python
from kubeflow.trainer.models.trainer_v1alpha1_container_override import TrainerV1alpha1ContainerOverride

# TODO update the JSON string below
json = "{}"
# create an instance of TrainerV1alpha1ContainerOverride from a JSON string
trainer_v1alpha1_container_override_instance = TrainerV1alpha1ContainerOverride.from_json(json)
# print the JSON string representation of the object
print(TrainerV1alpha1ContainerOverride.to_json())

# convert the object into a dict
trainer_v1alpha1_container_override_dict = trainer_v1alpha1_container_override_instance.to_dict()
# create an instance of TrainerV1alpha1ContainerOverride from a dict
trainer_v1alpha1_container_override_from_dict = TrainerV1alpha1ContainerOverride.from_dict(trainer_v1alpha1_container_override_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


