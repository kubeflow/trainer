# TrainerV1alpha1DatasetConfig

DatasetConfig represents the desired dataset configuration. When this API is used, the training runtime must have the `dataset-initializer` container in the `Initializer` Job.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**env** | [**List[IoK8sApiCoreV1EnvVar]**](IoK8sApiCoreV1EnvVar.md) | List of environment variables to set in the dataset initializer container. These values will be merged with the TrainingRuntime&#39;s dataset initializer environments. | [optional] 
**secret_ref** | [**IoK8sApiCoreV1LocalObjectReference**](IoK8sApiCoreV1LocalObjectReference.md) |  | [optional] 
**storage_uri** | **str** | Storage uri for the dataset provider. | [optional] 

## Example

```python
from kubeflow.trainer.models.trainer_v1alpha1_dataset_config import TrainerV1alpha1DatasetConfig

# TODO update the JSON string below
json = "{}"
# create an instance of TrainerV1alpha1DatasetConfig from a JSON string
trainer_v1alpha1_dataset_config_instance = TrainerV1alpha1DatasetConfig.from_json(json)
# print the JSON string representation of the object
print(TrainerV1alpha1DatasetConfig.to_json())

# convert the object into a dict
trainer_v1alpha1_dataset_config_dict = trainer_v1alpha1_dataset_config_instance.to_dict()
# create an instance of TrainerV1alpha1DatasetConfig from a dict
trainer_v1alpha1_dataset_config_from_dict = TrainerV1alpha1DatasetConfig.from_dict(trainer_v1alpha1_dataset_config_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


