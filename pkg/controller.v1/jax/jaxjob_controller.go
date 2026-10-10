// Copyright 2024 The Kubeflow Authors
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

package jax

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
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"
	"volcano.sh/apis/pkg/apis/scheduling/v1beta1"
)

const (
	controllerName = "jaxjob-controller"
)

// NewReconciler creates a JAXJob Reconciler
func NewReconciler(mgr manager.Manager, gangSchedulingSetupFunc common.GangSchedulingSetupFunc) *JAXJobReconciler {
	r := &JAXJobReconciler{
		client:    mgr.GetClient(),
		scheme:    mgr.GetScheme(),
		recorder:  mgr.GetEventRecorderFor(controllerName),
		apiReader: mgr.GetAPIReader(),
		log:       ctrl.Log.WithName(controllerName),
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

// JAXJobReconciler reconciles a JAXJob object
type JAXJobReconciler struct {
	common.JobController
	client    client.Client
	scheme    *runtime.Scheme
	log       logr.Logger
	recorder  record.EventRecorder
	apiReader client.Reader
}

// +kubebuilder:rbac:groups=kubeflow.org,resources=jaxjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubeflow.org,resources=jaxjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubeflow.org,resources=jaxjobs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=scheduling.volcano.sh,resources=podgroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=scheduling.x-k8s.io,resources=podgroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the JAXJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *JAXJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	jaxjob := &kubeflowv1.JAXJob{}
	err := r.client.Get(ctx, req.NamespacedName, jaxjob)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !r.MatchesJobLabels(jaxjob) {
		return ctrl.Result{}, nil
	}

	// log := ctrl.LoggerFrom(ctx).WithValues("jaxjob", klog.KObj(&jaxjob))
	// ctrl.LoggerInto(ctx, log)
	// log.V(2).Info("Reconciling JAXJob")

	// Check if reconciliation is needed
	jobKey, err := common.KeyFunc(jaxjob)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get jobKey for job object %#v: %v", jaxjob, err))
	}

	replicaTypes := util.GetReplicaTypes(jaxjob.Spec.JAXReplicaSpecs)
	needReconcile := util.SatisfiedExpectations(r.Expectations, jobKey, replicaTypes)

	if !needReconcile || jaxjob.GetDeletionTimestamp() != nil {
		r.log.Info("reconcile cancelled, job does not need to do reconcile or has been deleted",
			"sync", needReconcile, "deleted", jaxjob.GetDeletionTimestamp() != nil)
		return ctrl.Result{}, nil
	}

	// Set default priorities to jax job
	r.scheme.Default(jaxjob)

	// Use common to reconcile the job related pod and service
	err = r.ReconcileJobs(jaxjob, jaxjob.Spec.JAXReplicaSpecs, jaxjob.Status, &jaxjob.Spec.RunPolicy)
	if err != nil {
		r.log.Error(err, "Reconcile JAXJob error")
		return ctrl.Result{}, err
	}
	t, err := util.DurationUntilExpireTime(&jaxjob.Spec.RunPolicy, jaxjob.Status)
	if err != nil {
		logrus.Warnf("Reconcile JAXJob error %v", err)
		return ctrl.Result{}, err
	}
	if t >= 0 {
		return ctrl.Result{Requeue: true, RequeueAfter: t}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *JAXJobReconciler) SetupWithManager(mgr ctrl.Manager, controllerThreads int) error {
	c, err := controller.New(r.ControllerName(), mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: controllerThreads,
	})
	if err != nil {
		return err
	}
	// using onOwnerCreateFunc is easier to set defaults
	if err = c.Watch(source.Kind[*kubeflowv1.JAXJob](mgr.GetCache(), &kubeflowv1.JAXJob{},
		&handler.TypedEnqueueRequestForObject[*kubeflowv1.JAXJob]{},
		predicate.TypedFuncs[*kubeflowv1.JAXJob]{CreateFunc: r.onOwnerCreateFunc()}),
	); err != nil {
		return err
	}
	// inject watching for job related pod
	if err = c.Watch(source.Kind[*corev1.Pod](mgr.GetCache(), &corev1.Pod{},
		handler.TypedEnqueueRequestForOwner[*corev1.Pod](mgr.GetScheme(), mgr.GetRESTMapper(), &kubeflowv1.JAXJob{}, handler.OnlyControllerOwner()),
		util.OnDependentFuncs[*corev1.Pod](r.scheme, r.Expectations, &r.JobController))); err != nil {
		return err
	}
	// inject watching for job related service
	if err = c.Watch(source.Kind[*corev1.Service](mgr.GetCache(), &corev1.Service{},
		handler.TypedEnqueueRequestForOwner[*corev1.Service](mgr.GetScheme(), mgr.GetRESTMapper(), &kubeflowv1.JAXJob{}, handler.OnlyControllerOwner()),
		util.OnDependentFuncs[*corev1.Service](r.scheme, r.Expectations, &r.JobController))); err != nil {
		return err
	}
	// skip watching volcano PodGroup if volcano PodGroup is not installed
	if _, err = mgr.GetRESTMapper().RESTMapping(schema.GroupKind{Group: v1beta1.GroupName, Kind: "PodGroup"},
		v1beta1.SchemeGroupVersion.Version); err == nil {
		// inject watching for job related volcano PodGroup
		if err = c.Watch(source.Kind[*v1beta1.PodGroup](mgr.GetCache(), &v1beta1.PodGroup{},
			handler.TypedEnqueueRequestForOwner[*v1beta1.PodGroup](mgr.GetScheme(), mgr.GetRESTMapper(), &kubeflowv1.JAXJob{}, handler.OnlyControllerOwner()),
			util.OnDependentFuncs[*v1beta1.PodGroup](r.scheme, r.Expectations, &r.JobController))); err != nil {
			return err
		}
	}
	// skip watching scheduler-plugins PodGroup if scheduler-plugins PodGroup is not installed
	if _, err = mgr.GetRESTMapper().RESTMapping(schema.GroupKind{Group: schedulerpluginsv1alpha1.SchemeGroupVersion.Group, Kind: "PodGroup"},
		schedulerpluginsv1alpha1.SchemeGroupVersion.Version); err == nil {
		// inject watching for job related scheduler-plugins PodGroup
		if err = c.Watch(source.Kind[*schedulerpluginsv1alpha1.PodGroup](mgr.GetCache(), &schedulerpluginsv1alpha1.PodGroup{},
			handler.TypedEnqueueRequestForOwner[*schedulerpluginsv1alpha1.PodGroup](mgr.GetScheme(), mgr.GetRESTMapper(), &kubeflowv1.JAXJob{}, handler.OnlyControllerOwner()),
			util.OnDependentFuncs[*schedulerpluginsv1alpha1.PodGroup](r.scheme, r.Expectations, &r.JobController))); err != nil {
			return err
		}
	}
	return nil
}

