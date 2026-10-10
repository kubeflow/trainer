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

package xgboost

import (
	"context"
	"fmt"
	"reflect"
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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
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
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"
	"volcano.sh/apis/pkg/apis/scheduling/v1beta1"
)

const (
	controllerName = "xgboostjob-controller"

	// Reasons for job events.
	FailedDeleteJobReason     = "FailedDeleteJob"
	SuccessfulDeleteJobReason = "SuccessfulDeleteJob"
)

// NewReconciler creates a XGBoostJob Reconciler
func NewReconciler(mgr manager.Manager, gangSchedulingSetupFunc common.GangSchedulingSetupFunc) *XGBoostJobReconciler {
	r := &XGBoostJobReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		recorder:  mgr.GetEventRecorderFor(controllerName),
		apiReader: mgr.GetAPIReader(),
		Log:       ctrl.Log.WithName("controllers").WithName(kubeflowv1.XGBoostJobKind),
	}

	// Create clients
	cfg := mgr.GetConfig()
	kubeClientSet := kubeclientset.NewForConfigOrDie(cfg)
	sharedInformers := informers.NewSharedInformerFactory(kubeClientSet, 0)
	priorityClassInformer := sharedInformers.Scheduling().V1().PriorityClasses()

	// Initialize common job controller
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

// XGBoostJobReconciler reconciles a XGBoostJob object
type XGBoostJobReconciler struct {
	common.JobController
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	recorder  record.EventRecorder
	apiReader client.Reader
}

// +kubebuilder:rbac:groups=kubeflow.org,resources=xgboostjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubeflow.org,resources=xgboostjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubeflow.org,resources=xgboostjobs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=scheduling.volcano.sh,resources=podgroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=scheduling.x-k8s.io,resources=podgroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete

