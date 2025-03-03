# TrainerV1alpha1TrainJobList

TrainJobList is a collection of training jobs.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**api_version** | **str** | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources | [optional] 
**items** | [**List[TrainerV1alpha1TrainJob]**](TrainerV1alpha1TrainJob.md) | List of TrainJobs. | 
**kind** | **str** | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds | [optional] 
**metadata** | [**IoK8sApimachineryPkgApisMetaV1ListMeta**](IoK8sApimachineryPkgApisMetaV1ListMeta.md) |  | [optional] 

## Example

```python
from kubeflow.trainer.models.trainer_v1alpha1_train_job_list import TrainerV1alpha1TrainJobList

# TODO update the JSON string below
json = "{}"
# create an instance of TrainerV1alpha1TrainJobList from a JSON string
trainer_v1alpha1_train_job_list_instance = TrainerV1alpha1TrainJobList.from_json(json)
# print the JSON string representation of the object
print(TrainerV1alpha1TrainJobList.to_json())

# convert the object into a dict
trainer_v1alpha1_train_job_list_dict = trainer_v1alpha1_train_job_list_instance.to_dict()
# create an instance of TrainerV1alpha1TrainJobList from a dict
trainer_v1alpha1_train_job_list_from_dict = TrainerV1alpha1TrainJobList.from_dict(trainer_v1alpha1_train_job_list_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


