# TrainerV1alpha1OutputModel

OutputModel represents the desired trained model configuration.
## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**env** | [**list[V1EnvVar]**](V1EnvVar.md) | List of environment variables to set in the model exporter container. These values will be merged with the TrainingRuntime&#39;s model exporter environments. | [optional] 
**secret_ref** | [**V1LocalObjectReference**](V1LocalObjectReference.md) |  | [optional] 
**storage_uri** | **str** | Storage uri for the model exporter. | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


