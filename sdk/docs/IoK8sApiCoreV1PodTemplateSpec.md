# IoK8sApiCoreV1PodTemplateSpec

PodTemplateSpec describes the data a pod should have when created from a template

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**metadata** | [**IoK8sApimachineryPkgApisMetaV1ObjectMeta**](IoK8sApimachineryPkgApisMetaV1ObjectMeta.md) |  | [optional] 
**spec** | [**IoK8sApiCoreV1PodSpec**](IoK8sApiCoreV1PodSpec.md) |  | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_pod_template_spec import IoK8sApiCoreV1PodTemplateSpec

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1PodTemplateSpec from a JSON string
io_k8s_api_core_v1_pod_template_spec_instance = IoK8sApiCoreV1PodTemplateSpec.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1PodTemplateSpec.to_json())

# convert the object into a dict
io_k8s_api_core_v1_pod_template_spec_dict = io_k8s_api_core_v1_pod_template_spec_instance.to_dict()
# create an instance of IoK8sApiCoreV1PodTemplateSpec from a dict
io_k8s_api_core_v1_pod_template_spec_from_dict = IoK8sApiCoreV1PodTemplateSpec.from_dict(io_k8s_api_core_v1_pod_template_spec_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


