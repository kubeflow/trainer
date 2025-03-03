# TrainerV1alpha1InputModel

InputModel represents the desired pre-trained model configuration.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**env** | [**List[IoK8sApiCoreV1EnvVar]**](IoK8sApiCoreV1EnvVar.md) | List of environment variables to set in the model initializer container. These values will be merged with the TrainingRuntime&#39;s model initializer environments. | [optional] 
**secret_ref** | [**IoK8sApiCoreV1LocalObjectReference**](IoK8sApiCoreV1LocalObjectReference.md) |  | [optional] 
**storage_uri** | **str** | Storage uri for the model provider. | [optional] 

## Example

```python
from kubeflow.trainer.models.trainer_v1alpha1_input_model import TrainerV1alpha1InputModel

# TODO update the JSON string below
json = "{}"
# create an instance of TrainerV1alpha1InputModel from a JSON string
trainer_v1alpha1_input_model_instance = TrainerV1alpha1InputModel.from_json(json)
# print the JSON string representation of the object
print(TrainerV1alpha1InputModel.to_json())

# convert the object into a dict
trainer_v1alpha1_input_model_dict = trainer_v1alpha1_input_model_instance.to_dict()
# create an instance of TrainerV1alpha1InputModel from a dict
trainer_v1alpha1_input_model_from_dict = TrainerV1alpha1InputModel.from_dict(trainer_v1alpha1_input_model_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


