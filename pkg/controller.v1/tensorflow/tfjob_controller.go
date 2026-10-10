// Copyright 2021 The Kubeflow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tensorflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	kubeflowv1 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v1"
	trainingoperatorcommon "github.com/kubeflow/training-operator/pkg/common"
	"github.com/kubeflow/training-operator/pkg/common/util"
	"github.com/kubeflow/training-operator/pkg/controller.v1/common"
	"github.com/kubeflow/training-operator/pkg/controller.v1/control"
	"github.com/kubeflow/training-operator/pkg/controller.v1/expectation"
	commonutil "github.com/kubeflow/training-operator/pkg/util"

	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	kubeclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"
	"volcano.sh/apis/pkg/apis/scheduling/v1beta1"
)

const (
	FailedDeleteJobReason     = "FailedDeleteJob"
	SuccessfulDeleteJobReason = "SuccessfulDeleteJob"

	controllerName = "tfjob-controller"

	// tfConfig is the environment variable name of TensorFlow cluster spec.
	tfConfig = "TF_CONFIG"
)

func NewReconciler(mgr manager.Manager, gangSchedulingSetupFunc common.GangSchedulingSetupFunc) *TFJobReconciler {
	r := &TFJobReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		recorder:  mgr.GetEventRecorderFor(controllerName),
		apiReader: mgr.GetAPIReader(),
		Log:       log.Log,
	}

	cfg := mgr.GetConfig()
	kubeClientSet := kubeclientset.NewForConfigOrDie(cfg)
	sharedInformers := informers.NewSharedInformerFactory(kubeClientSet, 0)
	priorityClassInformer := sharedInformers.Scheduling().V1().PriorityClasses()

	r.JobController = common.JobController{
		Controller:                  r,
		Expectations:                expectation.NewControllerExpectations(),
		WorkQueue:                   &util.FakeWorkQueue[string]{},
		Recorder:                    r.recorder,
		KubeClientSet:               kubeClientSet,
		PriorityClassLister:         priorityClassInformer.Lister(),
		PriorityClassInformerSynced: priorityClassInformer.Informer().HasSynced,
		PodControl:                  control.RealPodControl{KubeClient: kubeClientSet, Recorder: r.recorder},
		ServiceControl:              control.RealServiceControl{KubeClient: kubeClientSet, Recorder: r.recorder},
	}

	gangSchedulingSetupFunc(&r.JobController)

	return r
}

// TFJobReconciler reconciles a TFJob object
type TFJobReconciler struct {
	common.JobController
	client.Client
	Scheme    *runtime.Scheme
	recorder  record.EventRecorder
	apiReader client.Reader
	Log       logr.Logger
}

