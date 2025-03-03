# IoK8sApiAutoscalingV2CrossVersionObjectReference

CrossVersionObjectReference contains enough information to let you identify the referred resource.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**api_version** | **str** | apiVersion is the API version of the referent | [optional] 
**kind** | **str** | kind is the kind of the referent; More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds | 
**name** | **str** | name is the name of the referent; More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names | 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_autoscaling_v2_cross_version_object_reference import IoK8sApiAutoscalingV2CrossVersionObjectReference

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiAutoscalingV2CrossVersionObjectReference from a JSON string
io_k8s_api_autoscaling_v2_cross_version_object_reference_instance = IoK8sApiAutoscalingV2CrossVersionObjectReference.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiAutoscalingV2CrossVersionObjectReference.to_json())

# convert the object into a dict
io_k8s_api_autoscaling_v2_cross_version_object_reference_dict = io_k8s_api_autoscaling_v2_cross_version_object_reference_instance.to_dict()
# create an instance of IoK8sApiAutoscalingV2CrossVersionObjectReference from a dict
io_k8s_api_autoscaling_v2_cross_version_object_reference_from_dict = IoK8sApiAutoscalingV2CrossVersionObjectReference.from_dict(io_k8s_api_autoscaling_v2_cross_version_object_reference_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


