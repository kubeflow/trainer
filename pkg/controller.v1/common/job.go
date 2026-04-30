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
	"fmt"
	"reflect"
	"time"

	apiv1 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v1"
	"github.com/kubeflow/training-operator/pkg/controller.v1/expectation"
	"github.com/kubeflow/training-operator/pkg/core"
	commonutil "github.com/kubeflow/training-operator/pkg/util"
	"github.com/kubeflow/training-operator/pkg/util/k8sutil"
	trainutil "github.com/kubeflow/training-operator/pkg/util/train"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"
	volcanov1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"
)

// DeletePodsAndServices deletes pods and services considering cleanPodPolicy.
// However, if the job doesn't have Succeeded or Failed condition, it ignores cleanPodPolicy.
func (jc *JobController) DeletePodsAndServices(runtimeObject runtime.Object, runPolicy *apiv1.RunPolicy, jobStatus apiv1.JobStatus, pods []*corev1.Pod) error {
	if len(pods) == 0 {
		return nil
	}

	// Delete nothing when the cleanPodPolicy is None and the job has Succeeded or Failed condition.
	if commonutil.IsFinished(jobStatus) && *runPolicy.CleanPodPolicy == apiv1.CleanPodPolicyNone {
		return nil
	}

	for _, pod := range pods {
		// Note that pending pod will turn into running once schedulable,
		// not cleaning it may leave orphan running pod in the future,
		// we should treat it equivalent to running phase here.
		if commonutil.IsFinished(jobStatus) && *runPolicy.CleanPodPolicy == apiv1.CleanPodPolicyRunning && pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodPending {
			continue
		}
		if err := jc.PodControl.DeletePod(pod.Namespace, pod.Name, runtimeObject); err != nil {
			return err
		}
		// Pod and service have the same name, thus the service could be deleted using pod's name.
		if err := jc.ServiceControl.DeleteService(pod.Namespace, pod.Name, runtimeObject); err != nil {
			return err
		}
	}
	return nil
}

// recordAbnormalPods records the active pod whose latest condition is not in True status.
func (jc *JobController) recordAbnormalPods(activePods []*corev1.Pod, object runtime.Object) {
	core.RecordAbnormalPods(activePods, object, jc.Recorder)
}