func (r *JAXJobReconciler) ControllerName() string {
	return controllerName
}

func (r *JAXJobReconciler) GetAPIGroupVersionKind() schema.GroupVersionKind {
	return kubeflowv1.GroupVersion.WithKind(kubeflowv1.JAXJobKind)
}

func (r *JAXJobReconciler) GetAPIGroupVersion() schema.GroupVersion {
	return kubeflowv1.GroupVersion
}

func (r *JAXJobReconciler) GetGroupNameLabelValue() string {
	return kubeflowv1.GroupVersion.Group
}

func (r *JAXJobReconciler) GetFrameworkName() string {
	return kubeflowv1.JAXJobFrameworkName
}

func (r *JAXJobReconciler) GetJobFromInformerCache(namespace, name string) (metav1.Object, error) {
	job := &kubeflowv1.JAXJob{}
	err := r.client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, job)
	if err != nil {
		if errors.IsNotFound(err) {
			logrus.Error(err, "jax job not found", "namespace", namespace, "name", name)
		} else {
			logrus.Error(err, "failed to get job from api-server", "namespace", namespace, "name", name)
		}
		return nil, err
	}
	return job, nil
}

func (r *JAXJobReconciler) GetJobFromAPIClient(namespace, name string) (metav1.Object, error) {
	job := &kubeflowv1.JAXJob{}

	err := r.apiReader.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, job)
	if err != nil {
		if errors.IsNotFound(err) {
			logrus.Error(err, "jax job not found", "namespace", namespace, "name", name)
		} else {
			logrus.Error(err, "failed to get job from api-server", "namespace", namespace, "name", name)
		}
		return nil, err
	}
	return job, nil
}

func (r *JAXJobReconciler) GetPodsForJob(obj interface{}) ([]*corev1.Pod, error) {
	job, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	// List all pods to include those that don't match the selector anymore
	// but have a ControllerRef pointing to this controller.
	podlist := &corev1.PodList{}
	err = r.client.List(context.Background(), podlist, client.MatchingLabels(r.GenLabels(job.GetName())), client.InNamespace(job.GetNamespace()))
	if err != nil {
		return nil, err
	}

	return util.JobControlledPodList(podlist.Items, job), nil
}

func (r *JAXJobReconciler) GetServicesForJob(obj interface{}) ([]*corev1.Service, error) {
	job, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	// List all pods to include those that don't match the selector anymore
	// but have a ControllerRef pointing to this controller.
	serviceList := &corev1.ServiceList{}
	err = r.client.List(context.Background(), serviceList, client.MatchingLabels(r.GenLabels(job.GetName())), client.InNamespace(job.GetNamespace()))
	if err != nil {
		return nil, err
	}

	ret := util.ConvertServiceList(serviceList.Items)
	return ret, nil
}

