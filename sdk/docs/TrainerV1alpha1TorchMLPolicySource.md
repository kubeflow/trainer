# TrainerV1alpha1TorchMLPolicySource

TorchMLPolicySource represents a PyTorch runtime configuration.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**elastic_policy** | [**TrainerV1alpha1TorchElasticPolicy**](TrainerV1alpha1TorchElasticPolicy.md) |  | [optional] 
**num_proc_per_node** | [**object**](K8sIoApimachineryPkgUtilIntstrIntOrString.md) |  | [optional] 

## Example

```python
from kubeflow.trainer.models.trainer_v1alpha1_torch_ml_policy_source import TrainerV1alpha1TorchMLPolicySource

# TODO update the JSON string below
json = "{}"
# create an instance of TrainerV1alpha1TorchMLPolicySource from a JSON string
trainer_v1alpha1_torch_ml_policy_source_instance = TrainerV1alpha1TorchMLPolicySource.from_json(json)
# print the JSON string representation of the object
print(TrainerV1alpha1TorchMLPolicySource.to_json())

# convert the object into a dict
trainer_v1alpha1_torch_ml_policy_source_dict = trainer_v1alpha1_torch_ml_policy_source_instance.to_dict()
# create an instance of TrainerV1alpha1TorchMLPolicySource from a dict
trainer_v1alpha1_torch_ml_policy_source_from_dict = TrainerV1alpha1TorchMLPolicySource.from_dict(trainer_v1alpha1_torch_ml_policy_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


