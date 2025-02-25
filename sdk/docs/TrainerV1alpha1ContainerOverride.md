# TrainerV1alpha1ContainerOverride

ContainerOverride represents parameters that can be overridden using PodSpecOverrides. Parameters from the Trainer, DatasetConfig, and ModelConfig will take precedence.
## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**args** | **list[str]** | Arguments to the entrypoint for the training container. | [optional] 
**command** | **list[str]** | Entrypoint commands for the training container. | [optional] 
**env** | [**list[V1EnvVar]**](V1EnvVar.md) | List of environment variables to set in the container. These values will be merged with the TrainingRuntime&#39;s environments. | [optional] 
**env_from** | [**list[V1EnvFromSource]**](V1EnvFromSource.md) | List of sources to populate environment variables in the container. These   values will be merged with the TrainingRuntime&#39;s environments. | [optional] 
**name** | **str** | Name for the container. TrainingRuntime must have this container. | [default to '']
**volume_mounts** | [**list[V1VolumeMount]**](V1VolumeMount.md) | Pod volumes to mount into the container&#39;s filesystem. | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