func (r *JAXJobReconciler) DeleteJob(job interface{}) error {
	jaxjob, ok := job.(*kubeflowv1.JAXJob)
	if !ok {
		return fmt.Errorf("%+v is not a type of JAXJob", job)
	}
	if err := r.client.Delete(context.Background(), jaxjob); err != nil {
		r.recorder.Eventf(jaxjob, corev1.EventTypeWarning, control.FailedDeletePodReason, "Error deleting: %v", err)
		logrus.Error(err, "failed to delete job", "namespace", jaxjob.Namespace, "name", jaxjob.Name)
		return err
	}
	r.recorder.Eventf(jaxjob, corev1.EventTypeNormal, control.SuccessfulDeletePodReason, "Deleted job: %v", jaxjob.Name)
	logrus.Info("job deleted", "namespace", jaxjob.Namespace, "name", jaxjob.Name)
	trainingoperatorcommon.DeletedJobsCounterInc(jaxjob.Namespace, r.GetFrameworkName())
	return nil
}

func (r *JAXJobReconciler) GenLabelSelector(jobName string,
	rtype kubeflowv1.ReplicaType) *metav1.LabelSelector {
	labels := r.GenLabels(jobName)
	labels[kubeflowv1.ReplicaTypeLabel] = strings.ToLower(string(rtype))

	return &metav1.LabelSelector{
		MatchLabels: labels,
	}
}

// UpdateJobStatus updates the job status and job conditions
func (r *JAXJobReconciler) UpdateJobStatus(job interface{},
	replicas map[kubeflowv1.ReplicaType]*kubeflowv1.ReplicaSpec,
	jobStatus *kubeflowv1.JobStatus) error {
	jaxjob, ok := job.(*kubeflowv1.JAXJob)
	if !ok {
		return fmt.Errorf("%+v is not a type of JAXJob", job)
	}
	jaxjobKey, err := common.KeyFunc(jaxjob)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for jaxjob object %#v: %v", jaxjob, err))
		return err
	}

	logger := commonutil.LoggerForJob(jaxjob)

	// Set StartTime.
	if jobStatus.StartTime == nil {
		now := metav1.Now()
		jobStatus.StartTime = &now
		// enqueue a sync to check if job past ActiveDeadlineSeconds
		if jaxjob.Spec.RunPolicy.ActiveDeadlineSeconds != nil {
			logger.Infof("Job with ActiveDeadlineSeconds will sync after %d seconds", *jaxjob.Spec.RunPolicy.ActiveDeadlineSeconds)
			r.WorkQueue.AddAfter(jaxjobKey, time.Duration(*jaxjob.Spec.RunPolicy.ActiveDeadlineSeconds)*time.Second)
		}
	}

	for rtype, spec := range replicas {
		status := jobStatus.ReplicaStatuses[rtype]
		// Generate the label selector.
		status.Selector = metav1.FormatLabelSelector(r.GenLabelSelector(jaxjob.Name, rtype))

		succeeded := status.Succeeded
		expected := *(spec.Replicas) - succeeded
		running := status.Active
		failed := status.Failed
		specReplicas := *spec.Replicas

		logrus.Infof("JAXJob=%s, ReplicaType=%s expected=%d, running=%d, succeeded=%d, failed=%d, Replicas=%d",
			jaxjob.Name, rtype, expected, running, succeeded, failed, specReplicas)

		if rtype == kubeflowv1.JAXJobReplicaTypeWorker {
			if expected == 0 {
				msg := fmt.Sprintf("JAXJob %s/%s successfully completed.",
					jaxjob.Namespace, jaxjob.Name)
				r.recorder.Event(jaxjob, corev1.EventTypeNormal, commonutil.NewReason(kubeflowv1.JAXJobKind, commonutil.JobSucceededReason), msg)
				if jobStatus.CompletionTime == nil {
					now := metav1.Now()
					jobStatus.CompletionTime = &now
				}
				commonutil.UpdateJobConditions(jobStatus, kubeflowv1.JobSucceeded, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.JAXJobKind, commonutil.JobSucceededReason), msg)
				trainingoperatorcommon.SuccessfulJobsCounterInc(jaxjob.Namespace, r.GetFrameworkName())
			} else if running > 0 {
				// Some workers are still running, leave a running condition.
				msg := fmt.Sprintf("JAXJob %s/%s is running.",
					jaxjob.Namespace, jaxjob.Name)
				commonutil.UpdateJobConditions(jobStatus, kubeflowv1.JobRunning, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.JAXJobKind, commonutil.JobRunningReason), msg)
			}
		}

		if failed > 0 && (specReplicas > succeeded+running) {
			if spec.RestartPolicy != kubeflowv1.RestartPolicyNever {
				msg := fmt.Sprintf("JAXJob %s is restarting because %d %s replica(s) failed.", jaxjob.Name, failed, rtype)
				r.Recorder.Event(jaxjob, corev1.EventTypeWarning, commonutil.NewReason(kubeflowv1.JAXJobKind, commonutil.JobRestartingReason), msg)
				commonutil.UpdateJobConditions(jobStatus, kubeflowv1.JobRestarting, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.JAXJobKind, commonutil.JobRestartingReason), msg)
				trainingoperatorcommon.RestartedJobsCounterInc(jaxjob.Namespace, r.GetFrameworkName())
			} else {
				msg := fmt.Sprintf("JAXJob %s is failed because %d %s replica(s) failed.", jaxjob.Name, failed, rtype)
				r.Recorder.Event(jaxjob, corev1.EventTypeNormal, commonutil.NewReason(kubeflowv1.JAXJobKind, commonutil.JobFailedReason), msg)
				if jobStatus.CompletionTime == nil {
					now := metav1.Now()
					jobStatus.CompletionTime = &now
				}
				commonutil.UpdateJobConditions(jobStatus, kubeflowv1.JobFailed, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.JAXJobKind, commonutil.JobFailedReason), msg)
				trainingoperatorcommon.FailedJobsCounterInc(jaxjob.Namespace, r.GetFrameworkName())
			}
		}
	}
	return nil
}