// Reconcile reads that state of the cluster for a XGBoostJob object and makes changes based on the state read
// and what is in the XGBoostJob.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
func (r *XGBoostJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues(kubeflowv1.XGBoostJobSingular, req.NamespacedName)

	xgboostjob := &kubeflowv1.XGBoostJob{}
	err := r.Get(ctx, req.NamespacedName, xgboostjob)
	if err != nil {
		logger.Info(err.Error(), "unable to fetch XGBoostJob", req.NamespacedName.String())
		// Object not found, return.  Created objects are automatically garbage collected.
		// For additional cleanup logic use finalizers.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !r.MatchesJobLabels(xgboostjob) {
		return ctrl.Result{}, nil
	}

	if manager := r.ManagedByExternalController(xgboostjob.Spec.RunPolicy.ManagedBy); manager != nil {
		logger.Info("Skipping XGBoostJob managed by a custom controller", "managed-by", manager)
		return ctrl.Result{}, nil
	}

	// Check reconcile is required.
	jobKey, err := common.KeyFunc(xgboostjob)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get jobKey for job object %#v: %v", xgboostjob, err))
	}

	replicaTypes := util.GetReplicaTypes(xgboostjob.Spec.XGBReplicaSpecs)
	needSync := util.SatisfiedExpectations(r.Expectations, jobKey, replicaTypes)

	if !needSync || xgboostjob.GetDeletionTimestamp() != nil {
		logger.Info("reconcile cancelled, job does not need to do reconcile or has been deleted",
			"sync", needSync, "deleted", xgboostjob.GetDeletionTimestamp() != nil)
		return reconcile.Result{}, nil
	}

	// Set default priorities for xgboost job
	r.Scheme.Default(xgboostjob)

	// Use common to reconcile the job related pod and service
	err = r.ReconcileJobs(xgboostjob, xgboostjob.Spec.XGBReplicaSpecs, xgboostjob.Status, &xgboostjob.Spec.RunPolicy)
	if err != nil {
		logger.V(1).Error(err, "Reconcile XGBoost Job error")
		return ctrl.Result{}, err
	}

	t, err := util.DurationUntilExpireTime(&xgboostjob.Spec.RunPolicy, xgboostjob.Status)
	if err != nil {
		logrus.Warnf("Reconcile XGBoost Job error %v", err)
		return ctrl.Result{}, err
	}
	if t >= 0 {
		return ctrl.Result{Requeue: true, RequeueAfter: t}, nil
	}

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *XGBoostJobReconciler) SetupWithManager(mgr ctrl.Manager, controllerThreads int) error {
	c, err := controller.New(r.ControllerName(), mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: controllerThreads,
	})
	if err != nil {
		return err
	}
	// using onOwnerCreateFunc is easier to set defaults
	if err = c.Watch(source.Kind[*kubeflowv1.XGBoostJob](mgr.GetCache(), &kubeflowv1.XGBoostJob{},
		&handler.TypedEnqueueRequestForObject[*kubeflowv1.XGBoostJob]{},
		predicate.TypedFuncs[*kubeflowv1.XGBoostJob]{CreateFunc: r.onOwnerCreateFunc()}),
	); err != nil {
		return err
	}
	// inject watching for job related pod
	if err = c.Watch(source.Kind[*corev1.Pod](mgr.GetCache(), &corev1.Pod{},
		handler.TypedEnqueueRequestForOwner[*corev1.Pod](mgr.GetScheme(), mgr.GetRESTMapper(), &kubeflowv1.XGBoostJob{}, handler.OnlyControllerOwner()),
		util.OnDependentFuncs[*corev1.Pod](r.Scheme, r.Expectations, &r.JobController))); err != nil {
		return err
	}
	// inject watching for job related service
	if err = c.Watch(source.Kind[*corev1.Service](mgr.GetCache(), &corev1.Service{},
		handler.TypedEnqueueRequestForOwner[*corev1.Service](mgr.GetScheme(), mgr.GetRESTMapper(), &kubeflowv1.XGBoostJob{}, handler.OnlyControllerOwner()),
		util.OnDependentFuncs[*corev1.Service](r.Scheme, r.Expectations, &r.JobController))); err != nil {
		return err
	}
	// skip watching volcano PodGroup if volcano PodGroup is not installed
	if _, err = mgr.GetRESTMapper().RESTMapping(schema.GroupKind{Group: v1beta1.GroupName, Kind: "PodGroup"},
		v1beta1.SchemeGroupVersion.Version); err == nil {
		// inject watching for job related volcano PodGroup
		if err = c.Watch(source.Kind[*v1beta1.PodGroup](mgr.GetCache(), &v1beta1.PodGroup{},
			handler.TypedEnqueueRequestForOwner[*v1beta1.PodGroup](mgr.GetScheme(), mgr.GetRESTMapper(), &kubeflowv1.XGBoostJob{}, handler.OnlyControllerOwner()),
			util.OnDependentFuncs[*v1beta1.PodGroup](r.Scheme, r.Expectations, &r.JobController))); err != nil {
			return err
		}
	}
	// skip watching scheduler-plugins PodGroup if scheduler-plugins PodGroup is not installed
	if _, err = mgr.GetRESTMapper().RESTMapping(schema.GroupKind{Group: schedulerpluginsv1alpha1.SchemeGroupVersion.Group, Kind: "PodGroup"},
		schedulerpluginsv1alpha1.SchemeGroupVersion.Version); err == nil {
		// inject watching for job related scheduler-plugins PodGroup
		if err = c.Watch(source.Kind[*schedulerpluginsv1alpha1.PodGroup](mgr.GetCache(), &schedulerpluginsv1alpha1.PodGroup{},
			handler.TypedEnqueueRequestForOwner[*schedulerpluginsv1alpha1.PodGroup](mgr.GetScheme(), mgr.GetRESTMapper(), &kubeflowv1.XGBoostJob{}, handler.OnlyControllerOwner()),
			util.OnDependentFuncs[*schedulerpluginsv1alpha1.PodGroup](r.Scheme, r.Expectations, &r.JobController))); err != nil {
			return err
		}
	}
	return nil
}

func (r *XGBoostJobReconciler) ControllerName() string {
	return controllerName
}

