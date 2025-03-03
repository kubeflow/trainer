# IoK8sApiCoreV1PodReadinessGate

PodReadinessGate contains the reference to a pod condition

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**condition_type** | **str** | ConditionType refers to a condition in the pod&#39;s condition list with matching type. | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_pod_readiness_gate import IoK8sApiCoreV1PodReadinessGate

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1PodReadinessGate from a JSON string
io_k8s_api_core_v1_pod_readiness_gate_instance = IoK8sApiCoreV1PodReadinessGate.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1PodReadinessGate.to_json())

# convert the object into a dict
io_k8s_api_core_v1_pod_readiness_gate_dict = io_k8s_api_core_v1_pod_readiness_gate_instance.to_dict()
# create an instance of IoK8sApiCoreV1PodReadinessGate from a dict
io_k8s_api_core_v1_pod_readiness_gate_from_dict = IoK8sApiCoreV1PodReadinessGate.from_dict(io_k8s_api_core_v1_pod_readiness_gate_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


