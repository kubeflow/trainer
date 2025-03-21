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

package pytorch

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
	"k8s.io/apimachinery/pkg/api/equality"
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
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"
	"volcano.sh/apis/pkg/apis/scheduling/v1beta1"
)

const (
	controllerName = "pytorchjob-controller"
)

// NewReconciler creates a PyTorchJob Reconciler
func NewReconciler(mgr manager.Manager, gangSchedulingSetupFunc common.GangSchedulingSetupFunc) *PyTorchJobReconciler {
	r := &PyTorchJobReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		recorder:  mgr.GetEventRecorderFor(controllerName),
		apiReader: mgr.GetAPIReader(),
		Log:       log.Log,
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

// PyTorchJobReconciler reconciles a PyTorchJob object
type PyTorchJobReconciler struct {
	common.JobController
	client.Client
	Scheme    *runtime.Scheme
	Log       logr.Logger
	recorder  record.EventRecorder
	apiReader client.Reader
}

// +kubebuilder:rbac:groups=kubeflow.org,resources=pytorchjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubeflow.org,resources=pytorchjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubeflow.org,resources=pytorchjobs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=scheduling.volcano.sh,resources=podgroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=scheduling.x-k8s.io,resources=podgroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the PyTorchJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *PyTorchJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	logger := r.Log.WithValues(kubeflowv1.PyTorchJobSingular, req.NamespacedName)

	pytorchjob := &kubeflowv1.PyTorchJob{}
	err := r.Get(ctx, req.NamespacedName, pytorchjob)
	if err != nil {
		logger.Info(err.Error(), "unable to fetch PyTorchJob", req.NamespacedName.String())
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if manager := r.ManagedByExternalController(pytorchjob.Spec.RunPolicy.ManagedBy); manager != nil {
		logger.Info("Skipping PyTorchJob managed by a custom controller", "managed-by", manager)
		return ctrl.Result{}, nil
	}

	// Check if reconciliation is needed
	jobKey, err := common.KeyFunc(pytorchjob)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get jobKey for job object %#v: %v", pytorchjob, err))
	}

	replicaTypes := util.GetReplicaTypes(pytorchjob.Spec.PyTorchReplicaSpecs)
	needReconcile := util.SatisfiedExpectations(r.Expectations, jobKey, replicaTypes)

	if !needReconcile || pytorchjob.GetDeletionTimestamp() != nil {
		logger.Info("reconcile cancelled, job does not need to do reconcile or has been deleted",
			"sync", needReconcile, "deleted", pytorchjob.GetDeletionTimestamp() != nil)
		return ctrl.Result{}, nil
	}

	// Set default priorities to pytorch job
	r.Scheme.Default(pytorchjob)

	err = r.ReconcileHPA(pytorchjob)
	if err != nil {
		logger.Error(err, "Reconcile PyTorchJob HPA error")
		return ctrl.Result{}, err
	}
	// Use common to reconcile the job related pod and service
	err = r.ReconcileJobs(pytorchjob, pytorchjob.Spec.PyTorchReplicaSpecs, pytorchjob.Status, &pytorchjob.Spec.RunPolicy)
	if err != nil {
		logger.Error(err, "Reconcile PyTorchJob error")
		return ctrl.Result{}, err
	}
	t, err := util.DurationUntilExpireTime(&pytorchjob.Spec.RunPolicy, pytorchjob.Status)
	if err != nil {
		logrus.Warnf("Reconcile PyTorchJob error %v", err)
		return ctrl.Result{}, err
	}
	if t >= 0 {
		return ctrl.Result{Requeue: true, RequeueAfter: t}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PyTorchJobReconciler) SetupWithManager(mgr ctrl.Manager, controllerThreads int) error {
	c, err := controller.New(r.ControllerName(), mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: controllerThreads,
	})
	if err != nil {
		return err
	}
	// using onOwnerCreateFunc is easier to set defaults
	if err = c.Watch(source.Kind[*kubeflowv1.PyTorchJob](mgr.GetCache(), &kubeflowv1.PyTorchJob{},
		&handler.TypedEnqueueRequestForObject[*kubeflowv1.PyTorchJob]{},
		predicate.TypedFuncs[*kubeflowv1.PyTorchJob]{CreateFunc: r.onOwnerCreateFunc()}),
	); err != nil {
		return err
	}
	// inject watching for job related pod
	if err = c.Watch(source.Kind[*corev1.Pod](mgr.GetCache(), &corev1.Pod{},
		handler.TypedEnqueueRequestForOwner[*corev1.Pod](mgr.GetScheme(), mgr.GetRESTMapper(), &kubeflowv1.PyTorchJob{}, handler.OnlyControllerOwner()),
		util.OnDependentFuncs[*corev1.Pod](r.Scheme, r.Expectations, &r.JobController))); err != nil {
		return err
	}
	// inject watching for job related service
	if err = c.Watch(source.Kind[*corev1.Service](mgr.GetCache(), &corev1.Service{},
		handler.TypedEnqueueRequestForOwner[*corev1.Service](mgr.GetScheme(), mgr.GetRESTMapper(), &kubeflowv1.PyTorchJob{}, handler.OnlyControllerOwner()),
		util.OnDependentFuncs[*corev1.Service](r.Scheme, r.Expectations, &r.JobController))); err != nil {
		return err
	}
	// skip watching volcano PodGroup if volcano PodGroup is not installed
	if _, err = mgr.GetRESTMapper().RESTMapping(schema.GroupKind{Group: v1beta1.GroupName, Kind: "PodGroup"},
		v1beta1.SchemeGroupVersion.Version); err == nil {
		// inject watching for job related volcano PodGroup
		if err = c.Watch(source.Kind[*v1beta1.PodGroup](mgr.GetCache(), &v1beta1.PodGroup{},
			handler.TypedEnqueueRequestForOwner[*v1beta1.PodGroup](mgr.GetScheme(), mgr.GetRESTMapper(), &kubeflowv1.PyTorchJob{}, handler.OnlyControllerOwner()),
			util.OnDependentFuncs[*v1beta1.PodGroup](r.Scheme, r.Expectations, &r.JobController))); err != nil {
			return err
		}
	}
	// skip watching scheduler-plugins PodGroup if scheduler-plugins PodGroup is not installed
	if _, err = mgr.GetRESTMapper().RESTMapping(schema.GroupKind{Group: schedulerpluginsv1alpha1.SchemeGroupVersion.Group, Kind: "PodGroup"},
		schedulerpluginsv1alpha1.SchemeGroupVersion.Version); err == nil {
		// inject watching for job related scheduler-plugins PodGroup
		if err = c.Watch(source.Kind[*schedulerpluginsv1alpha1.PodGroup](mgr.GetCache(), &schedulerpluginsv1alpha1.PodGroup{},
			handler.TypedEnqueueRequestForOwner[*schedulerpluginsv1alpha1.PodGroup](mgr.GetScheme(), mgr.GetRESTMapper(), &kubeflowv1.PyTorchJob{}, handler.OnlyControllerOwner()),
			util.OnDependentFuncs[*schedulerpluginsv1alpha1.PodGroup](r.Scheme, r.Expectations, &r.JobController))); err != nil {
			return err
		}
	}
	return nil
}