func (r *XGBoostJobReconciler) GetAPIGroupVersionKind() schema.GroupVersionKind {
	return kubeflowv1.GroupVersion.WithKind(kubeflowv1.XGBoostJobKind)
}

func (r *XGBoostJobReconciler) GetAPIGroupVersion() schema.GroupVersion {
	return kubeflowv1.GroupVersion
}

func (r *XGBoostJobReconciler) GetGroupNameLabelValue() string {
	return kubeflowv1.GroupVersion.Group
}

func (r *XGBoostJobReconciler) GetFrameworkName() string {
	return kubeflowv1.XGBoostJobFrameworkName
}

// GetJobFromInformerCache returns the Job from Informer Cache
func (r *XGBoostJobReconciler) GetJobFromInformerCache(namespace, name string) (metav1.Object, error) {
	job := &kubeflowv1.XGBoostJob{}
	// Default reader for XGBoostJob is cache reader.
	err := r.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, job)
	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Error(err, "xgboost job not found", "namespace", namespace, "name", name)
		} else {
			r.Log.Error(err, "failed to get job from api-server", "namespace", namespace, "name", name)
		}
		return nil, err
	}
	return job, nil
}

// GetJobFromAPIClient returns the Job from API server
func (r *XGBoostJobReconciler) GetJobFromAPIClient(namespace, name string) (metav1.Object, error) {
	job := &kubeflowv1.XGBoostJob{}

	err := r.apiReader.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, job)
	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Error(err, "xgboost job not found", "namespace", namespace, "name", name)
		} else {
			r.Log.Error(err, "failed to get job from api-server", "namespace", namespace, "name", name)
		}
		return nil, err
	}
	return job, nil
}

// GetPodsForJob returns the pods managed by the job. This can be achieved by selecting pods using label key "job-name"
// i.e. all pods created by the job will come with label "job-name" = <this_job_name>
func (r *XGBoostJobReconciler) GetPodsForJob(obj interface{}) ([]*corev1.Pod, error) {
	job, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	// List all pods to include those that don't match the selector anymore
	// but have a ControllerRef pointing to this controller.
	podlist := &corev1.PodList{}
	err = r.List(context.Background(), podlist, client.MatchingLabels(r.GenLabels(job.GetName())), client.InNamespace(job.GetNamespace()))
	if err != nil {
		return nil, err
	}

	return util.JobControlledPodList(podlist.Items, job), nil
}

// GetServicesForJob returns the services managed by the job. This can be achieved by selecting services using label key "job-name"
// i.e. all services created by the job will come with label "job-name" = <this_job_name>
func (r *XGBoostJobReconciler) GetServicesForJob(obj interface{}) ([]*corev1.Service, error) {
	job, err := meta.Accessor(obj)
	if err != nil {
		return nil, fmt.Errorf("%+v is not a type of XGBoostJob", job)
	}
	// List all pods to include those that don't match the selector anymore
	// but have a ControllerRef pointing to this controller.
	serviceList := &corev1.ServiceList{}
	err = r.List(context.Background(), serviceList, client.MatchingLabels(r.GenLabels(job.GetName())), client.InNamespace(job.GetNamespace()))
	if err != nil {
		return nil, err
	}

	ret := util.ConvertServiceList(serviceList.Items)
	return ret, nil
}

// DeleteJob deletes the job
func (r *XGBoostJobReconciler) DeleteJob(job interface{}) error {
	xgboostjob, ok := job.(*kubeflowv1.XGBoostJob)
	if !ok {
		return fmt.Errorf("%+v is not a type of XGBoostJob", xgboostjob)
	}
	if err := r.Delete(context.Background(), xgboostjob); err != nil {
		r.recorder.Eventf(xgboostjob, corev1.EventTypeWarning, FailedDeleteJobReason, "Error deleting: %v", err)
		r.Log.Error(err, "failed to delete job", "namespace", xgboostjob.Namespace, "name", xgboostjob.Name)
		return err
	}
	r.recorder.Eventf(xgboostjob, corev1.EventTypeNormal, SuccessfulDeleteJobReason, "Deleted job: %v", xgboostjob.Name)
	r.Log.Info("job deleted", "namespace", xgboostjob.Namespace, "name", xgboostjob.Name)
	trainingoperatorcommon.DeletedJobsCounterInc(xgboostjob.Namespace, r.GetFrameworkName())
	return nil
}

