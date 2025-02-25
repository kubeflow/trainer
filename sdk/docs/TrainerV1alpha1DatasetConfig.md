# TrainerV1alpha1DatasetConfig

DatasetConfig represents the desired dataset configuration. When this API is used, the training runtime must have the `dataset-initializer` container in the `Initializer` Job.
## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**env** | [**list[V1EnvVar]**](V1EnvVar.md) | List of environment variables to set in the dataset initializer container. These values will be merged with the TrainingRuntime&#39;s dataset initializer environments. | [optional] 
**secret_ref** | [**V1LocalObjectReference**](V1LocalObjectReference.md) |  | [optional] 
**storage_uri** | **str** | Storage uri for the dataset provider. | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


