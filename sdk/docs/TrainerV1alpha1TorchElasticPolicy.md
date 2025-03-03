# TrainerV1alpha1TorchElasticPolicy

TorchElasticPolicy represents a configuration for the PyTorch elastic training. If this policy is set, the `.spec.numNodes` parameter must be omitted, since min and max node is used to configure the `torchrun` CLI argument: `--nnodes=minNodes:maxNodes`. Only `c10d` backend is supported for the Rendezvous communication.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**max_nodes** | **int** | Upper limit for the number of nodes to which training job can scale up. | [optional] 
**max_restarts** | **int** | How many times the training job can be restarted. This value is inserted into the &#x60;--max-restarts&#x60; argument of the &#x60;torchrun&#x60; CLI and the &#x60;.spec.failurePolicy.maxRestarts&#x60; parameter of the training Job. | [optional] 
**metrics** | [**List[IoK8sApiAutoscalingV2MetricSpec]**](IoK8sApiAutoscalingV2MetricSpec.md) | Specification which are used to calculate the desired number of nodes. See the individual metric source types for more information about how each type of metric must respond. The HPA will be created to perform auto-scaling. | [optional] 
**min_nodes** | **int** | Lower limit for the number of nodes to which training job can scale down. | [optional] 

## Example

```python
from kubeflow.trainer.models.trainer_v1alpha1_torch_elastic_policy import TrainerV1alpha1TorchElasticPolicy

# TODO update the JSON string below
json = "{}"
# create an instance of TrainerV1alpha1TorchElasticPolicy from a JSON string
trainer_v1alpha1_torch_elastic_policy_instance = TrainerV1alpha1TorchElasticPolicy.from_json(json)
# print the JSON string representation of the object
print(TrainerV1alpha1TorchElasticPolicy.to_json())

# convert the object into a dict
trainer_v1alpha1_torch_elastic_policy_dict = trainer_v1alpha1_torch_elastic_policy_instance.to_dict()
# create an instance of TrainerV1alpha1TorchElasticPolicy from a dict
trainer_v1alpha1_torch_elastic_policy_from_dict = TrainerV1alpha1TorchElasticPolicy.from_dict(trainer_v1alpha1_torch_elastic_policy_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