func (r *PyTorchJobReconciler) ControllerName() string {
	return controllerName
}

func (r *PyTorchJobReconciler) GetAPIGroupVersionKind() schema.GroupVersionKind {
	return kubeflowv1.GroupVersion.WithKind(kubeflowv1.PyTorchJobKind)
}

func (r *PyTorchJobReconciler) GetAPIGroupVersion() schema.GroupVersion {
	return kubeflowv1.GroupVersion
}

func (r *PyTorchJobReconciler) GetGroupNameLabelValue() string {
	return kubeflowv1.GroupVersion.Group
}

func (r *PyTorchJobReconciler) GetFrameworkName() string {
	return kubeflowv1.PyTorchJobFrameworkName
}

func (r *PyTorchJobReconciler) GetJobFromInformerCache(namespace, name string) (metav1.Object, error) {
	job := &kubeflowv1.PyTorchJob{}
	err := r.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, job)
	if err != nil {
		if errors.IsNotFound(err) {
			logrus.Error(err, "pytorch job not found", "namespace", namespace, "name", name)
		} else {
			logrus.Error(err, "failed to get job from api-server", "namespace", namespace, "name", name)
		}
		return nil, err
	}
	return job, nil
}

func (r *PyTorchJobReconciler) GetJobFromAPIClient(namespace, name string) (metav1.Object, error) {
	job := &kubeflowv1.PyTorchJob{}

	err := r.apiReader.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, job)
	if err != nil {
		if errors.IsNotFound(err) {
			logrus.Error(err, "pytorch job not found", "namespace", namespace, "name", name)
		} else {
			logrus.Error(err, "failed to get job from api-server", "namespace", namespace, "name", name)
		}
		return nil, err
	}
	return job, nil
}

