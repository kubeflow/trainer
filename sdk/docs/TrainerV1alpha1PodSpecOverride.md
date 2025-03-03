# TrainerV1alpha1PodSpecOverride

PodSpecOverride represents the custom overrides that will be applied for the TrainJob's resources.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**containers** | [**List[TrainerV1alpha1ContainerOverride]**](TrainerV1alpha1ContainerOverride.md) | Overrides for the containers in the desired job templates. | [optional] 
**init_containers** | [**List[TrainerV1alpha1ContainerOverride]**](TrainerV1alpha1ContainerOverride.md) | Overrides for the init container in the desired job templates. | [optional] 
**node_selector** | **Dict[str, str]** | Override for the node selector to place Pod on the specific mode. | [optional] 
**service_account_name** | **str** | Override for the service account. | [optional] 
**target_jobs** | [**List[TrainerV1alpha1PodSpecOverrideTargetJob]**](TrainerV1alpha1PodSpecOverrideTargetJob.md) | TrainJobs is the training job replicas in the training runtime template to apply the overrides. | 
**tolerations** | [**List[IoK8sApiCoreV1Toleration]**](IoK8sApiCoreV1Toleration.md) | Override for the Pod&#39;s tolerations. | [optional] 
**volumes** | [**List[IoK8sApiCoreV1Volume]**](IoK8sApiCoreV1Volume.md) | Overrides for the Pod volume configuration. | [optional] 

## Example

```python
from kubeflow.trainer.models.trainer_v1alpha1_pod_spec_override import TrainerV1alpha1PodSpecOverride

# TODO update the JSON string below
json = "{}"
# create an instance of TrainerV1alpha1PodSpecOverride from a JSON string
trainer_v1alpha1_pod_spec_override_instance = TrainerV1alpha1PodSpecOverride.from_json(json)
# print the JSON string representation of the object
print(TrainerV1alpha1PodSpecOverride.to_json())

# convert the object into a dict
trainer_v1alpha1_pod_spec_override_dict = trainer_v1alpha1_pod_spec_override_instance.to_dict()
# create an instance of TrainerV1alpha1PodSpecOverride from a dict
trainer_v1alpha1_pod_spec_override_from_dict = TrainerV1alpha1PodSpecOverride.from_dict(trainer_v1alpha1_pod_spec_override_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


