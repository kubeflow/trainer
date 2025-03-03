# IoK8sApiBatchV1PodFailurePolicy

PodFailurePolicy describes how failed pods influence the backoffLimit.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**rules** | [**List[IoK8sApiBatchV1PodFailurePolicyRule]**](IoK8sApiBatchV1PodFailurePolicyRule.md) | A list of pod failure policy rules. The rules are evaluated in order. Once a rule matches a Pod failure, the remaining of the rules are ignored. When no rule matches the Pod failure, the default handling applies - the counter of pod failures is incremented and it is checked against the backoffLimit. At most 20 elements are allowed. | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_batch_v1_pod_failure_policy import IoK8sApiBatchV1PodFailurePolicy

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiBatchV1PodFailurePolicy from a JSON string
io_k8s_api_batch_v1_pod_failure_policy_instance = IoK8sApiBatchV1PodFailurePolicy.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiBatchV1PodFailurePolicy.to_json())

# convert the object into a dict
io_k8s_api_batch_v1_pod_failure_policy_dict = io_k8s_api_batch_v1_pod_failure_policy_instance.to_dict()
# create an instance of IoK8sApiBatchV1PodFailurePolicy from a dict
io_k8s_api_batch_v1_pod_failure_policy_from_dict = IoK8sApiBatchV1PodFailurePolicy.from_dict(io_k8s_api_batch_v1_pod_failure_policy_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


