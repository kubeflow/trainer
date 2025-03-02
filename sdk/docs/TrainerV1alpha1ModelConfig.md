# TrainerV1alpha1ModelConfig

ModelConfig represents the desired model configuration.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**input** | [**TrainerV1alpha1InputModel**](TrainerV1alpha1InputModel.md) |  | [optional] 
**output** | [**TrainerV1alpha1OutputModel**](TrainerV1alpha1OutputModel.md) |  | [optional] 

## Example

```python
from kubeflow.trainer.models.trainer_v1alpha1_model_config import TrainerV1alpha1ModelConfig

# TODO update the JSON string below
json = "{}"
# create an instance of TrainerV1alpha1ModelConfig from a JSON string
trainer_v1alpha1_model_config_instance = TrainerV1alpha1ModelConfig.from_json(json)
# print the JSON string representation of the object
print(TrainerV1alpha1ModelConfig.to_json())

# convert the object into a dict
trainer_v1alpha1_model_config_dict = trainer_v1alpha1_model_config_instance.to_dict()
# create an instance of TrainerV1alpha1ModelConfig from a dict
trainer_v1alpha1_model_config_from_dict = TrainerV1alpha1ModelConfig.from_dict(trainer_v1alpha1_model_config_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