// +kubebuilder:rbac:groups=kubeflow.org,resources=tfjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubeflow.org,resources=tfjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubeflow.org,resources=tfjobs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=scheduling.volcano.sh,resources=podgroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=scheduling.x-k8s.io,resources=podgroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *TFJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	logger := r.Log.WithValues(kubeflowv1.TFJobSingular, req.NamespacedName)

	tfjob := &kubeflowv1.TFJob{}
	err := r.Get(ctx, req.NamespacedName, tfjob)
	if err != nil {
		logger.Info(err.Error(), "unable to fetch TFJob", req.NamespacedName.String())
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !r.MatchesJobLabels(tfjob) {
		return ctrl.Result{}, nil
	}

	if manager := r.ManagedByExternalController(tfjob.Spec.RunPolicy.ManagedBy); manager != nil {
		logger.Info("Skipping TFJob managed by a custom controller", "managed-by", manager)
		return ctrl.Result{}, nil
	}

	// Check if reconciliation is needed
	jobKey, err := common.KeyFunc(tfjob)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get jobKey for job object %#v: %v", tfjob, err))
	}

	replicaTypes := util.GetReplicaTypes(tfjob.Spec.TFReplicaSpecs)
	needReconcile := util.SatisfiedExpectations(r.Expectations, jobKey, replicaTypes)

	if !needReconcile || tfjob.GetDeletionTimestamp() != nil {
		logger.Info("reconcile cancelled, job does not need to do reconcile or has been deleted",
			"sync", needReconcile, "deleted", tfjob.GetDeletionTimestamp() != nil)
		return ctrl.Result{}, nil
	}

	// Set default priorities to tfjob
	r.Scheme.Default(tfjob)

	// Use common to reconcile the job related pod and service
	err = r.ReconcileJobs(tfjob, tfjob.Spec.TFReplicaSpecs, tfjob.Status, &tfjob.Spec.RunPolicy)
	if err != nil {
		logrus.Warnf("Reconcile Tensorflow Job error %v", err)
		return ctrl.Result{}, err
	}

	t, err := util.DurationUntilExpireTime(&tfjob.Spec.RunPolicy, tfjob.Status)
	if err != nil {
		logrus.Warnf("Reconcile Tensorflow Job error %v", err)
		return ctrl.Result{}, err
	}
	if t >= 0 {
		return ctrl.Result{Requeue: true, RequeueAfter: t}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TFJobReconciler) SetupWithManager(mgr ctrl.Manager, controllerThreads int) error {
	c, err := controller.New(r.ControllerName(), mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: controllerThreads,
	})
	if err != nil {
		return err
	}
	// using onOwnerCreateFunc is easier to set defaults
	if err = c.Watch(source.Kind[*kubeflowv1.TFJob](mgr.GetCache(), &kubeflowv1.TFJob{},
		&handler.TypedEnqueueRequestForObject[*kubeflowv1.TFJob]{},
		predicate.TypedFuncs[*kubeflowv1.TFJob]{CreateFunc: r.onOwnerCreateFunc()}),
	); err != nil {
		return err
	}
	// inject watching for job related pod
	if err = c.Watch(source.Kind[*corev1.Pod](mgr.GetCache(), &corev1.Pod{},
		handler.TypedEnqueueRequestForOwner[*corev1.Pod](mgr.GetScheme(), mgr.GetRESTMapper(), &kubeflowv1.TFJob{}, handler.OnlyControllerOwner()),
		util.OnDependentFuncs[*corev1.Pod](r.Scheme, r.Expectations, &r.JobController))); err != nil {
		return err
	}
	// inject watching for job related service
	if err = c.Watch(source.Kind[*corev1.Service](mgr.GetCache(), &corev1.Service{},
		handler.TypedEnqueueRequestForOwner[*corev1.Service](mgr.GetScheme(), mgr.GetRESTMapper(), &kubeflowv1.TFJob{}, handler.OnlyControllerOwner()),
		util.OnDependentFuncs[*corev1.Service](r.Scheme, r.Expectations, &r.JobController))); err != nil {
		return err
	}
	// skip watching volcano PodGroup if volcano PodGroup is not installed
	if _, err = mgr.GetRESTMapper().RESTMapping(schema.GroupKind{Group: v1beta1.GroupName, Kind: "PodGroup"},
		v1beta1.SchemeGroupVersion.Version); err == nil {
		// inject watching for job related volcano PodGroup
		if err = c.Watch(source.Kind[*v1beta1.PodGroup](mgr.GetCache(), &v1beta1.PodGroup{},
			handler.TypedEnqueueRequestForOwner[*v1beta1.PodGroup](mgr.GetScheme(), mgr.GetRESTMapper(), &kubeflowv1.TFJob{}, handler.OnlyControllerOwner()),
			util.OnDependentFuncs[*v1beta1.PodGroup](r.Scheme, r.Expectations, &r.JobController))); err != nil {
			return err
		}
	}
	// skip watching scheduler-plugins PodGroup if scheduler-plugins PodGroup is not installed
	if _, err = mgr.GetRESTMapper().RESTMapping(schema.GroupKind{Group: schedulerpluginsv1alpha1.SchemeGroupVersion.Group, Kind: "PodGroup"},
		schedulerpluginsv1alpha1.SchemeGroupVersion.Version); err == nil {
		// inject watching for job related scheduler-plugins PodGroup
		if err = c.Watch(source.Kind[*schedulerpluginsv1alpha1.PodGroup](mgr.GetCache(), &schedulerpluginsv1alpha1.PodGroup{},
			handler.TypedEnqueueRequestForOwner[*schedulerpluginsv1alpha1.PodGroup](mgr.GetScheme(), mgr.GetRESTMapper(), &kubeflowv1.TFJob{}, handler.OnlyControllerOwner()),
			util.OnDependentFuncs[*schedulerpluginsv1alpha1.PodGroup](r.Scheme, r.Expectations, &r.JobController))); err != nil {
			return err
		}
	}
	return nil
}

