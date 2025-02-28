# TrainerV1alpha1Trainer

Trainer represents the desired trainer configuration. Every training runtime contains `trainer` container which represents Trainer.
## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**args** | **list[str]** | Arguments to the entrypoint for the training container. | [optional] 
**command** | **list[str]** | Entrypoint commands for the training container. | [optional] 
**env** | [**list[V1EnvVar]**](V1EnvVar.md) | List of environment variables to set in the training container. These values will be merged with the TrainingRuntime&#39;s trainer environments. | [optional] 
**image** | **str** | Docker image for the training container. | [optional] 
**num_nodes** | **int** | Number of training nodes. | [optional] 
**num_proc_per_node** | [**Union[int, str]**](K8sIoApimachineryPkgUtilIntstrIntOrString.md) |  | [optional] 
**resources_per_node** | [**V1ResourceRequirements**](V1ResourceRequirements.md) |  | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


