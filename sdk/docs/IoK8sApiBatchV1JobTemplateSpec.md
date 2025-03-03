# IoK8sApiBatchV1JobTemplateSpec

JobTemplateSpec describes the data a Job should have when created from a template

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**metadata** | [**IoK8sApimachineryPkgApisMetaV1ObjectMeta**](IoK8sApimachineryPkgApisMetaV1ObjectMeta.md) |  | [optional] 
**spec** | [**IoK8sApiBatchV1JobSpec**](IoK8sApiBatchV1JobSpec.md) |  | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_batch_v1_job_template_spec import IoK8sApiBatchV1JobTemplateSpec

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiBatchV1JobTemplateSpec from a JSON string
io_k8s_api_batch_v1_job_template_spec_instance = IoK8sApiBatchV1JobTemplateSpec.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiBatchV1JobTemplateSpec.to_json())

# convert the object into a dict
io_k8s_api_batch_v1_job_template_spec_dict = io_k8s_api_batch_v1_job_template_spec_instance.to_dict()
# create an instance of IoK8sApiBatchV1JobTemplateSpec from a dict
io_k8s_api_batch_v1_job_template_spec_from_dict = IoK8sApiBatchV1JobTemplateSpec.from_dict(io_k8s_api_batch_v1_job_template_spec_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