func (r *PyTorchJobReconciler) GetPodsForJob(obj interface{}) ([]*corev1.Pod, error) {
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

func (r *PyTorchJobReconciler) GetServicesForJob(obj interface{}) ([]*corev1.Service, error) {
	job, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
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

func (r *PyTorchJobReconciler) DeleteJob(job interface{}) error {
	pytorchjob, ok := job.(*kubeflowv1.PyTorchJob)
	if !ok {
		return fmt.Errorf("%+v is not a type of PyTorchJob", job)
	}
	if err := r.Delete(context.Background(), pytorchjob); err != nil {
		r.recorder.Eventf(pytorchjob, corev1.EventTypeWarning, control.FailedDeletePodReason, "Error deleting: %v", err)
		logrus.Error(err, "failed to delete job", "namespace", pytorchjob.Namespace, "name", pytorchjob.Name)
		return err
	}
	r.recorder.Eventf(pytorchjob, corev1.EventTypeNormal, control.SuccessfulDeletePodReason, "Deleted job: %v", pytorchjob.Name)
	logrus.Info("job deleted", "namespace", pytorchjob.Namespace, "name", pytorchjob.Name)
	trainingoperatorcommon.DeletedJobsCounterInc(pytorchjob.Namespace, r.GetFrameworkName())
	return nil
}

func (r *PyTorchJobReconciler) GenLabelSelector(jobName string,
	rtype kubeflowv1.ReplicaType) *metav1.LabelSelector {
	labels := r.GenLabels(jobName)
	labels[kubeflowv1.ReplicaTypeLabel] = strings.ToLower(string(rtype))

	return &metav1.LabelSelector{
		MatchLabels: labels,
	}
}

// UpdateJobStatus updates the job status and job conditions
func (r *PyTorchJobReconciler) UpdateJobStatus(job interface{},
	replicas map[kubeflowv1.ReplicaType]*kubeflowv1.ReplicaSpec,
	jobStatus *kubeflowv1.JobStatus) error {
	pytorchjob, ok := job.(*kubeflowv1.PyTorchJob)
	if !ok {
		return fmt.Errorf("%+v is not a type of PyTorchJob", job)
	}
	pytorchjobKey, err := common.KeyFunc(pytorchjob)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for pytorchjob object %#v: %v", pytorchjob, err))
		return err
	}

	logger := commonutil.LoggerForJob(pytorchjob)

	// Set StartTime.
	if jobStatus.StartTime == nil {
		now := metav1.Now()
		jobStatus.StartTime = &now
		// enqueue a sync to check if job past ActiveDeadlineSeconds
		if pytorchjob.Spec.RunPolicy.ActiveDeadlineSeconds != nil {
			logger.Infof("Job with ActiveDeadlineSeconds will sync after %d seconds", *pytorchjob.Spec.RunPolicy.ActiveDeadlineSeconds)
			r.WorkQueue.AddAfter(pytorchjobKey, time.Duration(*pytorchjob.Spec.RunPolicy.ActiveDeadlineSeconds)*time.Second)
		}
	}

	for rtype, spec := range replicas {
		status := jobStatus.ReplicaStatuses[rtype]
		// Generate the label selector.
		status.Selector = metav1.FormatLabelSelector(r.GenLabelSelector(pytorchjob.Name, rtype))

		succeeded := status.Succeeded
		expected := *(spec.Replicas) - succeeded
		running := status.Active
		failed := status.Failed
		specReplicas := *spec.Replicas

		logrus.Infof("PyTorchJob=%s, ReplicaType=%s expected=%d, running=%d, succeeded=%d, failed=%d, Replicas=%d",
			pytorchjob.Name, rtype, expected, running, succeeded, failed, specReplicas)

		if ContainsMasterSpec(replicas) {
			if rtype == kubeflowv1.PyTorchJobReplicaTypeMaster {
				if running > 0 {
					msg := fmt.Sprintf("PyTorchJob %s is running.", pytorchjob.Name)
					commonutil.UpdateJobConditions(jobStatus, kubeflowv1.JobRunning, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.PyTorchJobKind, commonutil.JobRunningReason), msg)
				}
				created, _, successful, _, _, err := trainingoperatorcommon.GetMetricsValues(pytorchjob.Namespace, r.GetFrameworkName())
				if err != nil {
					logger.Errorf("Failed to get metrics values: %v", err)
					return err
				}
				// when master is succeed, the job is finished.
				if expected == 0 && (created>successful) {
					msg := fmt.Sprintf("PyTorchJob %s is successfully completed.", pytorchjob.Name)
					logrus.Info(msg)
					r.Recorder.Event(pytorchjob, corev1.EventTypeNormal, commonutil.NewReason(kubeflowv1.PyTorchJobKind, commonutil.JobSucceededReason), msg)
					if jobStatus.CompletionTime == nil {
						now := metav1.Now()
						jobStatus.CompletionTime = &now
					}
					commonutil.UpdateJobConditions(jobStatus, kubeflowv1.JobSucceeded, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.PyTorchJobKind, commonutil.JobSucceededReason), msg)
					trainingoperatorcommon.SuccessfulJobsCounterInc(pytorchjob.Namespace, r.GetFrameworkName())
					return nil
				}
			}
		} else {
			if rtype == kubeflowv1.PyTorchJobReplicaTypeWorker {
				// TODO(gaocegege): Support SuccessPolicy
				// Leave a succeeded condition for the following two cases:
				// 1. If all workers are succeeded.
				// 2. If `ElasticPolicy` is not nil and any worker has completed.
				if expected == 0 || (pytorchjob.Spec.ElasticPolicy != nil && succeeded > 0) {
					msg := fmt.Sprintf("PyTorchJob %s/%s successfully completed.",
						pytorchjob.Namespace, pytorchjob.Name)
					r.recorder.Event(pytorchjob, corev1.EventTypeNormal, commonutil.NewReason(kubeflowv1.PyTorchJobKind, commonutil.JobSucceededReason), msg)
					if jobStatus.CompletionTime == nil {
						now := metav1.Now()
						jobStatus.CompletionTime = &now
					}
					commonutil.UpdateJobConditions(jobStatus, kubeflowv1.JobSucceeded, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.PyTorchJobKind, commonutil.JobSucceededReason), msg)
					trainingoperatorcommon.SuccessfulJobsCounterInc(pytorchjob.Namespace, r.GetFrameworkName())
				} else if running > 0 {
					// Some workers are still running, leave a running condition.
					msg := fmt.Sprintf("PyTorchJob %s/%s is running.",
						pytorchjob.Namespace, pytorchjob.Name)
					commonutil.UpdateJobConditions(jobStatus, kubeflowv1.JobRunning, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.PyTorchJobKind, commonutil.JobRunningReason), msg)
				}
			}
		}

		if failed > 0 && (specReplicas > succeeded+running) {
			if spec.RestartPolicy != kubeflowv1.RestartPolicyNever {
				msg := fmt.Sprintf("PyTorchJob %s is restarting because %d %s replica(s) failed.", pytorchjob.Name, failed, rtype)
				r.Recorder.Event(pytorchjob, corev1.EventTypeWarning, commonutil.NewReason(kubeflowv1.PyTorchJobKind, commonutil.JobRestartingReason), msg)
				commonutil.UpdateJobConditions(jobStatus, kubeflowv1.JobRestarting, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.PyTorchJobKind, commonutil.JobRestartingReason), msg)
				trainingoperatorcommon.RestartedJobsCounterInc(pytorchjob.Namespace, r.GetFrameworkName())
			} else {
				msg := fmt.Sprintf("PyTorchJob %s is failed because %d %s replica(s) failed.", pytorchjob.Name, failed, rtype)
				r.Recorder.Event(pytorchjob, corev1.EventTypeNormal, commonutil.NewReason(kubeflowv1.PyTorchJobKind, commonutil.JobFailedReason), msg)
				if jobStatus.CompletionTime == nil {
					now := metav1.Now()
					jobStatus.CompletionTime = &now
				}
				commonutil.UpdateJobConditions(jobStatus, kubeflowv1.JobFailed, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.PyTorchJobKind, commonutil.JobFailedReason), msg)
				trainingoperatorcommon.FailedJobsCounterInc(pytorchjob.Namespace, r.GetFrameworkName())
			}
		}
	}
	return nil
}

