# TrainerV1alpha1TorchElasticPolicy

TorchElasticPolicy represents a configuration for the PyTorch elastic training. If this policy is set, the `.spec.numNodes` parameter must be omitted, since min and max node is used to configure the `torchrun` CLI argument: `--nnodes=minNodes:maxNodes`. Only `c10d` backend is supported for the Rendezvous communication.
## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**max_nodes** | **int** | Upper limit for the number of nodes to which training job can scale up. | [optional] 
**max_restarts** | **int** | How many times the training job can be restarted. This value is inserted into the &#x60;--max-restarts&#x60; argument of the &#x60;torchrun&#x60; CLI and the &#x60;.spec.failurePolicy.maxRestarts&#x60; parameter of the training Job. | [optional] 
**metrics** | [**list[V2MetricSpec]**](K8sIoApiAutoscalingV2MetricSpec.md) | Specification which are used to calculate the desired number of nodes. See the individual metric source types for more information about how each type of metric must respond. The HPA will be created to perform auto-scaling. | [optional] 
**min_nodes** | **int** | Lower limit for the number of nodes to which training job can scale down. | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


