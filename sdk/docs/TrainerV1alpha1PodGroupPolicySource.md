# TrainerV1alpha1PodGroupPolicySource

PodGroupPolicySource represents supported plugins for gang-scheduling. Only one of its members may be specified.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**coscheduling** | [**TrainerV1alpha1CoschedulingPodGroupPolicySource**](TrainerV1alpha1CoschedulingPodGroupPolicySource.md) |  | [optional] 

## Example

```python
from kubeflow.trainer.models.trainer_v1alpha1_pod_group_policy_source import TrainerV1alpha1PodGroupPolicySource

# TODO update the JSON string below
json = "{}"
# create an instance of TrainerV1alpha1PodGroupPolicySource from a JSON string
trainer_v1alpha1_pod_group_policy_source_instance = TrainerV1alpha1PodGroupPolicySource.from_json(json)
# print the JSON string representation of the object
print(TrainerV1alpha1PodGroupPolicySource.to_json())

# convert the object into a dict
trainer_v1alpha1_pod_group_policy_source_dict = trainer_v1alpha1_pod_group_policy_source_instance.to_dict()
# create an instance of TrainerV1alpha1PodGroupPolicySource from a dict
trainer_v1alpha1_pod_group_policy_source_from_dict = TrainerV1alpha1PodGroupPolicySource.from_dict(trainer_v1alpha1_pod_group_policy_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


