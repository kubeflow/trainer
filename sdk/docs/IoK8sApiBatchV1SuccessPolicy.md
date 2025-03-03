# IoK8sApiBatchV1SuccessPolicy

SuccessPolicy describes when a Job can be declared as succeeded based on the success of some indexes.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**rules** | [**List[IoK8sApiBatchV1SuccessPolicyRule]**](IoK8sApiBatchV1SuccessPolicyRule.md) | rules represents the list of alternative rules for the declaring the Jobs as successful before &#x60;.status.succeeded &gt;&#x3D; .spec.completions&#x60;. Once any of the rules are met, the \&quot;SucceededCriteriaMet\&quot; condition is added, and the lingering pods are removed. The terminal state for such a Job has the \&quot;Complete\&quot; condition. Additionally, these rules are evaluated in order; Once the Job meets one of the rules, other rules are ignored. At most 20 elements are allowed. | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_batch_v1_success_policy import IoK8sApiBatchV1SuccessPolicy

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiBatchV1SuccessPolicy from a JSON string
io_k8s_api_batch_v1_success_policy_instance = IoK8sApiBatchV1SuccessPolicy.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiBatchV1SuccessPolicy.to_json())

# convert the object into a dict
io_k8s_api_batch_v1_success_policy_dict = io_k8s_api_batch_v1_success_policy_instance.to_dict()
# create an instance of IoK8sApiBatchV1SuccessPolicy from a dict
io_k8s_api_batch_v1_success_policy_from_dict = IoK8sApiBatchV1SuccessPolicy.from_dict(io_k8s_api_batch_v1_success_policy_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


