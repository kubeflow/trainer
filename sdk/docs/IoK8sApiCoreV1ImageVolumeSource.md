# IoK8sApiCoreV1ImageVolumeSource

ImageVolumeSource represents a image volume resource.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**pull_policy** | **str** | Policy for pulling OCI objects. Possible values are: Always: the kubelet always attempts to pull the reference. Container creation will fail If the pull fails. Never: the kubelet never pulls the reference and only uses a local image or artifact. Container creation will fail if the reference isn&#39;t present. IfNotPresent: the kubelet pulls if the reference isn&#39;t already present on disk. Container creation will fail if the reference isn&#39;t present and the pull fails. Defaults to Always if :latest tag is specified, or IfNotPresent otherwise. | [optional] 
**reference** | **str** | Required: Image or artifact reference to be used. Behaves in the same way as pod.spec.containers[*].image. Pull secrets will be assembled in the same way as for the container image by looking up node credentials, SA image pull secrets, and pod spec image pull secrets. More info: https://kubernetes.io/docs/concepts/containers/images This field is optional to allow higher level config management to default or override container images in workload controllers like Deployments and StatefulSets. | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_image_volume_source import IoK8sApiCoreV1ImageVolumeSource

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1ImageVolumeSource from a JSON string
io_k8s_api_core_v1_image_volume_source_instance = IoK8sApiCoreV1ImageVolumeSource.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1ImageVolumeSource.to_json())

# convert the object into a dict
io_k8s_api_core_v1_image_volume_source_dict = io_k8s_api_core_v1_image_volume_source_instance.to_dict()
# create an instance of IoK8sApiCoreV1ImageVolumeSource from a dict
io_k8s_api_core_v1_image_volume_source_from_dict = IoK8sApiCoreV1ImageVolumeSource.from_dict(io_k8s_api_core_v1_image_volume_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