// UpdateJobStatusInApiServer updates the job status in to cluster.
func (r *JAXJobReconciler) UpdateJobStatusInApiServer(job interface{}, jobStatus *kubeflowv1.JobStatus) error {
	if jobStatus.ReplicaStatuses == nil {
		jobStatus.ReplicaStatuses = map[kubeflowv1.ReplicaType]*kubeflowv1.ReplicaStatus{}
	}

	jaxjob, ok := job.(*kubeflowv1.JAXJob)
	trainingoperatorcommon.ClearGeneratedFields(&jaxjob.ObjectMeta)
	if !ok {
		return fmt.Errorf("%+v is not a type of JAXJob", job)
	}

	// Job status passed in differs with status in job, update in basis of the passed in one.
	if !equality.Semantic.DeepEqual(&jaxjob.Status, jobStatus) {
		jaxjob = jaxjob.DeepCopy()
		jaxjob.Status = *jobStatus.DeepCopy()
	}

	result := r.client.Status().Update(context.Background(), jaxjob)

	if result != nil {
		r.log.WithValues("jaxjob", types.NamespacedName{
			Namespace: jaxjob.GetNamespace(),
			Name:      jaxjob.GetName(),
		})
		return result
	}

	return nil
}

// SetClusterSpec sets the cluster spec and init container for the pod
func (r *JAXJobReconciler) SetClusterSpec(job interface{}, podTemplate *corev1.PodTemplateSpec, rtype, index string) error {
	jaxjob, ok := job.(*kubeflowv1.JAXJob)
	if !ok {
		return fmt.Errorf("%+v is not a type of JAXJob", job)
	}
	if err := setPodEnv(jaxjob, podTemplate, rtype, index); err != nil {
		return err
	}
	return nil
}

func (r *JAXJobReconciler) GetDefaultContainerName() string {
	return kubeflowv1.JAXJobDefaultContainerName
}

func (r *JAXJobReconciler) GetDefaultContainerPortName() string {
	return kubeflowv1.JAXJobDefaultPortName
}

func (r *JAXJobReconciler) IsMasterRole(replicas map[kubeflowv1.ReplicaType]*kubeflowv1.ReplicaSpec,
	rtype kubeflowv1.ReplicaType, index int) bool {
	return index == 0
}

// onOwnerCreateFunc modify creation condition.
func (r *JAXJobReconciler) onOwnerCreateFunc() func(createEvent event.TypedCreateEvent[*kubeflowv1.JAXJob]) bool {
	return func(e event.TypedCreateEvent[*kubeflowv1.JAXJob]) bool {
		if !r.MatchesJobLabels(e.Object) {
			return false
		}
		jaxjob := e.Object
		r.scheme.Default(jaxjob)
		msg := fmt.Sprintf("JAXJob %s is created.", e.Object.GetName())
		logrus.Info(msg)
		trainingoperatorcommon.CreatedJobsCounterInc(jaxjob.Namespace, r.GetFrameworkName())
		commonutil.UpdateJobConditions(&jaxjob.Status, kubeflowv1.JobCreated, corev1.ConditionTrue, commonutil.NewReason(kubeflowv1.JAXJobKind, commonutil.JobCreatedReason), msg)
		return true
	}
}
