# TrainerV1alpha1OutputModel

OutputModel represents the desired trained model configuration.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**env** | [**List[IoK8sApiCoreV1EnvVar]**](IoK8sApiCoreV1EnvVar.md) | List of environment variables to set in the model exporter container. These values will be merged with the TrainingRuntime&#39;s model exporter environments. | [optional] 
**secret_ref** | [**IoK8sApiCoreV1LocalObjectReference**](IoK8sApiCoreV1LocalObjectReference.md) |  | [optional] 
**storage_uri** | **str** | Storage uri for the model exporter. | [optional] 

## Example

```python
from kubeflow.trainer.models.trainer_v1alpha1_output_model import TrainerV1alpha1OutputModel

# TODO update the JSON string below
json = "{}"
# create an instance of TrainerV1alpha1OutputModel from a JSON string
trainer_v1alpha1_output_model_instance = TrainerV1alpha1OutputModel.from_json(json)
# print the JSON string representation of the object
print(TrainerV1alpha1OutputModel.to_json())

# convert the object into a dict
trainer_v1alpha1_output_model_dict = trainer_v1alpha1_output_model_instance.to_dict()
# create an instance of TrainerV1alpha1OutputModel from a dict
trainer_v1alpha1_output_model_from_dict = TrainerV1alpha1OutputModel.from_dict(trainer_v1alpha1_output_model_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


