# TrainerV1alpha1PodGroupPolicy

PodGroupPolicy represents a PodGroup configuration for gang-scheduling.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**coscheduling** | [**TrainerV1alpha1CoschedulingPodGroupPolicySource**](TrainerV1alpha1CoschedulingPodGroupPolicySource.md) |  | [optional] 

## Example

```python
from kubeflow.trainer.models.trainer_v1alpha1_pod_group_policy import TrainerV1alpha1PodGroupPolicy

# TODO update the JSON string below
json = "{}"
# create an instance of TrainerV1alpha1PodGroupPolicy from a JSON string
trainer_v1alpha1_pod_group_policy_instance = TrainerV1alpha1PodGroupPolicy.from_json(json)
# print the JSON string representation of the object
print(TrainerV1alpha1PodGroupPolicy.to_json())

# convert the object into a dict
trainer_v1alpha1_pod_group_policy_dict = trainer_v1alpha1_pod_group_policy_instance.to_dict()
# create an instance of TrainerV1alpha1PodGroupPolicy from a dict
trainer_v1alpha1_pod_group_policy_from_dict = TrainerV1alpha1PodGroupPolicy.from_dict(trainer_v1alpha1_pod_group_policy_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


