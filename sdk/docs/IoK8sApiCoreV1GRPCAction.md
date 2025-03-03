# IoK8sApiCoreV1GRPCAction

GRPCAction specifies an action involving a GRPC service.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**port** | **int** | Port number of the gRPC service. Number must be in the range 1 to 65535. | 
**service** | **str** | Service is the name of the service to place in the gRPC HealthCheckRequest (see https://github.com/grpc/grpc/blob/master/doc/health-checking.md).  If this is not specified, the default behavior is defined by gRPC. | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_grpc_action import IoK8sApiCoreV1GRPCAction

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1GRPCAction from a JSON string
io_k8s_api_core_v1_grpc_action_instance = IoK8sApiCoreV1GRPCAction.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1GRPCAction.to_json())

# convert the object into a dict
io_k8s_api_core_v1_grpc_action_dict = io_k8s_api_core_v1_grpc_action_instance.to_dict()
# create an instance of IoK8sApiCoreV1GRPCAction from a dict
io_k8s_api_core_v1_grpc_action_from_dict = IoK8sApiCoreV1GRPCAction.from_dict(io_k8s_api_core_v1_grpc_action_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