func (r *TFJobReconciler) ControllerName() string {
	return controllerName
}

func (r *TFJobReconciler) GetAPIGroupVersionKind() schema.GroupVersionKind {
	return kubeflowv1.GroupVersion.WithKind(kubeflowv1.TFJobKind)
}

func (r *TFJobReconciler) GetAPIGroupVersion() schema.GroupVersion {
	return kubeflowv1.GroupVersion
}

func (r *TFJobReconciler) GetGroupNameLabelValue() string {
	return kubeflowv1.GroupVersion.Group
}

func (r *TFJobReconciler) GetFrameworkName() string {
	return kubeflowv1.TFJobFrameworkName
}

func (r *TFJobReconciler) GetJobFromInformerCache(namespace, name string) (metav1.Object, error) {
	tfjob := &kubeflowv1.TFJob{}
	err := r.Get(context.Background(), types.NamespacedName{
		Namespace: namespace, Name: name,
	}, tfjob)
	return tfjob, err
}

func (r *TFJobReconciler) GetJobFromAPIClient(namespace, name string) (metav1.Object, error) {
	job := &kubeflowv1.TFJob{}

	err := r.apiReader.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, job)
	if err != nil {
		if errors.IsNotFound(err) {
			logrus.Error(err, "tensorflow job not found", "namespace", namespace, "name", name)
		} else {
			logrus.Error(err, "failed to get job from api-server", "namespace", namespace, "name", name)
		}
		return nil, err
	}
	return job, nil
}

// GetPodsForJob returns the set of pods that this job should manage.
// It also reconciles ControllerRef by adopting/orphaning.
// Note that the returned Pods are pointers into the cache.
func (r *TFJobReconciler) GetPodsForJob(jobObject interface{}) ([]*corev1.Pod, error) {
	job, ok := jobObject.(metav1.Object)
	if !ok {
		return nil, fmt.Errorf("job is not of type metav1.Object")
	}

	// Create selector.
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: r.GenLabels(job.GetName()),
	})

	if err != nil {
		return nil, fmt.Errorf("couldn't convert Job selector: %v", err)
	}
	// List all pods to include those that don't match the selector anymore
	// but have a ControllerRef pointing to this controller.
	podlist := &corev1.PodList{}
	err = r.List(context.Background(), podlist,
		client.MatchingLabelsSelector{Selector: selector}, client.InNamespace(job.GetNamespace()))
	if err != nil {
		return nil, err
	}

	pods := util.JobControlledPodList(podlist.Items, job)

	// If any adoptions are attempted, we should first recheck for deletion
	// with an uncached quorum read sometime after listing Pods (see #42639).
	canAdoptFunc := common.RecheckDeletionTimestamp(func() (metav1.Object, error) {
		fresh, err := r.Controller.GetJobFromAPIClient(job.GetNamespace(), job.GetName())
		if err != nil {
			return nil, err
		}
		if fresh.GetUID() != job.GetUID() {
			return nil, fmt.Errorf("original Job %v/%v is gone: got uid %v, wanted %v", job.GetNamespace(), job.GetName(), fresh.GetUID(), job.GetUID())
		}
		return fresh, nil
	})
	cm := control.NewPodControllerRefManager(r.PodControl, job, selector, r.Controller.GetAPIGroupVersionKind(), canAdoptFunc)
	return cm.ClaimPods(pods)
}

