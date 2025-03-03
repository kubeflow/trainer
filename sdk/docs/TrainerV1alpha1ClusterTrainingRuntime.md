# TrainerV1alpha1ClusterTrainingRuntime

ClusterTrainingRuntime represents a training runtime which can be referenced as part of `runtimeRef` API in TrainJob. This resource is a cluster-scoped and can be referenced by TrainJob that created in *any* namespace.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**api_version** | **str** | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources | [optional] 
**kind** | **str** | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds | [optional] 
**metadata** | [**IoK8sApimachineryPkgApisMetaV1ObjectMeta**](IoK8sApimachineryPkgApisMetaV1ObjectMeta.md) |  | [optional] 
**spec** | [**TrainerV1alpha1TrainingRuntimeSpec**](TrainerV1alpha1TrainingRuntimeSpec.md) |  | [optional] 

## Example

```python
from kubeflow.trainer.models.trainer_v1alpha1_cluster_training_runtime import TrainerV1alpha1ClusterTrainingRuntime

# TODO update the JSON string below
json = "{}"
# create an instance of TrainerV1alpha1ClusterTrainingRuntime from a JSON string
trainer_v1alpha1_cluster_training_runtime_instance = TrainerV1alpha1ClusterTrainingRuntime.from_json(json)
# print the JSON string representation of the object
print(TrainerV1alpha1ClusterTrainingRuntime.to_json())

# convert the object into a dict
trainer_v1alpha1_cluster_training_runtime_dict = trainer_v1alpha1_cluster_training_runtime_instance.to_dict()
# create an instance of TrainerV1alpha1ClusterTrainingRuntime from a dict
trainer_v1alpha1_cluster_training_runtime_from_dict = TrainerV1alpha1ClusterTrainingRuntime.from_dict(trainer_v1alpha1_cluster_training_runtime_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


