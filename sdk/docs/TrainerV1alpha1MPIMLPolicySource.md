# TrainerV1alpha1MPIMLPolicySource

MPIMLPolicySource represents a MPI runtime configuration.
## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**mpi_implementation** | **str** | Implementation name for the MPI to create the appropriate hostfile. Defaults to OpenMPI. | [optional] 
**num_proc_per_node** | **int** | Number of processes per node. This value is equal to the number of slots for each node in the hostfile. | [optional] 
**run_launcher_as_node** | **bool** | Whether to run training process on the launcher Job. Defaults to false. | [optional] 
**ssh_auth_mount_path** | **str** | Directory where SSH keys are mounted. Defaults to /root/.ssh. | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