// GetServicesForJob returns the set of services that this job should manage.
// It also reconciles ControllerRef by adopting/orphaning.
// Note that the returned services are pointers into the cache.
func (r *TFJobReconciler) GetServicesForJob(jobObject interface{}) ([]*corev1.Service, error) {
	job, ok := jobObject.(metav1.Object)
	if !ok {
		return nil, fmt.Errorf("job is not of type metav1.Object")
	}

	// Create selector
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: r.GenLabels(job.GetName()),
	})

	if err != nil {
		return nil, fmt.Errorf("couldn't convert Job selector: %v", err)
	}
	// List all services to include those that don't match the selector anymore
	// but have a ControllerRef pointing to this controller.
	svclist := &corev1.ServiceList{}
	err = r.List(context.Background(), svclist,
		client.MatchingLabelsSelector{Selector: selector}, client.InNamespace(job.GetNamespace()))
	if err != nil {
		return nil, fmt.Errorf("couldn't get Service: %v", err)
	}

	// If any adoptions are attempted, we should first recheck for deletion
	// with an uncached quorum read sometime after listing services (see #42639).
	canAdoptFunc := common.RecheckDeletionTimestamp(func() (metav1.Object, error) {
		fresh, err := r.GetJobFromInformerCache(job.GetNamespace(), job.GetName())
		if err != nil {
			return nil, err
		}
		if fresh.GetUID() != job.GetUID() {
			return nil, fmt.Errorf("original Job %v/%v is gone: got uid %v, wanted %v", job.GetNamespace(), job.GetName(), fresh.GetUID(), job.GetUID())
		}
		return fresh, nil
	})
	cm := control.NewServiceControllerRefManager(r.ServiceControl, job, selector, r.Controller.GetAPIGroupVersionKind(), canAdoptFunc)

	services := util.ConvertServiceList(svclist.Items)
	return cm.ClaimServices(services)
}

func (r *TFJobReconciler) DeleteJob(job interface{}) error {
	tfJob, ok := job.(*kubeflowv1.TFJob)
	if !ok {
		return fmt.Errorf("%v is not a type of TFJob", tfJob)
	}

	log := commonutil.LoggerForJob(tfJob)
	if err := r.Delete(context.Background(), tfJob); err != nil {
		r.recorder.Eventf(tfJob, v1.EventTypeWarning, FailedDeleteJobReason, "Error deleting: %v", err)
		log.Errorf("failed to delete job %s/%s, %v", tfJob.Namespace, tfJob.Name, err)
		return err
	}

	r.recorder.Eventf(tfJob, v1.EventTypeNormal, SuccessfulDeleteJobReason, "Deleted job: %v", tfJob.Name)
	log.Infof("job %s/%s has been deleted", tfJob.Namespace, tfJob.Name)
	trainingoperatorcommon.DeletedJobsCounterInc(tfJob.Namespace, r.GetFrameworkName())
	return nil
}

