# TrainerV1alpha1MPIMLPolicySource

MPIMLPolicySource represents a MPI runtime configuration.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**mpi_implementation** | **str** | Implementation name for the MPI to create the appropriate hostfile. Defaults to OpenMPI. | [optional] 
**num_proc_per_node** | **int** | Number of processes per node. This value is equal to the number of slots for each node in the hostfile. | [optional] 
**run_launcher_as_node** | **bool** | Whether to run training process on the launcher Job. Defaults to false. | [optional] 
**ssh_auth_mount_path** | **str** | Directory where SSH keys are mounted. Defaults to /root/.ssh. | [optional] 

## Example

```python
from kubeflow.trainer.models.trainer_v1alpha1_mpiml_policy_source import TrainerV1alpha1MPIMLPolicySource

# TODO update the JSON string below
json = "{}"
# create an instance of TrainerV1alpha1MPIMLPolicySource from a JSON string
trainer_v1alpha1_mpiml_policy_source_instance = TrainerV1alpha1MPIMLPolicySource.from_json(json)
# print the JSON string representation of the object
print(TrainerV1alpha1MPIMLPolicySource.to_json())

# convert the object into a dict
trainer_v1alpha1_mpiml_policy_source_dict = trainer_v1alpha1_mpiml_policy_source_instance.to_dict()
# create an instance of TrainerV1alpha1MPIMLPolicySource from a dict
trainer_v1alpha1_mpiml_policy_source_from_dict = TrainerV1alpha1MPIMLPolicySource.from_dict(trainer_v1alpha1_mpiml_policy_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


