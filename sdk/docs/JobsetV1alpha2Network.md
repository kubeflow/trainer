# JobsetV1alpha2Network


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**enable_dns_hostnames** | **bool** | EnableDNSHostnames allows pods to be reached via their hostnames. Pods will be reachable using the fully qualified pod hostname: &lt;jobSet.name&gt;-&lt;spec.replicatedJob.name&gt;-&lt;job-index&gt;-&lt;pod-index&gt;.&lt;subdomain&gt; | [optional] 
**publish_not_ready_addresses** | **bool** | Indicates if DNS records of pods should be published before the pods are ready. Defaults to True. | [optional] 
**subdomain** | **str** | Subdomain is an explicit choice for a network subdomain name When set, any replicated job in the set is added to this network. Defaults to &lt;jobSet.name&gt; if not set. | [optional] 

## Example

```python
from kubeflow.trainer.models.jobset_v1alpha2_network import JobsetV1alpha2Network

# TODO update the JSON string below
json = "{}"
# create an instance of JobsetV1alpha2Network from a JSON string
jobset_v1alpha2_network_instance = JobsetV1alpha2Network.from_json(json)
# print the JSON string representation of the object
print(JobsetV1alpha2Network.to_json())

# convert the object into a dict
jobset_v1alpha2_network_dict = jobset_v1alpha2_network_instance.to_dict()
# create an instance of JobsetV1alpha2Network from a dict
jobset_v1alpha2_network_from_dict = JobsetV1alpha2Network.from_dict(jobset_v1alpha2_network_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