// ContainsMasterSpec returns true if the pytorchjob contains master spec.
func ContainsMasterSpec(replicas map[kubeflowv1.ReplicaType]*kubeflowv1.ReplicaSpec) bool {
	if _, ok := replicas[kubeflowv1.PyTorchJobReplicaTypeMaster]; ok {
		return true
	}
	return false
}

// UpdateJobStatusInApiServer updates the job status in to cluster.
func (r *PyTorchJobReconciler) UpdateJobStatusInApiServer(job interface{}, jobStatus *kubeflowv1.JobStatus) error {
	if jobStatus.ReplicaStatuses == nil {
		jobStatus.ReplicaStatuses = map[kubeflowv1.ReplicaType]*kubeflowv1.ReplicaStatus{}
	}

	pytorchjob, ok := job.(*kubeflowv1.PyTorchJob)
	trainingoperatorcommon.ClearGeneratedFields(&pytorchjob.ObjectMeta)
	if !ok {
		return fmt.Errorf("%+v is not a type of PyTorchJob", job)
	}

	// Job status passed in differs with status in job, update in basis of the passed in one.
	if !equality.Semantic.DeepEqual(&pytorchjob.Status, jobStatus) {
		pytorchjob = pytorchjob.DeepCopy()
		pytorchjob.Status = *jobStatus.DeepCopy()
	}

	result := r.Status().Update(context.Background(), pytorchjob)

	if result != nil {
		r.Log.WithValues("pytorchjob", types.NamespacedName{
			Namespace: pytorchjob.GetNamespace(),
			Name:      pytorchjob.GetName(),
		})
		return result
	}

	return nil
}

