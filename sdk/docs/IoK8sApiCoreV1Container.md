# IoK8sApiCoreV1Container

A single application container that you want to run within a pod.

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**args** | **List[str]** | Arguments to the entrypoint. The container image&#39;s CMD is used if this is not provided. Variable references $(VAR_NAME) are expanded using the container&#39;s environment. If a variable cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. \&quot;$$(VAR_NAME)\&quot; will produce the string literal \&quot;$(VAR_NAME)\&quot;. Escaped references will never be expanded, regardless of whether the variable exists or not. Cannot be updated. More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell | [optional] 
**command** | **List[str]** | Entrypoint array. Not executed within a shell. The container image&#39;s ENTRYPOINT is used if this is not provided. Variable references $(VAR_NAME) are expanded using the container&#39;s environment. If a variable cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. \&quot;$$(VAR_NAME)\&quot; will produce the string literal \&quot;$(VAR_NAME)\&quot;. Escaped references will never be expanded, regardless of whether the variable exists or not. Cannot be updated. More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell | [optional] 
**env** | [**List[IoK8sApiCoreV1EnvVar]**](IoK8sApiCoreV1EnvVar.md) | List of environment variables to set in the container. Cannot be updated. | [optional] 
**env_from** | [**List[IoK8sApiCoreV1EnvFromSource]**](IoK8sApiCoreV1EnvFromSource.md) | List of sources to populate environment variables in the container. The keys defined within a source must be a C_IDENTIFIER. All invalid keys will be reported as an event when the container is starting. When a key exists in multiple sources, the value associated with the last source will take precedence. Values defined by an Env with a duplicate key will take precedence. Cannot be updated. | [optional] 
**image** | **str** | Container image name. More info: https://kubernetes.io/docs/concepts/containers/images This field is optional to allow higher level config management to default or override container images in workload controllers like Deployments and StatefulSets. | [optional] 
**image_pull_policy** | **str** | Image pull policy. One of Always, Never, IfNotPresent. Defaults to Always if :latest tag is specified, or IfNotPresent otherwise. Cannot be updated. More info: https://kubernetes.io/docs/concepts/containers/images#updating-images | [optional] 
**lifecycle** | [**IoK8sApiCoreV1Lifecycle**](IoK8sApiCoreV1Lifecycle.md) |  | [optional] 
**liveness_probe** | [**IoK8sApiCoreV1Probe**](IoK8sApiCoreV1Probe.md) |  | [optional] 
**name** | **str** | Name of the container specified as a DNS_LABEL. Each container in a pod must have a unique name (DNS_LABEL). Cannot be updated. | 
**ports** | [**List[IoK8sApiCoreV1ContainerPort]**](IoK8sApiCoreV1ContainerPort.md) | List of ports to expose from the container. Not specifying a port here DOES NOT prevent that port from being exposed. Any port which is listening on the default \&quot;0.0.0.0\&quot; address inside a container will be accessible from the network. Modifying this array with strategic merge patch may corrupt the data. For more information See https://github.com/kubernetes/kubernetes/issues/108255. Cannot be updated. | [optional] 
**readiness_probe** | [**IoK8sApiCoreV1Probe**](IoK8sApiCoreV1Probe.md) |  | [optional] 
**resize_policy** | [**List[IoK8sApiCoreV1ContainerResizePolicy]**](IoK8sApiCoreV1ContainerResizePolicy.md) | Resources resize policy for the container. | [optional] 
**resources** | [**IoK8sApiCoreV1ResourceRequirements**](IoK8sApiCoreV1ResourceRequirements.md) |  | [optional] 
**restart_policy** | **str** | RestartPolicy defines the restart behavior of individual containers in a pod. This field may only be set for init containers, and the only allowed value is \&quot;Always\&quot;. For non-init containers or when this field is not specified, the restart behavior is defined by the Pod&#39;s restart policy and the container type. Setting the RestartPolicy as \&quot;Always\&quot; for the init container will have the following effect: this init container will be continually restarted on exit until all regular containers have terminated. Once all regular containers have completed, all init containers with restartPolicy \&quot;Always\&quot; will be shut down. This lifecycle differs from normal init containers and is often referred to as a \&quot;sidecar\&quot; container. Although this init container still starts in the init container sequence, it does not wait for the container to complete before proceeding to the next init container. Instead, the next init container starts immediately after this init container is started, or after any startupProbe has successfully completed. | [optional] 
**security_context** | [**IoK8sApiCoreV1SecurityContext**](IoK8sApiCoreV1SecurityContext.md) |  | [optional] 
**startup_probe** | [**IoK8sApiCoreV1Probe**](IoK8sApiCoreV1Probe.md) |  | [optional] 
**stdin** | **bool** | Whether this container should allocate a buffer for stdin in the container runtime. If this is not set, reads from stdin in the container will always result in EOF. Default is false. | [optional] 
**stdin_once** | **bool** | Whether the container runtime should close the stdin channel after it has been opened by a single attach. When stdin is true the stdin stream will remain open across multiple attach sessions. If stdinOnce is set to true, stdin is opened on container start, is empty until the first client attaches to stdin, and then remains open and accepts data until the client disconnects, at which time stdin is closed and remains closed until the container is restarted. If this flag is false, a container processes that reads from stdin will never receive an EOF. Default is false | [optional] 
**termination_message_path** | **str** | Optional: Path at which the file to which the container&#39;s termination message will be written is mounted into the container&#39;s filesystem. Message written is intended to be brief final status, such as an assertion failure message. Will be truncated by the node if greater than 4096 bytes. The total message length across all containers will be limited to 12kb. Defaults to /dev/termination-log. Cannot be updated. | [optional] 
**termination_message_policy** | **str** | Indicate how the termination message should be populated. File will use the contents of terminationMessagePath to populate the container status message on both success and failure. FallbackToLogsOnError will use the last chunk of container log output if the termination message file is empty and the container exited with an error. The log output is limited to 2048 bytes or 80 lines, whichever is smaller. Defaults to File. Cannot be updated. | [optional] 
**tty** | **bool** | Whether this container should allocate a TTY for itself, also requires &#39;stdin&#39; to be true. Default is false. | [optional] 
**volume_devices** | [**List[IoK8sApiCoreV1VolumeDevice]**](IoK8sApiCoreV1VolumeDevice.md) | volumeDevices is the list of block devices to be used by the container. | [optional] 
**volume_mounts** | [**List[IoK8sApiCoreV1VolumeMount]**](IoK8sApiCoreV1VolumeMount.md) | Pod volumes to mount into the container&#39;s filesystem. Cannot be updated. | [optional] 
**working_dir** | **str** | Container&#39;s working directory. If not specified, the container runtime&#39;s default will be used, which might be configured in the container image. Cannot be updated. | [optional] 

## Example

```python
from kubeflow.trainer.models.io_k8s_api_core_v1_container import IoK8sApiCoreV1Container

# TODO update the JSON string below
json = "{}"
# create an instance of IoK8sApiCoreV1Container from a JSON string
io_k8s_api_core_v1_container_instance = IoK8sApiCoreV1Container.from_json(json)
# print the JSON string representation of the object
print(IoK8sApiCoreV1Container.to_json())

# convert the object into a dict
io_k8s_api_core_v1_container_dict = io_k8s_api_core_v1_container_instance.to_dict()
# create an instance of IoK8sApiCoreV1Container from a dict
io_k8s_api_core_v1_container_from_dict = IoK8sApiCoreV1Container.from_dict(io_k8s_api_core_v1_container_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


