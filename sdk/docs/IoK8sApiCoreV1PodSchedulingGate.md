# IoK8sApiCoreV1PodSchedulingGate

PodSchedulingGate is associated to a Pod to guard its scheduling.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**name** | **str** | Name of the scheduling gate. Each scheduling gate must have a unique name field. | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_pod_scheduling_gate import IoK8sApiCoreV1PodSchedulingGate

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1PodSchedulingGate from a JSON string
io_k8s_api_core_v1_pod_scheduling_gate_instance = IoK8sApiCoreV1PodSchedulingGate.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1PodSchedulingGate.to_json())

# convert the object into a dict
io_k8s_api_core_v1_pod_scheduling_gate_dict = io_k8s_api_core_v1_pod_scheduling_gate_instance.to_dict()
# create an instance of IoK8sApiCoreV1PodSchedulingGate from a dict
io_k8s_api_core_v1_pod_scheduling_gate_from_dict = IoK8sApiCoreV1PodSchedulingGate.from_dict(io_k8s_api_core_v1_pod_scheduling_gate_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