// ReconcileJobs checks and updates replicas for each given ReplicaSpec.
// It will requeue the job in case of an error while creating/deleting pods/services.
func (jc *JobController) ReconcileJobs(
	job interface{},
	replicas map[apiv1.ReplicaType]*apiv1.ReplicaSpec,
	jobStatus apiv1.JobStatus,
	runPolicy *apiv1.RunPolicy) error {

	metaObject, ok := job.(metav1.Object)
	jobName := metaObject.GetName()
	if !ok {
		return fmt.Errorf("job is not of type metav1.Object")
	}
	runtimeObject, ok := job.(runtime.Object)
	if !ok {
		return fmt.Errorf("job is not of type runtime.Object")
	}
	jobKey, err := KeyFunc(job)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for job object %#v: %v", job, err))
		return err
	}
	jobKind := jc.Controller.GetAPIGroupVersionKind().Kind
	// Reset expectations
	// 1. Since `ReconcileJobs` is called, we expect that previous expectations are all satisfied,
	//    and it's safe to reset the expectations
	// 2. Reset expectations can avoid dirty data such as `expectedDeletion = -1`
	//    (pod or service was deleted unexpectedly)
	if err = jc.ResetExpectations(jobKey, replicas); err != nil {
		log.Warnf("Failed to reset expectations: %v", err)
	}

	log.Infof("Reconciling for job %s", metaObject.GetName())
	pods, err := jc.Controller.GetPodsForJob(job)
	if err != nil {
		log.Warnf("GetPodsForJob error %v", err)
		return err
	}

	services, err := jc.Controller.GetServicesForJob(job)
	if err != nil {
		log.Warnf("GetServicesForJob error %v", err)
		return err
	}

	oldStatus := jobStatus.DeepCopy()
	if commonutil.IsFinished(jobStatus) {
		// If the Job is succeeded or failed, delete all pods, services, and podGroup.
		if err = jc.CleanUpResources(runPolicy, runtimeObject, metaObject, jobStatus, pods); err != nil {
			return err
		}

		// At this point the pods may have been deleted.
		// 1) If the job succeeded, we manually set the replica status.
		// 2) If any replicas are still active, set their status to 'succeeded'.
		if commonutil.IsSucceeded(jobStatus) {
			for rtype := range jobStatus.ReplicaStatuses {
				jobStatus.ReplicaStatuses[rtype].Succeeded += jobStatus.ReplicaStatuses[rtype].Active
				jobStatus.ReplicaStatuses[rtype].Active = 0
			}
		}

		// No need to update the job status if the status hasn't changed since last time.
		if !reflect.DeepEqual(*oldStatus, jobStatus) {
			return jc.Controller.UpdateJobStatusInApiServer(job, &jobStatus)
		}

		return nil
	}

	if trainutil.IsJobSuspended(runPolicy) {
		if err = jc.CleanUpResources(runPolicy, runtimeObject, metaObject, jobStatus, pods); err != nil {
			return err
		}
		for rType := range jobStatus.ReplicaStatuses {
			jobStatus.ReplicaStatuses[rType].Active = 0
		}
		msg := fmt.Sprintf("%s %s is suspended.", jobKind, jobName)
		if commonutil.IsRunning(jobStatus) {
			commonutil.UpdateJobConditions(&jobStatus, apiv1.JobRunning, corev1.ConditionFalse, commonutil.NewReason(jobKind, commonutil.JobSuspendedReason), msg)
		}
		// We add the suspended condition to the job only when the job doesn't have a suspended condition.
		if !commonutil.IsSuspended(jobStatus) {
			commonutil.UpdateJobConditions(&jobStatus, apiv1.JobSuspended, corev1.ConditionTrue, commonutil.NewReason(jobKind, commonutil.JobSuspendedReason), msg)
		}
		jc.Recorder.Event(runtimeObject, corev1.EventTypeNormal, commonutil.NewReason(jobKind, commonutil.JobSuspendedReason), msg)
		if !reflect.DeepEqual(*oldStatus, jobStatus) {
			return jc.Controller.UpdateJobStatusInApiServer(job, &jobStatus)
		}
		return nil
	}
	if commonutil.IsSuspended(jobStatus) {
		msg := fmt.Sprintf("%s %s is resumed.", jobKind, jobName)
		commonutil.UpdateJobConditions(&jobStatus, apiv1.JobSuspended, corev1.ConditionFalse, commonutil.NewReason(jobKind, commonutil.JobResumedReason), msg)
		now := metav1.Now()
		jobStatus.StartTime = &now
		jc.Recorder.Eventf(runtimeObject, corev1.EventTypeNormal, commonutil.NewReason(jobKind, commonutil.JobResumedReason), msg)
	}

	// retrieve the previous number of retry
	previousRetry := jc.WorkQueue.NumRequeues(jobKey)

	activePods := k8sutil.FilterActivePods(pods)

	jc.recordAbnormalPods(activePods, runtimeObject)

	active := int32(len(activePods))
	failed := k8sutil.FilterPodCount(pods, corev1.PodFailed)
	totalReplicas := k8sutil.GetTotalReplicas(replicas)
	prevReplicasFailedNum := k8sutil.GetTotalFailedReplicas(jobStatus.ReplicaStatuses)

	var failureMessage string
	jobExceedsLimit := false
	exceedsBackoffLimit := false
	pastBackoffLimit := false

	if runPolicy.BackoffLimit != nil {
		jobHasNewFailure := failed > prevReplicasFailedNum
		// new failures happen when status does not reflect the failures and active
		// is different from parallelism, otherwise the previous controller loop
		// failed updating status so even if we pick up failure it is not a new one
		exceedsBackoffLimit = jobHasNewFailure && (active != totalReplicas) &&
			(int32(previousRetry)+1 > *runPolicy.BackoffLimit)

		pastBackoffLimit, err = jc.PastBackoffLimit(jobName, runPolicy, replicas, pods)
		if err != nil {
			return err
		}
	}

	if exceedsBackoffLimit || pastBackoffLimit {
		// check if the number of pod restart exceeds backoff (for restart OnFailure only)
		// OR if the number of failed jobs increased since the last syncJob
		jobExceedsLimit = true
		failureMessage = fmt.Sprintf("Job %s has failed because it has reached the specified backoff limit", jobName)
	} else if jc.PastActiveDeadline(runPolicy, jobStatus) {
		failureMessage = fmt.Sprintf("Job %s has failed because it was active longer than specified deadline", jobName)
		jobExceedsLimit = true
	}

	if jobExceedsLimit {
		// Set job completion time before resource cleanup
		if jobStatus.CompletionTime == nil {
			now := metav1.Now()
			jobStatus.CompletionTime = &now
		}

		// Set JobFailed condition BEFORE cleanup so that DeletePodsAndServices
		// sees IsFinished()==true and correctly respects cleanPodPolicy.
		// Fix: https://github.com/kubeflow/trainer/issues/3419
		jc.Recorder.Event(runtimeObject, corev1.EventTypeNormal, commonutil.NewReason(jobKind, commonutil.JobFailedReason), failureMessage)
		commonutil.UpdateJobConditions(&jobStatus, apiv1.JobFailed, corev1.ConditionTrue, commonutil.NewReason(jobKind, commonutil.JobFailedReason), failureMessage)

		if err := jc.DeletePodsAndServices(runtimeObject, runPolicy, jobStatus, pods); err != nil {
			return err
		}

		if err := jc.CleanupJob(runPolicy, jobStatus, job); err != nil {
			return err
		}

		if jc.Config.EnableGangScheduling() {
			jc.Recorder.Event(runtimeObject, corev1.EventTypeNormal, "JobTerminated", "Job has been terminated. Deleting PodGroup")
			if err := jc.DeletePodGroup(metaObject); err != nil {
				jc.Recorder.Eventf(runtimeObject, corev1.EventTypeWarning, "FailedDeletePodGroup", "Error deleting: %v", err)
				return err
			} else {
				jc.Recorder.Eventf(runtimeObject, corev1.EventTypeNormal, "SuccessfulDeletePodGroup", "Deleted PodGroup: %v", jobName)
			}
		}

		return jc.Controller.UpdateJobStatusInApiServer(job, &jobStatus)
	} else {
		// General cases which need to reconcile
		if jc.Config.EnableGangScheduling() {
			minMember := totalReplicas
			queue := "default"
			priorityClass := ""
			var schedulerTimeout *int32
			var minResources *corev1.ResourceList

			if runPolicy.SchedulingPolicy != nil {
				if minAvailable := runPolicy.SchedulingPolicy.MinAvailable; minAvailable != nil {
					minMember = *minAvailable
				}
				if q := runPolicy.SchedulingPolicy.Queue; len(q) != 0 {
					queue = q
				}
				if pc := runPolicy.SchedulingPolicy.PriorityClass; len(pc) != 0 {
					priorityClass = pc
				}
				if mr := runPolicy.SchedulingPolicy.MinResources; mr != nil {
					minResources = (*corev1.ResourceList)(mr)
				}
				if timeout := runPolicy.SchedulingPolicy.ScheduleTimeoutSeconds; timeout != nil {
					schedulerTimeout = timeout
				}
			}

			if minResources == nil {
				minResources = jc.calcPGMinResources(minMember, replicas)
			}

			var pgSpecFill FillPodGroupSpecFunc
			switch jc.Config.GangScheduling {
			case GangSchedulerVolcano:
				pgSpecFill = func(pg metav1.Object) error {
					volcanoPodGroup, match := pg.(*volcanov1beta1.PodGroup)
					if !match {
						return fmt.Errorf("unable to recognize PodGroup: %v", klog.KObj(pg))
					}

					if q := volcanoPodGroup.Spec.Queue; len(q) > 0 {
						queue = q
					}

					volcanoPodGroup.Spec = volcanov1beta1.PodGroupSpec{
						MinMember:         minMember,
						Queue:             queue,
						PriorityClassName: priorityClass,
						MinResources:      minResources,
					}
					return nil
				}
			default:
				pgSpecFill = func(pg metav1.Object) error {
					schedulerPluginsPodGroup, match := pg.(*schedulerpluginsv1alpha1.PodGroup)
					if !match {
						return fmt.Errorf("unable to recognize PodGroup: %v", klog.KObj(pg))
					}
					schedulerPluginsPodGroup.Spec = schedulerpluginsv1alpha1.PodGroupSpec{
						MinMember:              minMember,
						MinResources:           *minResources,
						ScheduleTimeoutSeconds: schedulerTimeout,
					}
					return nil
				}
			}

			syncReplicas := true
			pg, err := jc.SyncPodGroup(metaObject, pgSpecFill)
			if err != nil {
				log.Warnf("Sync PodGroup %v: %v", jobKey, err)
				syncReplicas = false
			}

			// Delay pods creation until PodGroup status is Inqueue
			if jc.PodGroupControl.DelayPodCreationDueToPodGroup(pg) {
				log.Warnf("PodGroup %v unschedulable", jobKey)
				syncReplicas = false
			}

			if !syncReplicas {
				now := metav1.Now()
				jobStatus.LastReconcileTime = &now

				// Update job status here to trigger a new reconciliation
				return jc.Controller.UpdateJobStatusInApiServer(job, &jobStatus)
			}
		}

		// Diff current active pods/services with replicas.
		for rtype, spec := range replicas {
			err := jc.Controller.ReconcilePods(metaObject, &jobStatus, pods, rtype, spec, replicas)
			if err != nil {
				log.Warnf("ReconcilePods error %v", err)
				return err
			}

			err = jc.Controller.ReconcileServices(metaObject, services, rtype, spec)

			if err != nil {
				log.Warnf("ReconcileServices error %v", err)
				return err
			}
		}
	}

	err = jc.Controller.UpdateJobStatus(job, replicas, &jobStatus)
	if err != nil {
		log.Warnf("UpdateJobStatus error %v", err)
		return err
	}
	// No need to update the job status if the status hasn't changed since last time.
	if !reflect.DeepEqual(*oldStatus, jobStatus) {
		return jc.Controller.UpdateJobStatusInApiServer(job, &jobStatus)
	}
	return nil
}