func (r *TFJobReconciler) UpdateJobStatus(job interface{}, replicas map[kubeflowv1.ReplicaType]*kubeflowv1.ReplicaSpec, jobStatus *kubeflowv1.JobStatus) error {
	tfJob, ok := job.(*kubeflowv1.TFJob)
	if !ok {
		return fmt.Errorf("%v is not a type of TFJob", tfJob)
	}

	tfJobKey, err := common.KeyFunc(tfJob)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for tfjob object %#v: %v", tfJob, err))
		return err
	}

	logger := commonutil.LoggerForJob(tfJob)

	worker0Completed, err := r.IsWorker0Completed(tfJob, replicas)
	if err != nil {
		logger.Warnf("check if worker 0 completed error %v", err)
		return err
	}

	// Set StartTime.
	if jobStatus.StartTime == nil {
		now := metav1.Now()
		jobStatus.StartTime = &now
		// enqueue a sync to check if job past ActiveDeadlineSeconds
		if tfJob.Spec.RunPolicy.ActiveDeadlineSeconds != nil {
			logger.Infof("Job with ActiveDeadlineSeconds will sync after %d seconds", *tfJob.Spec.RunPolicy.ActiveDeadlineSeconds)
			// TODO(Jeffwan): requeue job key in reconciler scenarios
			r.WorkQueue.AddAfter(tfJobKey, time.Duration(*tfJob.Spec.RunPolicy.ActiveDeadlineSeconds)*time.Second)
		}
	}

	// For the situation that jobStatus has a restarting condition, and append a running condition,
	// the restarting condition will be removed from jobStatus by kubeflowv1.filterOutCondition(),
	// so we need to record the existing restarting condition for later use.
	var existingRestartingCondition *kubeflowv1.JobCondition
	for _, condition := range jobStatus.Conditions {
		if condition.Type == kubeflowv1.JobRestarting {
			existingRestartingCondition = &kubeflowv1.JobCondition{
				Reason:  condition.Reason,
				Message: condition.Message,
			}
		}
	}

	// iterate the replica spec based on this order
	allTypes := []kubeflowv1.ReplicaType{
		kubeflowv1.TFJobReplicaTypeChief,
		kubeflowv1.TFJobReplicaTypeEval,
		kubeflowv1.TFJobReplicaTypeMaster,
		kubeflowv1.TFJobReplicaTypePS,
		kubeflowv1.TFJobReplicaTypeWorker,
	}
	for _, rtype := range allTypes {
		if replicas[rtype] == nil {
			continue
		}
		spec := replicas[rtype]
		status := jobStatus.ReplicaStatuses[rtype]

		// Expect to have `replicas - succeeded` pods alive.
		succeeded := status.Succeeded
		expected := *(spec.Replicas) - succeeded
		running := status.Active
		failed := status.Failed

		logger.Infof("TFJob=%s/%s, ReplicaType=%s expected=%d, running=%d, failed=%d",
			tfJob.Namespace, tfJob.Name, rtype, expected, running, failed)

		// If the TFJob contains Chief or Master spec, then we will update the status
		// according to the Chief/Master spec.
		if ContainsChiefOrMasterSpec(tfJob.Spec.TFReplicaSpecs) {
			if kubeflowv1.IsChiefOrMaster(rtype) {
				if running > 0 {
					msg := fmt.Sprintf("TFJob %s/%s is running.", tfJob.Namespace, tfJob.Name)
					commonutil.UpdateJobConditions(jobStatus, kubeflowv1.JobRunning, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.TFJobKind, commonutil.JobRunningReason), msg)
				}
				if expected == 0 {
					msg := fmt.Sprintf("TFJob %s/%s successfully completed.",
						tfJob.Namespace, tfJob.Name)
					r.recorder.Event(tfJob, corev1.EventTypeNormal, commonutil.NewReason(kubeflowv1.TFJobKind, commonutil.JobSucceededReason), msg)
					if jobStatus.CompletionTime == nil {
						now := metav1.Now()
						jobStatus.CompletionTime = &now
					}
					commonutil.UpdateJobConditions(jobStatus, kubeflowv1.JobSucceeded, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.TFJobKind, commonutil.JobSucceededReason), msg)
					trainingoperatorcommon.SuccessfulJobsCounterInc(tfJob.Namespace, r.GetFrameworkName())
				}
			}
		} else {
			if rtype == kubeflowv1.TFJobReplicaTypeWorker {
				// Leave a succeeded condition for the following two cases:
				// 1. If default success policy is used and worker 0 has completed.
				// 2. If `SuccessPolicyAllWorkers` success policy is used and all workers are succeeded.
				if expected == 0 || (worker0Completed && *tfJob.Spec.SuccessPolicy != kubeflowv1.SuccessPolicyAllWorkers) {
					msg := fmt.Sprintf("TFJob %s/%s successfully completed.",
						tfJob.Namespace, tfJob.Name)
					r.recorder.Event(tfJob, corev1.EventTypeNormal, commonutil.NewReason(kubeflowv1.TFJobKind, commonutil.JobSucceededReason), msg)
					if jobStatus.CompletionTime == nil {
						now := metav1.Now()
						jobStatus.CompletionTime = &now
					}
					commonutil.UpdateJobConditions(jobStatus, kubeflowv1.JobSucceeded, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.TFJobKind, commonutil.JobSucceededReason), msg)
					trainingoperatorcommon.SuccessfulJobsCounterInc(tfJob.Namespace, r.GetFrameworkName())
				} else if running > 0 {
					// Some workers are still running, leave a running condition.
					msg := fmt.Sprintf("TFJob %s/%s is running.", tfJob.Namespace, tfJob.Name)
					commonutil.UpdateJobConditions(jobStatus, kubeflowv1.JobRunning, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.TFJobKind, commonutil.JobRunningReason), msg)
				}
			}
		}

		if failed > 0 {
			// For the situation that jobStatus has a restarting condition, and appends a new running condition,
			// the restarting condition will be removed from jobStatus by kubeflowv1.filterOutCondition(),
			// so we need to append the restarting condition back to jobStatus.
			if existingRestartingCondition != nil {
				commonutil.UpdateJobConditions(jobStatus, kubeflowv1.JobRestarting, corev1.ConditionTrue, existingRestartingCondition.Reason, existingRestartingCondition.Message)
				// job is restarting, no need to set it failed
				// we know it because we update the status condition when reconciling the replicas
				trainingoperatorcommon.RestartedJobsCounterInc(tfJob.Namespace, r.GetFrameworkName())
			} else {
				if tfJob.Spec.EnableDynamicWorker && rtype == kubeflowv1.TFJobReplicaTypeWorker {
					commonutil.LoggerForJob(tfJob).Infof("TFJob %s/%s continues regardless %d Worker replica(s) failed as enableDynamicWorker is set true.",
						tfJob.Namespace, tfJob.Name, failed)
					continue
				}
				msg := fmt.Sprintf("TFJob %s/%s has failed because %d %s replica(s) failed.",
					tfJob.Namespace, tfJob.Name, failed, rtype)
				r.recorder.Event(tfJob, corev1.EventTypeNormal, commonutil.NewReason(kubeflowv1.TFJobKind, commonutil.JobFailedReason), msg)
				if jobStatus.CompletionTime == nil {
					now := metav1.Now()
					jobStatus.CompletionTime = &now
				}
				commonutil.UpdateJobConditions(jobStatus, kubeflowv1.JobFailed, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.TFJobKind, commonutil.JobFailedReason), msg)
				trainingoperatorcommon.FailedJobsCounterInc(tfJob.Namespace, r.GetFrameworkName())
			}
		}
	}
	// we assign the jobStatus to the tfJob.Status for testing purpose
	// it won't effect the main reconcile logic
	// because we already use oldStatus := jobStatus.DeepCopy() to record the oldStatus
	// and use !reflect.DeepEqual(*oldStatus, jobStatus) to decide whether to update the tfJob or not
	tfJob.Status = *jobStatus.DeepCopy()

	return nil
}

