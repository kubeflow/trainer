# KEP-263: Kubeflow PVC Template Support

## Table of Contents

- [Summary](#summary)
- [Motivation](#motivation)
  - [User Stories](#user-stories-optional)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
  - [Implementation Details](#implementation-details)
  - [Risks and Mitigations](#risks-and-mitigations)
- [Testing Plan](#testing-plan)

## Summary

Kubeflow cannot automatically provision volume for user Pods as their working space. This doc proposes PVC
template support which adds an automatic volume provisioning option to user.

## Motivation

Machine learning related workloads usually requires to process large amount of data sets for training purpose, Tensorflow,
PyTorch are just example of them, and after the workload finishes, the data is usually discarded. Today, to provide such
scratch space for KubeFlow workloads, user would have the following options:

- Use host disk such as `EmptyDir` or `HostPath`
- Mount shared file system such as `AWS EFS`
- Pre-provision block devices such as `AWS EBS`
- Implement customized volume provisioning logics via `CSI` or `FlexVolume`

For the above options, each in its own way has cons:

- EmptyDir
  - Requires careful host disk space pre-provisioning
  - Scheduling multiple training jobs onto same host might cause tension in host disk space since default
    kube-scheduler does not take `EmptyDir` size into consideration
  - If training job gets retried and scheduled onto a different host, it need to fetch all its data again
- Shared file system
  - Throughput bottleneck, as jobs might use storage in a bursty fashion during phases such as downloading training data
  - Shared file system is usually expensive
- Pre provisioned block device
  - Requires additional manual / automated work for provisioning devices and synchronize volume naming for replica
- CSI / FlexVolume
  - Additional dev work would be required.
  - [CSI just GA-ed](https://kubernetes.io/blog/2019/01/15/container-storage-interface-ga/), and has not been widely adopped yet
  - Flex volume is out of tree and is [deprecated](https://github.com/kubernetes/community/blob/master/sig-storage/volume-plugin-faq.md)

While block device is the best volume choice as the scratch space of training job, and k8s has native support for auto
block device provisioning through [Persistent Volume Claim](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#lifecycle-of-a-volume-and-claim),
it would be a easy leverage for kubeflow to support dynamic scratch volume provisioning for training jobs.

### User Stories

- As a data scientist / engineer, I want to use arbitrary disk type and size for my different training jobs
- As a data scientist / engineer, I want the infrastructure of my training jobs get automatically handled and I don't need to worry about provisioning and recycling them
- As an infrastructure developer, I don't want to have user application to use host disk space with unlimited space as this increases the risk of using up host disk and affect stability
- As an infrastructure developer, I want to provision infrastructure to my customers only when needed or there will be a waste of money

### Goals

- Automatically provision / de-provision block device as scratch space for user in an efficient and reliable way
- Design should be generic enough so any distributed training job can take this feature
- When a pod gets retried (recreated), it should get it's volume back so it has the option to continue with its work
- User don't have to define volume for every single replica of worker
- Volume creation throttling
  - This has to be added for volume provisioning feature to be production ready since most cloud provider throttles
    API calls per account - if there is a burst of volume creation and cloud provider started to throttle calls, all
    clusters under the entire cloud account would get affected, and massive back-offs are hard to handle given the
    error might propagate through upstream services

### Non-Goals

- Volume pooling for reducing Create/Delete calls as we can re-use volume for different jobs
- Smart throttling such that one training job will not starve other ones due to volume throttling, similar for different
  worker types within same training job
- [Volume resizing](https://kubernetes.io/blog/2018/07/12/resizing-persistent-volumes-using-kubernetes/)
- Volume reclaim policy, i.e. user can choose if they want their PVC gets deleted after deleting the training job

## Proposal

### Implementation Details

I suggest that we start to support dynamic volume provisioning next version release, and change will not be back ported
to older versions. Currently we need to modify `v1.ReplicaSpec`, and add volume support in job controller, that is going
to be commonly shared by any type of distributed training job controller.

#### API Change

Similar to stateful set, we will add a field in `common.ReplicaSpec` to define PVC template:

```go
// ReplicaSpec is a description of the replica
type ReplicaSpec struct {
	// ...

	// VolumeClaimTemplates specifies a list of volume claim templates, which defines volumes specified in template.
	VolumeClaimTemplates []corev1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`
}
```

List is used here instead of map since according to Kubernetes convention,
[lists is preferred over maps](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#lists-of-named-subobjects-preferred-over-maps).
In this case, if user redefine volume claim templates with same name but different spec, **the actual outcome will be undefined**.

After v1, `common.ReplicaSpec` will be able to be shared with all types of controllers.

#### PVC Management

VolumeClaimTemplate will be per worker type, which means that all replicas will have same volume configuration. The exact
volume type will be up to the user through storage class, not our responsibility.

To request a volume defined by VolumeClaimTemplates, one need to specify the volume in `ReplicaSpec.Template.Spec.Containers[].VolumeMounts`,
and provide the template in `ReplicaSpec.VolumeClaimTemplates` with **the same volume name**.

**NOTE:** Volume defined in `ReplicaSpec.Template.Spec.Volumes` will override volume with same name in
`ReplicaSpec.VolumeClaimTemplates`

To avoid conflict of volume naming, the PVC created will have format `{volname}-{podname}`. If this combination
does not fit requirement, i.e. name too long, we will have error creating the volume, and in this case, related Pods will
stop progress in a way such that they stuck in `Pending` state as scheduler is unable to find the volume for scheduling.
In such case, event will be recorded for debugging purpose.

Volume's ownerReference will be set to the corresponding distributed training job, and once volume is created, we will
never delete the volume when recreating pods. Since Pod's ordinal will not change, recreated pods will always get its
volume back.

Here is an example of TFJob:

```yaml
# This is a snippet of TFJobSpec
tfReplicaSpecs:
  Worker:
    replicas: 1
    template:
      metadata:
        name: "my-worker"
      spec:
        containers:
          - name: "my-container"
            image: "some-image:latest"
            volumeMounts:
              - name: "my-data"
                path: "/data"
    volumeClaimTemplates:
      - metadata:
          name: "data"
        spec:
          storageClassName: "my-ebs-vol"
          accessModes:
            - "ReadWriteOnce"
          resources:
            requests:
              storage: 100Gi
```

We will generate volume definition in the actual Pod spec (only showing worker-0, pod and volume will be identical for
worker-1 besides the ordinal will change from 0 to 1):

```yaml
# worker 0 pod
apiVersion: v1
kind: Pod
metadata:
  name: "mytf-worker-0"
spec:
  containers:
    - name: "my-container"
      image: "some-image:latest"
      volumeMounts:
        - name: "my-data"
          path: "/data"
  volumes:
    - name: "my-data"
      persistentVolumeClaim:
        claimName: "my-data-mytf-worker-0"

---
# worker 0 pvc
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: "my-data-mytf-worker-0"
  ownerReferences:
    - apiVersion: kubeflow.org/v1beta1
      blockOwnerDeletion: true
      controller: true
      kind: TFJob
      name: "mytf"
      uid: "xxxxxxx"
spec:
  accessModes:
    - "ReadWriteOnce"
  resources:
    requests:
      storage: 100Gi
  storageClassName: "my-ebs-vol"
```

Once Pod and PVC are created, k8s control plane will handle the rest of the coordination.

#### Volume Provisioning / De-provisioning Throttling

##### Provision

Though k8s PV controller has relatively aggressive exponential and jittered back-off when making cloud provider calls,
a burst in creating PVC will still result in a burst in cloud provider calls. Therefore, cluster administrator should
be able to configure the rate each application should use to create PVC.

##### De-provision

We do not need to have a centralized throttle of volume de-provision for the following 2 reasons:

1. Volume de-provision is relatively light weighted (only contains unmount and detach, not necessarily delete volume)
2. Volume de-provision happens in a distributed fashion and have more randomness on timing, as container tear-down time varies

We can rely on k8s garbage collector to recycle Pod and Volume when a training job is deleted, and kubelet will ensure
that it does not start volume tear down routine before all containers are terminated.

##### Throttling

With volume creation throttling, controller will send creation request that contains Pod/PVC pair to a queue and let the
volume and pod to be created asynchronously. We always create volume first before we create Pod. The reason is this will
make control plane more efficient as Pod without its volume get created first will make k8s controller / scheduler retry
and generate other warning metrics / events, which adds load to important services such as ETCD and is not necessary in
this case. Async worker creation request will look like the follows:

```go
type podVolumeThrottleRequest struct {
	Pod *v1.Pod
	Vol *v1.PersistentVolume
}
```

A worker will spin up that takes requests from queue and create PVC with throttled rate. K8s provides
well tested [token bucket filter through its client-go](https://godoc.org/k8s.io/client-go/util/flowcontrol#NewTokenBucketRateLimiter),
and we will be able to use that directly. Worker will requeue failed requests with back-off. With this throttling, worker
will have head-of-line blocking globally (among all training jobs controlled by the controller). This is intended behavior
as we want to have a global volume creation throttling.
Also, this worker should be able to be commonly shared among different types of controllers so should reside in `common`.

##### Alternatives

1. Where to throttle volume creation?
   An alternative is to work with k8s upstream to add throttling in PV controller, but given the dev/release cycle, it'd be
   more convenient to add it in application, while keeping upstream logic stable and simple. Also adding throttling logic in
   application has finer granularity as we can configure throttling.

#### Status Report and Error Propagation

For now I don't think it necessary for propagating volume status back to `common.JobStatus`, as PVC will have its own
status update. If we fail to create PVC, we will f event for the targeting training job. Since we don't have distributed
training job specific metrics exporting for now, adding metrics for volume provisioning failure or so will be out of the
scope of this design.

### Risks and Mitigations

1. Even though we can throttle volume creation, there is still possibility that we hit [k8s volume binding bug](https://github.com/kubernetes/kubernetes/pull/72045) if user's
   storage class is `WaitForFirstConsumer` with k8s before 1.14.0. When this bug is hit, manual intervention is needed to
   restart scheduler to refresh its cache. But from kubeflow perspective, allowing user to specifying volume provisioning
   throttling is all what we can do

2. Fairness in throttled volume provisioning. Theoretically if we use a FIFO queue for volume creation request throttling,
   a training job with large number of replica might starve a training job that is processed after due to volume creation
   throttling. For simplicity of this feature, we can start with FIFO queue but come up with fairer throttling later
   (Non Goal #2)

## Test Plan

1. Unit tests are needed for proper transfer volume claim template into Pod's volume definitions
2. Controller integration test for testing volume creation and throttling
