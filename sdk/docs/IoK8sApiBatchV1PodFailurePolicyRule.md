# IoK8sApiBatchV1PodFailurePolicyRule

PodFailurePolicyRule describes how a pod failure is handled when the requirements are met. One of onExitCodes and onPodConditions, but not both, can be used in each rule.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**action** | **str** | Specifies the action taken on a pod failure when the requirements are satisfied. Possible values are:  - FailJob: indicates that the pod&#39;s job is marked as Failed and all   running pods are terminated. - FailIndex: indicates that the pod&#39;s index is marked as Failed and will   not be restarted.   This value is beta-level. It can be used when the   &#x60;JobBackoffLimitPerIndex&#x60; feature gate is enabled (enabled by default). - Ignore: indicates that the counter towards the .backoffLimit is not   incremented and a replacement pod is created. - Count: indicates that the pod is handled in the default way - the   counter towards the .backoffLimit is incremented. Additional values are considered to be added in the future. Clients should react to an unknown action by skipping the rule. | 
**on_exit_codes** | [**IoK8sApiBatchV1PodFailurePolicyOnExitCodesRequirement**](IoK8sApiBatchV1PodFailurePolicyOnExitCodesRequirement.md) |  | [optional] 
**on_pod_conditions** | [**List[IoK8sApiBatchV1PodFailurePolicyOnPodConditionsPattern]**](IoK8sApiBatchV1PodFailurePolicyOnPodConditionsPattern.md) | Represents the requirement on the pod conditions. The requirement is represented as a list of pod condition patterns. The requirement is satisfied if at least one pattern matches an actual pod condition. At most 20 elements are allowed. | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_batch_v1_pod_failure_policy_rule import IoK8sApiBatchV1PodFailurePolicyRule

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiBatchV1PodFailurePolicyRule from a JSON string
io_k8s_api_batch_v1_pod_failure_policy_rule_instance = IoK8sApiBatchV1PodFailurePolicyRule.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiBatchV1PodFailurePolicyRule.to_json())

# convert the object into a dict
io_k8s_api_batch_v1_pod_failure_policy_rule_dict = io_k8s_api_batch_v1_pod_failure_policy_rule_instance.to_dict()
# create an instance of IoK8sApiBatchV1PodFailurePolicyRule from a dict
io_k8s_api_batch_v1_pod_failure_policy_rule_from_dict = IoK8sApiBatchV1PodFailurePolicyRule.from_dict(io_k8s_api_batch_v1_pod_failure_policy_rule_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