func (r *TFJobReconciler) UpdateJobStatusInApiServer(job interface{}, jobStatus *kubeflowv1.JobStatus) error {
	if jobStatus.ReplicaStatuses == nil {
		jobStatus.ReplicaStatuses = map[kubeflowv1.ReplicaType]*kubeflowv1.ReplicaStatus{}
	}

	tfJob, ok := job.(*kubeflowv1.TFJob)
	if !ok {
		return fmt.Errorf("%v is not a type of TFJob", tfJob)
	}

	startTime := time.Now()
	logger := commonutil.LoggerForJob(tfJob)
	defer func() {
		logger.Infof("Finished updating TFJobs Status %q (%v)",
			tfJob.Name, time.Since(startTime))
	}()

	tfJob = tfJob.DeepCopy()
	tfJob.Status = *jobStatus.DeepCopy()

	result := r.Status().Update(context.Background(), tfJob)

	if result != nil {
		r.Log.WithValues("tfjob", types.NamespacedName{
			Namespace: tfJob.GetNamespace(),
			Name:      tfJob.GetName(),
		})
		return result
	}

	return nil
}

// Same as Func (tc *TFController) SetClusterSpec(...) in pod.go
func (r *TFJobReconciler) SetClusterSpec(job interface{}, podTemplate *corev1.PodTemplateSpec, rtype, index string) error {
	tfjob, ok := job.(*kubeflowv1.TFJob)
	if !ok {
		return fmt.Errorf("%v is not a type of TFJob", tfjob)
	}

	// Do not set TF_CONFIG for local training jobs.
	if !isDistributed(tfjob) {
		return nil
	}
	// Generate TF_CONFIG JSON string.
	tfConfigStr, err := genTFConfigJSONStr(tfjob, rtype, index)
	if err != nil {
		return err
	}

	if tfConfigStr == "" {
		return nil
	}
	// Add TF_CONFIG environment variable to tensorflow container in the pod.
	for i := range podTemplate.Spec.Containers {
		if podTemplate.Spec.Containers[i].Name == kubeflowv1.TFJobDefaultContainerName {
			if len(podTemplate.Spec.Containers[i].Env) == 0 {
				podTemplate.Spec.Containers[i].Env = make([]corev1.EnvVar, 0)
			}
			podTemplate.Spec.Containers[i].Env = append(podTemplate.Spec.Containers[i].Env, corev1.EnvVar{
				Name:  tfConfig,
				Value: tfConfigStr,
			})
			break
		}
	}
	return nil
}

