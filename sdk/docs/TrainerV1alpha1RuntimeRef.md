# TrainerV1alpha1RuntimeRef

RuntimeRef represents the reference to the existing training runtime.
## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**api_group** | **str** | APIGroup of the runtime being referenced. Defaults to &#x60;trainer.kubeflow.org&#x60;. | [optional] 
**kind** | **str** | Kind of the runtime being referenced. Defaults to ClusterTrainingRuntime. | [optional] 
**name** | **str** | Name of the runtime being referenced. When namespaced-scoped TrainingRuntime is used, the TrainJob must have the same namespace as the deployed runtime. | [default to '']

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