func (jc *JobController) CleanUpResources(
	runPolicy *apiv1.RunPolicy,
	runtimeObject runtime.Object,
	metaObject metav1.Object,
	jobStatus apiv1.JobStatus,
	pods []*corev1.Pod,
) error {
	if err := jc.DeletePodsAndServices(runtimeObject, runPolicy, jobStatus, pods); err != nil {
		return err
	}
	if jc.Config.EnableGangScheduling() {

		jc.Recorder.Event(runtimeObject, corev1.EventTypeNormal, "JobTerminated", "Job has been terminated. Deleting PodGroup")
		if err := jc.DeletePodGroup(metaObject); err != nil {
			jc.Recorder.Eventf(runtimeObject, corev1.EventTypeWarning, "FailedDeletePodGroup", "Error deleting: %v", err)
			return err
		} else {
			jc.Recorder.Eventf(runtimeObject, corev1.EventTypeNormal, "SuccessfulDeletePodGroup", "Deleted PodGroup: %v", metaObject.GetName())
		}
	}
	if err := jc.CleanupJob(runPolicy, jobStatus, runtimeObject); err != nil {
		return err
	}
	return nil
}

// ResetExpectations reset the expectation for creates and deletes of pod/service to zero.
func (jc *JobController) ResetExpectations(jobKey string, replicas map[apiv1.ReplicaType]*apiv1.ReplicaSpec) error {
	var allErrs error
	for rtype := range replicas {
		expectationPodsKey := expectation.GenExpectationPodsKey(jobKey, string(rtype))
		if err := jc.Expectations.SetExpectations(expectationPodsKey, 0, 0); err != nil {
			allErrs = err
		}
		expectationServicesKey := expectation.GenExpectationServicesKey(jobKey, string(rtype))
		if err := jc.Expectations.SetExpectations(expectationServicesKey, 0, 0); err != nil {
			allErrs = fmt.Errorf("%s: %w", allErrs.Error(), err)
		}
	}
	return allErrs
}

