# IoK8sApiCoreV1ContainerPort

ContainerPort represents a network port in a single container.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**container_port** | **int** | Number of port to expose on the pod&#39;s IP address. This must be a valid port number, 0 &lt; x &lt; 65536. | 
**host_ip** | **str** | What host IP to bind the external port to. | [optional] 
**host_port** | **int** | Number of port to expose on the host. If specified, this must be a valid port number, 0 &lt; x &lt; 65536. If HostNetwork is specified, this must match ContainerPort. Most containers do not need this. | [optional] 
**name** | **str** | If specified, this must be an IANA_SVC_NAME and unique within the pod. Each named port in a pod must have a unique name. Name for the port that can be referred to by services. | [optional] 
**protocol** | **str** | Protocol for port. Must be UDP, TCP, or SCTP. Defaults to \&quot;TCP\&quot;. | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_container_port import IoK8sApiCoreV1ContainerPort

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1ContainerPort from a JSON string
io_k8s_api_core_v1_container_port_instance = IoK8sApiCoreV1ContainerPort.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1ContainerPort.to_json())

# convert the object into a dict
io_k8s_api_core_v1_container_port_dict = io_k8s_api_core_v1_container_port_instance.to_dict()
# create an instance of IoK8sApiCoreV1ContainerPort from a dict
io_k8s_api_core_v1_container_port_from_dict = IoK8sApiCoreV1ContainerPort.from_dict(io_k8s_api_core_v1_container_port_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