// UpdateJobStatus updates the job status and job conditions
func (r *XGBoostJobReconciler) UpdateJobStatus(job interface{}, replicas map[kubeflowv1.ReplicaType]*kubeflowv1.ReplicaSpec, jobStatus *kubeflowv1.JobStatus) error {
	xgboostJob, ok := job.(*kubeflowv1.XGBoostJob)
	if !ok {
		return fmt.Errorf("%+v is not a type of xgboostJob", xgboostJob)
	}

	xgboostJobKey, err := common.KeyFunc(xgboostJob)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for xgboostjob object %#v: %v", xgboostJob, err))
		return err
	}

	logger := commonutil.LoggerForJob(xgboostJob)

	// Set StartTime.
	if jobStatus.StartTime == nil {
		now := metav1.Now()
		jobStatus.StartTime = &now
		// enqueue a sync to check if job past ActiveDeadlineSeconds
		if xgboostJob.Spec.RunPolicy.ActiveDeadlineSeconds != nil {
			logger.Infof("Job with ActiveDeadlineSeconds will sync after %d seconds", *xgboostJob.Spec.RunPolicy.ActiveDeadlineSeconds)
			r.WorkQueue.AddAfter(xgboostJobKey, time.Duration(*xgboostJob.Spec.RunPolicy.ActiveDeadlineSeconds)*time.Second)
		}
	}

	for rtype, spec := range replicas {
		status := jobStatus.ReplicaStatuses[rtype]

		succeeded := status.Succeeded
		expected := *(spec.Replicas) - succeeded
		running := status.Active
		failed := status.Failed
		runningMsg := fmt.Sprintf("XGBoostJob %s is running.", xgboostJob.Name)

		logrus.Infof("XGBoostJob=%s, ReplicaType=%s expected=%d, running=%d, succeeded=%d , failed=%d",
			xgboostJob.Name, rtype, expected, running, succeeded, failed)

		if rtype == kubeflowv1.XGBoostJobReplicaTypeMaster {
			if running > 0 {
				commonutil.UpdateJobConditions(jobStatus, kubeflowv1.JobRunning, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.XGBoostJobKind, commonutil.JobRunningReason), runningMsg)
			}
			// when master is succeed, the job is finished.
			if expected == 0 {
				commonutil.UpdateJobConditions(jobStatus, kubeflowv1.JobRunning, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.XGBoostJobKind, commonutil.JobRunningReason), runningMsg)
				msg := fmt.Sprintf("XGBoostJob %s is successfully completed.", xgboostJob.Name)
				logrus.Info(msg)
				r.Recorder.Event(xgboostJob, corev1.EventTypeNormal, commonutil.NewReason(kubeflowv1.XGBoostJobKind, commonutil.JobSucceededReason), msg)
				if jobStatus.CompletionTime == nil {
					now := metav1.Now()
					jobStatus.CompletionTime = &now
				}
				commonutil.UpdateJobConditions(jobStatus, kubeflowv1.JobSucceeded, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.XGBoostJobKind, commonutil.JobSucceededReason), msg)
				trainingoperatorcommon.SuccessfulJobsCounterInc(xgboostJob.Namespace, r.GetFrameworkName())
				return nil
			}
		}
		if failed > 0 {
			commonutil.UpdateJobConditions(jobStatus, kubeflowv1.JobRunning, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.XGBoostJobKind, commonutil.JobRunningReason), runningMsg)
			if spec.RestartPolicy == kubeflowv1.RestartPolicyExitCode {
				msg := fmt.Sprintf("XGBoostJob %s is restarting because %d %s replica(s) failed.", xgboostJob.Name, failed, rtype)
				r.Recorder.Event(xgboostJob, corev1.EventTypeWarning, commonutil.NewReason(kubeflowv1.XGBoostJobKind, commonutil.JobRestartingReason), msg)
				commonutil.UpdateJobConditions(jobStatus, kubeflowv1.JobRestarting, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.XGBoostJobKind, commonutil.JobRestartingReason), msg)
				trainingoperatorcommon.RestartedJobsCounterInc(xgboostJob.Namespace, r.GetFrameworkName())
			} else {
				msg := fmt.Sprintf("XGBoostJob %s is failed because %d %s replica(s) failed.", xgboostJob.Name, failed, rtype)
				r.Recorder.Event(xgboostJob, corev1.EventTypeNormal, commonutil.NewReason(kubeflowv1.XGBoostJobKind, commonutil.JobFailedReason), msg)
				if jobStatus.CompletionTime == nil {
					now := metav1.Now()
					jobStatus.CompletionTime = &now
				}
				commonutil.UpdateJobConditions(jobStatus, kubeflowv1.JobFailed, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.XGBoostJobKind, commonutil.JobFailedReason), msg)
				trainingoperatorcommon.FailedJobsCounterInc(xgboostJob.Namespace, r.GetFrameworkName())
			}
		}
	}
	return nil
}