// PastActiveDeadline checks if job has ActiveDeadlineSeconds field set and if it is exceeded.
func (jc *JobController) PastActiveDeadline(runPolicy *apiv1.RunPolicy, jobStatus apiv1.JobStatus) bool {
	return core.PastActiveDeadline(runPolicy, jobStatus)
}

// PastBackoffLimit checks if container restartCounts sum exceeds BackoffLimit
// this method applies only to pods when restartPolicy is one of OnFailure, Always or ExitCode
func (jc *JobController) PastBackoffLimit(jobName string, runPolicy *apiv1.RunPolicy,
	replicas map[apiv1.ReplicaType]*apiv1.ReplicaSpec, pods []*corev1.Pod) (bool, error) {
	return core.PastBackoffLimit(jobName, runPolicy, replicas, pods, jc.FilterPodsForReplicaType)
}

func (jc *JobController) CleanupJob(runPolicy *apiv1.RunPolicy, jobStatus apiv1.JobStatus, job interface{}) error {
	currentTime := time.Now()
	metaObject, _ := job.(metav1.Object)
	ttl := runPolicy.TTLSecondsAfterFinished
	if ttl == nil || trainutil.IsJobSuspended(runPolicy) {
		return nil
	}
	duration := time.Second * time.Duration(*ttl)
	if jobStatus.CompletionTime == nil {
		return fmt.Errorf("job completion time is nil, cannot cleanup")
	}
	finishTime := jobStatus.CompletionTime
	expireTime := finishTime.Add(duration)
	if currentTime.After(expireTime) {
		err := jc.Controller.DeleteJob(job)
		if err != nil {
			commonutil.LoggerForJob(metaObject).Warnf("Cleanup Job error: %v.", err)
			return err
		}
		return nil
	} else {
		if finishTime.After(currentTime) {
			commonutil.LoggerForJob(metaObject).Warnf("Found Job finished in the future. This is likely due to time skew in the cluster. Job cleanup will be deferred.")
		}
		remaining := expireTime.Sub(currentTime)
		key, err := KeyFunc(job)
		if err != nil {
			commonutil.LoggerForJob(metaObject).Warnf("Couldn't get key for job object: %v", err)
			return err
		}
		jc.WorkQueue.AddAfter(key, remaining)
		return nil
	}
}

func (jc *JobController) calcPGMinResources(minMember int32, replicas map[apiv1.ReplicaType]*apiv1.ReplicaSpec) *corev1.ResourceList {
	return CalcPGMinResources(minMember, replicas, jc.PriorityClassLister.Get)
}

func (jc *JobController) ManagedByExternalController(controllerName *string) *string {
	if controllerName != nil && *controllerName != apiv1.KubeflowJobsController {
		return controllerName
	}
	return nil
}
