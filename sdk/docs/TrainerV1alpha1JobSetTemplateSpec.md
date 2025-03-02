# TrainerV1alpha1JobSetTemplateSpec

JobSetTemplateSpec represents a template of the desired JobSet.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**metadata** | [**V1ObjectMeta**](V1ObjectMeta.md) |  | [optional] 
**spec** | [**JobsetV1alpha2JobSetSpec**](JobsetV1alpha2JobSetSpec.md) |  | [optional] 

## Example

```python
from kubeflow.trainer.models.trainer_v1alpha1_job_set_template_spec import TrainerV1alpha1JobSetTemplateSpec

# TODO update the JSON string below
json = "{}"
# create an instance of TrainerV1alpha1JobSetTemplateSpec from a JSON string
trainer_v1alpha1_job_set_template_spec_instance = TrainerV1alpha1JobSetTemplateSpec.from_json(json)
# print the JSON string representation of the object
print(TrainerV1alpha1JobSetTemplateSpec.to_json())

# convert the object into a dict
trainer_v1alpha1_job_set_template_spec_dict = trainer_v1alpha1_job_set_template_spec_instance.to_dict()
# create an instance of TrainerV1alpha1JobSetTemplateSpec from a dict
trainer_v1alpha1_job_set_template_spec_from_dict = TrainerV1alpha1JobSetTemplateSpec.from_dict(trainer_v1alpha1_job_set_template_spec_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


