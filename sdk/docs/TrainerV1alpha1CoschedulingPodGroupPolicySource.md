# TrainerV1alpha1CoschedulingPodGroupPolicySource

CoschedulingPodGroupPolicySource represents configuration for coscheduling plugin. The number of min members in the PodGroupSpec is always equal to the number of nodes.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**schedule_timeout_seconds** | **int** | Time threshold to schedule PodGroup for gang-scheduling. If the scheduling timeout is equal to 0, the default value is used. Defaults to 60 seconds. | [optional] 

## Example

```python
from kubeflow.trainer.models.trainer_v1alpha1_coscheduling_pod_group_policy_source import TrainerV1alpha1CoschedulingPodGroupPolicySource

# TODO update the JSON string below
json = "{}"
# create an instance of TrainerV1alpha1CoschedulingPodGroupPolicySource from a JSON string
trainer_v1alpha1_coscheduling_pod_group_policy_source_instance = TrainerV1alpha1CoschedulingPodGroupPolicySource.from_json(json)
# print the JSON string representation of the object
print(TrainerV1alpha1CoschedulingPodGroupPolicySource.to_json())

# convert the object into a dict
trainer_v1alpha1_coscheduling_pod_group_policy_source_dict = trainer_v1alpha1_coscheduling_pod_group_policy_source_instance.to_dict()
# create an instance of TrainerV1alpha1CoschedulingPodGroupPolicySource from a dict
trainer_v1alpha1_coscheduling_pod_group_policy_source_from_dict = TrainerV1alpha1CoschedulingPodGroupPolicySource.from_dict(trainer_v1alpha1_coscheduling_pod_group_policy_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


