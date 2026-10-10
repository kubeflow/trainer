/*
Copyright 2023 The Kubeflow Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"strings"

	apiv1 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v1"
	"github.com/kubeflow/training-operator/pkg/common"
	"github.com/kubeflow/training-operator/pkg/controller.v1/control"
	"github.com/kubeflow/training-operator/pkg/controller.v1/expectation"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
	kubeclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	schedulinglisters "k8s.io/client-go/listers/scheduling/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	volcanoclient "volcano.sh/apis/pkg/client/clientset/versioned"
)

var (
	// KeyFunc is the short name to DeletionHandlingMetaNamespaceKeyFunc.
	// IndexerInformer uses a delta queue, therefore for deletes we have to use this
	// key function but it should be just fine for non delete events.
	KeyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc

	createdPodGroupsCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "created_pod_groups_total",
		Help: "The total number of created pod groups",
	})
	deletedPodGroupsCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "deleted_pod_groups_total",
		Help: "The total number of deleted pod groups",
	})
)

type GangScheduler string

const (
	GangSchedulerNone    GangScheduler = "None"
	GangSchedulerVolcano GangScheduler = "volcano"
	// GangSchedulerSchedulerPlugins Using this scheduler name or any scheduler name different than volcano uses the scheduler-plugins PodGroup
	GangSchedulerSchedulerPlugins GangScheduler = "scheduler-plugins"
)

// JobControllerConfiguration contains configuration of operator.
type JobControllerConfiguration struct {
	// GangScheduling choice: None, volcano and scheduler-plugins
	GangScheduling GangScheduler
}

func (c *JobControllerConfiguration) EnableGangScheduling() bool {
	return c.GangScheduling != "" && c.GangScheduling != GangSchedulerNone
}

// JobController abstracts other operators to manage the lifecycle of Jobs.
// User need to first implement the ControllerInterface(objectA) and then initialize a JobController(objectB) struct with objectA
// as the parameter.
// And then call objectB.ReconcileJobs as mentioned below, the ReconcileJobs method is the entrypoint to trigger the
// reconcile logic of the job controller
//
// ReconcileJobs(
//
//	job interface{},
//	replicas map[apiv1.ReplicaType]*apiv1.ReplicaSpec,
//	jobStatus apiv1.JobStatus,
//	runPolicy *apiv1.RunPolicy) error
type JobController struct {
	// JobLabelSelector restricts reconciliation to matching jobs. Nil selects all jobs.
	// Configure before starting the controller; do not change it at runtime.
	JobLabelSelector labels.Selector

	Controller common.ControllerInterface

	Config JobControllerConfiguration

	// PodControl is used to add or delete pods.
	PodControl control.PodControlInterface

	// ServiceControl is used to add or delete services.
	ServiceControl control.ServiceControlInterface

	// KubeClientSet is a standard kubernetes clientset.
	KubeClientSet kubeclientset.Interface

	// PodGroupControl is used to add or delete PodGroup.
	PodGroupControl control.PodGroupControlInterface

	// PodLister can list/get pods from the shared informer's store.
	PodLister corelisters.PodLister

	// ServiceLister can list/get services from the shared informer's store.
	ServiceLister corelisters.ServiceLister

	// PriorityClassLister can list/get priorityClasses from the shared informer's store.
	PriorityClassLister schedulinglisters.PriorityClassLister

	// PodInformerSynced returns true if the pod store has been synced at least once.
	PodInformerSynced cache.InformerSynced

	// ServiceInformerSynced returns true if the service store has been synced at least once.
	ServiceInformerSynced cache.InformerSynced

	// PriorityClassInformerSynced returns true if the priority class store has been synced at least once.
	PriorityClassInformerSynced cache.InformerSynced

	// A TTLCache of pod/services creates/deletes each job expects to see
	// We use Job namespace/name + ReplicaType + pods/services as an expectation key,
	// For example, there is a TFJob with namespace "tf-operator" and name "tfjob-abc":
	// {
	//     "PS": {
	//         "Replicas": 2,
	//     },
	//     "Worker": {
	//         "Replicas": 4,
	//     }
	// }
	// We will create 4 expectations:
	// - "tf-operator/tfjob-abc/ps/services", expects 2 adds.
	// - "tf-operator/tfjob-abc/ps/pods", expects 2 adds.
	// - "tf-operator/tfjob-abc/worker/services", expects 4 adds.
	// - "tf-operator/tfjob-abc/worker/pods", expects 4 adds.
	Expectations expectation.ControllerExpectationsInterface

	// WorkQueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	WorkQueue workqueue.TypedRateLimitingInterface[string]

	// Recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	Recorder record.EventRecorder
}

type GangSchedulingSetupFunc func(jc *JobController)

var GenVolcanoSetupFunc = func(vci volcanoclient.Interface) GangSchedulingSetupFunc {
	return func(jc *JobController) {
		jc.Config.GangScheduling = GangSchedulerVolcano
		jc.PodGroupControl = control.NewVolcanoControl(vci)
	}
}

var GenSchedulerPluginsSetupFunc = func(c client.Client, gangSchedulerName string) GangSchedulingSetupFunc {
	return func(jc *JobController) {
		jc.Config.GangScheduling = GangScheduler(gangSchedulerName)
		jc.PodGroupControl = control.NewSchedulerPluginsControl(c, gangSchedulerName)
	}
}

var GenNonGangSchedulerSetupFunc = func() GangSchedulingSetupFunc {
	return func(jc *JobController) {
		jc.Config.GangScheduling = ""
		jc.PodGroupControl = nil
	}
}

func NewJobController(
	controllerImpl common.ControllerInterface,
	reconcilerSyncPeriod metav1.Duration,
	kubeClientSet kubeclientset.Interface,
	setupPodGroup GangSchedulingSetupFunc,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	workQueueName string) JobController {

	log.Debug("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClientSet.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerImpl.ControllerName()})

	podControl := control.RealPodControl{
		KubeClient: kubeClientSet,
		Recorder:   eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerImpl.ControllerName()}),
	}

	serviceControl := control.RealServiceControl{
		KubeClient: kubeClientSet,
		Recorder:   eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerImpl.ControllerName()}),
	}

	jc := JobController{
		Controller:     controllerImpl,
		Config:         JobControllerConfiguration{},
		PodControl:     podControl,
		ServiceControl: serviceControl,
		KubeClientSet:  kubeClientSet,
		Expectations:   expectation.NewControllerExpectations(),
		WorkQueue: workqueue.NewTypedRateLimitingQueueWithConfig(workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{Name: workQueueName}),
		Recorder: recorder,
	}

	setupPodGroup(&jc)

	return jc

}

func (jc *JobController) GenOwnerReference(obj metav1.Object) *metav1.OwnerReference {
	boolPtr := func(b bool) *bool { return &b }
	controllerRef := &metav1.OwnerReference{
		APIVersion:         jc.Controller.GetAPIGroupVersion().String(),
		Kind:               jc.Controller.GetAPIGroupVersionKind().Kind,
		Name:               obj.GetName(),
		UID:                obj.GetUID(),
		BlockOwnerDeletion: boolPtr(true),
		Controller:         boolPtr(true),
	}

	return controllerRef
}

func (jc *JobController) GenLabels(jobName string) map[string]string {
	jobName = strings.Replace(jobName, "/", "-", -1)
	return map[string]string{
		apiv1.OperatorNameLabel: jc.Controller.ControllerName(),
		apiv1.JobNameLabel:      jobName,
	}
}

// resolveControllerRef returns the job referenced by a ControllerRef,
// or nil if the ControllerRef could not be resolved to a matching job
// of the correct Kind.
func (jc *JobController) resolveControllerRef(namespace string, controllerRef *metav1.OwnerReference) metav1.Object {
	// We can't look up by UID, so look up by Name and then verify UID.
	// Don't even try to look up by Name if it's the wrong Kind.
	if controllerRef.Kind != jc.Controller.GetAPIGroupVersionKind().Kind {
		return nil
	}
	job, err := jc.Controller.GetJobFromInformerCache(namespace, controllerRef.Name)
	if err != nil {
		return nil
	}
	if job.GetUID() != controllerRef.UID {
		// The controller we found with this Name is not the same one that the
		// ControllerRef points to.
		return nil
	}
	return job
}

// MatchesJobLabels reports whether this controller manages the job labels.
func (jc *JobController) MatchesJobLabels(job metav1.Object) bool {
	return jc.JobLabelSelector == nil || jc.JobLabelSelector.Matches(labels.Set(job.GetLabels()))
}