func (r *TFJobReconciler) GetDefaultContainerName() string {
	return kubeflowv1.TFJobDefaultContainerName
}

func (r *TFJobReconciler) GetDefaultContainerPortName() string {
	return kubeflowv1.TFJobDefaultPortName
}

func (r *TFJobReconciler) IsMasterRole(replicas map[kubeflowv1.ReplicaType]*kubeflowv1.ReplicaSpec,
	rtype kubeflowv1.ReplicaType, index int) bool {
	if ContainsChiefOrMasterSpec(replicas) {
		return rtype == kubeflowv1.TFJobReplicaTypeChief || rtype == kubeflowv1.TFJobReplicaTypeMaster
	}
	// else check if it is worker with index 0
	return rtype == kubeflowv1.TFJobReplicaTypeWorker && index == 0
}

// IsWorker0Completed returns true if pod of worker0 succeeded and exited with 0
func (r *TFJobReconciler) IsWorker0Completed(tfJob *kubeflowv1.TFJob, replicas map[kubeflowv1.ReplicaType]*kubeflowv1.ReplicaSpec) (bool, error) {
	worker0Completed := false
	_, ok := replicas[kubeflowv1.TFJobReplicaTypeWorker]
	if !ok {
		return true, nil
	}
	podSlices, err := r.getPodSlices(tfJob, replicas[kubeflowv1.TFJobReplicaTypeWorker].Replicas)
	if err != nil {
		return false, err
	}
	for index, podSlice := range podSlices {
		if len(podSlice) == 1 {
			pod := podSlice[0]
			exitCode := getContainerExitCode(pod)
			if index == 0 && exitCode == 0 && pod.Status.Phase == v1.PodSucceeded {
				worker0Completed = true
			}
		}
	}
	return worker0Completed, nil
}

// getPodSlices returns a slice, which element is the slice of pod.
// It gives enough information to caller to make decision to up/down scale resources.
func (r *TFJobReconciler) getPodSlices(tfjob *kubeflowv1.TFJob, replicasNum *int32) ([][]*v1.Pod, error) {
	logger := commonutil.LoggerForReplica(tfjob, strings.ToLower(string(kubeflowv1.TFJobReplicaTypeWorker)))

	pods, err := r.GetPodsForJob(tfjob)
	if err != nil {
		commonutil.LoggerForJob(tfjob).Warnf("getPodsForTFJob error %v", err)
		return nil, err
	}

	// Get all pods for the type rt.
	pods, err = r.JobController.FilterPodsForReplicaType(pods, strings.ToLower(string(kubeflowv1.TFJobReplicaTypeWorker)))
	if err != nil {
		return nil, err
	}

	podSlices := r.GetPodSlices(pods, int(*replicasNum), logger)
	return podSlices, nil
}

// onOwnerCreateFunc modify creation condition.
func (r *TFJobReconciler) onOwnerCreateFunc() func(createEvent event.TypedCreateEvent[*kubeflowv1.TFJob]) bool {
	return func(e event.TypedCreateEvent[*kubeflowv1.TFJob]) bool {
		if !r.MatchesJobLabels(e.Object) {
			return false
		}
		tfJob := e.Object
		r.Scheme.Default(tfJob)
		msg := fmt.Sprintf("TFJob %s is created.", e.Object.GetName())
		logrus.Info(msg)
		trainingoperatorcommon.CreatedJobsCounterInc(tfJob.Namespace, r.GetFrameworkName())
		commonutil.UpdateJobConditions(&tfJob.Status, kubeflowv1.JobCreated, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.TFJobKind, commonutil.JobCreatedReason), msg)
		return true
	}
}