// SetClusterSpec sets the cluster spec and init container for the pod
func (r *PyTorchJobReconciler) SetClusterSpec(job interface{}, podTemplate *corev1.PodTemplateSpec, rtype, index string) error {
	if err := setPodEnv(job, podTemplate, rtype, index); err != nil {
		return err
	}
	if err := setInitContainer(job, podTemplate, rtype, index, r.Log); err != nil {
		return err
	}
	return nil
}

func (r *PyTorchJobReconciler) IsMasterRole(replicas map[kubeflowv1.ReplicaType]*kubeflowv1.ReplicaSpec,
	rtype kubeflowv1.ReplicaType, index int) bool {
	if _, ok := replicas[kubeflowv1.PyTorchJobReplicaTypeMaster]; ok {
		return rtype == kubeflowv1.PyTorchJobReplicaTypeMaster
	}
	// else check if it is worker with index 0
	return rtype == kubeflowv1.PyTorchJobReplicaTypeWorker && index == 0
}

func (r *PyTorchJobReconciler) GetDefaultContainerName() string {
	return kubeflowv1.PyTorchJobDefaultContainerName
}

func (r *PyTorchJobReconciler) GetDefaultContainerPortName() string {
	return kubeflowv1.PyTorchJobDefaultPortName
}

// onOwnerCreateFunc modify creation condition.
func (r *PyTorchJobReconciler) onOwnerCreateFunc() func(createEvent event.TypedCreateEvent[*kubeflowv1.PyTorchJob]) bool {
	return func(e event.TypedCreateEvent[*kubeflowv1.PyTorchJob]) bool {
		pytorchjob := e.Object
		r.Scheme.Default(pytorchjob)
		msg := fmt.Sprintf("PyTorchJob %s is created.", e.Object.GetName())
		logrus.Info(msg)
		trainingoperatorcommon.CreatedJobsCounterInc(pytorchjob.Namespace, r.GetFrameworkName())
		commonutil.UpdateJobConditions(&pytorchjob.Status, kubeflowv1.JobCreated, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.PyTorchJobKind, commonutil.JobCreatedReason), msg)
		return true
	}
}