// UpdateJobStatusInApiServer updates the job status in to cluster.
func (r *XGBoostJobReconciler) UpdateJobStatusInApiServer(job interface{}, jobStatus *kubeflowv1.JobStatus) error {
	if jobStatus.ReplicaStatuses == nil {
		jobStatus.ReplicaStatuses = map[kubeflowv1.ReplicaType]*kubeflowv1.ReplicaStatus{}
	}

	xgboostjob, ok := job.(*kubeflowv1.XGBoostJob)
	if !ok {
		return fmt.Errorf("%+v is not a type of XGBoostJob", xgboostjob)
	}

	// Job status passed in differs with status in job, update in basis of the passed in one.
	if !reflect.DeepEqual(&xgboostjob.Status, jobStatus) {
		xgboostjob = xgboostjob.DeepCopy()
		xgboostjob.Status = *jobStatus.DeepCopy()
	}

	result := r.Status().Update(context.Background(), xgboostjob)

	if result != nil {
		commonutil.LoggerForJob(xgboostjob).Error(result, "failed to update XGBoost Job conditions in the API server")
		return result
	}

	return nil
}

// SetClusterSpec sets the cluster spec for the pod
func (r *XGBoostJobReconciler) SetClusterSpec(job interface{}, podTemplate *corev1.PodTemplateSpec, rtype, index string) error {
	return SetPodEnv(job, podTemplate, rtype, index)
}

func (r *XGBoostJobReconciler) GetDefaultContainerName() string {
	return kubeflowv1.XGBoostJobDefaultContainerName
}

func (r *XGBoostJobReconciler) GetDefaultContainerPortName() string {
	return kubeflowv1.XGBoostJobDefaultPortName
}

func (r *XGBoostJobReconciler) IsMasterRole(replicas map[kubeflowv1.ReplicaType]*kubeflowv1.ReplicaSpec,
	rtype kubeflowv1.ReplicaType, index int) bool {
	return string(rtype) == string(kubeflowv1.XGBoostJobReplicaTypeMaster)
}

// onOwnerCreateFunc modify creation condition.
func (r *XGBoostJobReconciler) onOwnerCreateFunc() func(createEvent event.TypedCreateEvent[*kubeflowv1.XGBoostJob]) bool {
	return func(e event.TypedCreateEvent[*kubeflowv1.XGBoostJob]) bool {
		if !r.MatchesJobLabels(e.Object) {
			return false
		}
		xgboostJob := e.Object
		r.Scheme.Default(xgboostJob)
		msg := fmt.Sprintf("XGBoostJob %s is created.", e.Object.GetName())
		logrus.Info()
		trainingoperatorcommon.CreatedJobsCounterInc(xgboostJob.Namespace, r.GetFrameworkName())
		commonutil.UpdateJobConditions(&xgboostJob.Status, kubeflowv1.JobCreated, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.XGBoostJobKind, commonutil.JobCreatedReason), msg)
		return true
	}
}
