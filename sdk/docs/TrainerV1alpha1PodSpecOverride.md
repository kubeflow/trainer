# TrainerV1alpha1PodSpecOverride

PodSpecOverride represents the custom overrides that will be applied for the TrainJob's resources.
## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**containers** | [**list[TrainerV1alpha1ContainerOverride]**](TrainerV1alpha1ContainerOverride.md) | Overrides for the containers in the desired job templates. | [optional] 
**init_containers** | [**list[TrainerV1alpha1ContainerOverride]**](TrainerV1alpha1ContainerOverride.md) | Overrides for the init container in the desired job templates. | [optional] 
**node_selector** | **dict(str, str)** | Override for the node selector to place Pod on the specific mode. | [optional] 
**service_account_name** | **str** | Override for the service account. | [optional] 
**target_jobs** | [**list[TrainerV1alpha1PodSpecOverrideTargetJob]**](TrainerV1alpha1PodSpecOverrideTargetJob.md) | TrainJobs is the training job replicas in the training runtime template to apply the overrides. | 
**tolerations** | [**list[V1Toleration]**](V1Toleration.md) | Override for the Pod&#39;s tolerations. | [optional] 
**volumes** | [**list[V1Volume]**](V1Volume.md) | Overrides for the Pod volume configuration. | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


