/*
Copyright The Kubeflow Authors.

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

package controller

import (
	"context"
	stderrors "errors"
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	katibapi "github.com/kubeflow/katib/pkg/apis/manager/v1beta1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlpkg "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	optimizationjob "github.com/kubeflow/trainer/v2/pkg/util/optimizationjob"
	"github.com/kubeflow/trainer/v2/pkg/util/trainjob"
)

// SearchAlgorithmClient abstracts the gRPC call so we can mock it in unit tests
type SearchAlgorithmClient interface {
	GetSuggestions(ctx context.Context, addr string, req *katibapi.GetSuggestionsRequest) ([][]trainer.ParameterAssignment, error)
}

// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=optimizationjobs,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=optimizationjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=trainjobs,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

type OptimizationJobReconciler struct {
	client.Client
	Scheme                *runtime.Scheme
	Log                   logr.Logger
	Recorder              events.EventRecorder
	SearchAlgorithmClient SearchAlgorithmClient
	APIReader             client.Reader
	expectationsMu        sync.Mutex
	pendingTrials         map[types.NamespacedName]*trialCreationExpectations
}

type trialCreationExpectations struct {
	uid   types.UID
	names map[string]time.Time
}

const (
	trialCacheRequeueDelay = time.Second
	// Read the live API only after the cache has had time to observe a created trial.
	trialExpectationVerifyDelay = 30 * time.Second
)

func NewOptimizationJobReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	recorder events.EventRecorder,
	algorithmClient SearchAlgorithmClient,
) *OptimizationJobReconciler {
	if algorithmClient == nil {
		algorithmClient = &DefaultSearchAlgorithmClient{}
	}

	return &OptimizationJobReconciler{
		Client:                client,
		Scheme:                scheme,
		Log:                   ctrl.Log.WithName("optimizationjob-controller"),
		Recorder:              recorder,
		SearchAlgorithmClient: algorithmClient,
	}
}

const jobOwnerKey = ".metadata.controller"

func (r *OptimizationJobReconciler) expectTrialCreation(key types.NamespacedName, uid types.UID, name string) {
	r.expectationsMu.Lock()
	defer r.expectationsMu.Unlock()

	if r.pendingTrials == nil {
		r.pendingTrials = make(map[types.NamespacedName]*trialCreationExpectations)
	}
	expectations := r.pendingTrials[key]
	if expectations == nil || expectations.uid != uid {
		expectations = &trialCreationExpectations{uid: uid, names: make(map[string]time.Time)}
		r.pendingTrials[key] = expectations
	}
	expectations.names[name] = time.Now()
}

func (r *OptimizationJobReconciler) forgetTrialCreation(key types.NamespacedName, name string) {
	r.expectationsMu.Lock()
	defer r.expectationsMu.Unlock()

	if expectations := r.pendingTrials[key]; expectations != nil {
		delete(expectations.names, name)
		if len(expectations.names) == 0 {
			delete(r.pendingTrials, key)
		}
	}
}

func (r *OptimizationJobReconciler) observeTrialCreations(key types.NamespacedName, uid types.UID, trainJobs []trainer.TrainJob) []string {
	r.expectationsMu.Lock()
	defer r.expectationsMu.Unlock()

	expectations := r.pendingTrials[key]
	if expectations == nil {
		return nil
	}
	if expectations.uid != uid {
		delete(r.pendingTrials, key)
		return nil
	}
	for _, trainJob := range trainJobs {
		delete(expectations.names, trainJob.Name)
	}
	if len(expectations.names) == 0 {
		delete(r.pendingTrials, key)
	}
	var overdue []string
	for name, createdAt := range expectations.names {
		if time.Since(createdAt) >= trialExpectationVerifyDelay {
			overdue = append(overdue, name)
		}
	}
	return overdue
}

func (r *OptimizationJobReconciler) verifyPendingTrialCreations(ctx context.Context, optJob *trainer.OptimizationJob, overdue []string) (int32, error) {
	key := client.ObjectKeyFromObject(optJob)
	if r.APIReader != nil {
		for _, name := range overdue {
			trainJob := &trainer.TrainJob{}
			err := r.APIReader.Get(ctx, types.NamespacedName{Namespace: optJob.Namespace, Name: name}, trainJob)
			if errors.IsNotFound(err) {
				r.forgetTrialCreation(key, name)
				continue
			}
			if err != nil {
				return 0, fmt.Errorf("verify pending trial %q: %w", name, err)
			}
			if owner := metav1.GetControllerOf(trainJob); owner == nil || owner.UID != optJob.UID {
				r.forgetTrialCreation(key, name)
			}
		}
	}

	r.expectationsMu.Lock()
	defer r.expectationsMu.Unlock()
	if expectations := r.pendingTrials[key]; expectations != nil && expectations.uid == optJob.UID {
		return int32(len(expectations.names)), nil
	}
	return 0, nil
}

func (r *OptimizationJobReconciler) clearTrialExpectations(key types.NamespacedName) {
	r.expectationsMu.Lock()
	defer r.expectationsMu.Unlock()
	delete(r.pendingTrials, key)
}

func SetupIndexes(ctx context.Context, indexer client.FieldIndexer) error {
	return indexer.IndexField(ctx, &trainer.TrainJob{}, jobOwnerKey, func(rawObj client.Object) []string {
		tj, ok := rawObj.(*trainer.TrainJob)
		if !ok {
			return nil
		}
		owner := metav1.GetControllerOf(tj)
		if owner == nil {
			return nil
		}
		if owner.APIVersion != trainer.GroupVersion.String() || owner.Kind != "OptimizationJob" {
			return nil
		}
		return []string{owner.Name}
	})
}

func (r *OptimizationJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	log := r.Log.WithValues("optimizationJob", req.NamespacedName)
	ctx = ctrl.LoggerInto(ctx, log)

	optJob := &trainer.OptimizationJob{}
	if getErr := r.Get(ctx, req.NamespacedName, optJob); getErr != nil {
		if errors.IsNotFound(getErr) {
			r.clearTrialExpectations(req.NamespacedName)
		}
		return ctrl.Result{}, client.IgnoreNotFound(getErr)
	}

	if optJob.Status == nil {
		optJob.Status = &trainer.OptimizationJobStatus{}
	}

	prevOptJob := optJob.DeepCopy()

	// Defer the single Status Patch operation
	defer func() {
		if !equality.Semantic.DeepEqual(optJob.Status, prevOptJob.Status) {
			patchErr := r.Status().Patch(ctx, optJob, client.MergeFrom(prevOptJob))
			if patchErr != nil {
				log.Error(patchErr, "Failed to patch OptimizationJob status")
				if err == nil {
					err = patchErr
				}
			}
		}
	}()

	// 1. Fetch Owned TrainJobs
	var allTrainJobs trainer.TrainJobList
	if listErr := r.List(ctx, &allTrainJobs, client.InNamespace(req.Namespace), client.MatchingFields{jobOwnerKey: req.Name}); listErr != nil {
		log.Error(listErr, "Unable to list TrainJobs")
		return ctrl.Result{}, listErr
	}

	var trainJobs []trainer.TrainJob
	var validTrainJobs []trainer.TrainJob
	var activeTrials, succeededTrials, failedTrials int32
	var invalidMetricTrial string
	var invalidMetricErr error
	targetMetric := ""
	if len(optJob.Spec.Objectives) > 0 {
		targetMetric = optJob.Spec.Objectives[0].Metric
	}

	for _, tj := range allTrainJobs.Items {
		if owner := metav1.GetControllerOf(&tj); owner == nil || owner.UID != optJob.UID {
			continue
		}
		trainJobs = append(trainJobs, tj)

		if trainjob.IsTrainJobFinished(&tj) {
			// Determine if it was successful or failed
			if meta.IsStatusConditionTrue(tj.Status.Conditions, trainer.TrainJobFailed) {
				failedTrials++
			} else {
				if _, _, metricErr := optimizationjob.GetFinalObjectiveMetric(&tj, targetMetric); metricErr != nil {
					failedTrials++
					if invalidMetricErr == nil {
						invalidMetricTrial = tj.Name
						invalidMetricErr = metricErr
					}
					continue
				}
				succeededTrials++
				validTrainJobs = append(validTrainJobs, tj)
			}
		} else {
			activeTrials++
		}
	}
	overdueTrials := r.observeTrialCreations(req.NamespacedName, optJob.UID, trainJobs)
	pendingTrials, verifyErr := r.verifyPendingTrialCreations(ctx, optJob, overdueTrials)
	if verifyErr != nil {
		return ctrl.Result{}, verifyErr
	}
	totalTrials := activeTrials + succeededTrials + failedTrials + pendingTrials

	// 2. Continuous Best Result Tracking
	if len(validTrainJobs) > 0 {
		if bestResult := optimizationjob.ExtractBestResult(optJob, validTrainJobs); bestResult != nil {
			optJob.Status.Result = *bestResult
		}
	}

	// 3. Ignore completed jobs and trigger cleanup
	if isJobCompleted(optJob) {
		r.clearTrialExpectations(req.NamespacedName)
		return ctrl.Result{}, r.cleanupAlgorithmService(ctx, optJob)
	}

	// 4. Check for Overall Experiment Failure (Phase 1 logic)
	if invalidMetricErr != nil {
		r.clearTrialExpectations(req.NamespacedName)
		reason := "ObjectiveMetricInvalid"
		if stderrors.Is(invalidMetricErr, optimizationjob.ErrObjectiveMetricMissing) {
			reason = "ObjectiveMetricMissing"
		}
		meta.SetStatusCondition(&optJob.Status.Conditions, metav1.Condition{
			Type:    constants.OptimizationJobFailed,
			Status:  metav1.ConditionTrue,
			Reason:  reason,
			Message: fmt.Sprintf("completed trial %q did not report a valid objective metric %q: %v", invalidMetricTrial, targetMetric, invalidMetricErr),
		})
		return ctrl.Result{}, nil
	}
	if failedTrials > 0 {
		r.clearTrialExpectations(req.NamespacedName)
		log.Info("A trial failed. Marking OptimizationJob Failed.")
		meta.SetStatusCondition(&optJob.Status.Conditions, metav1.Condition{
			Type:    constants.OptimizationJobFailed,
			Status:  metav1.ConditionTrue,
			Reason:  "TrialFailed",
			Message: fmt.Sprintf("%d trial(s) failed", failedTrials),
		})
		return ctrl.Result{}, nil
	}

	// 5. Check for Overall Experiment Completion
	if totalTrials >= optJob.Spec.NumTrials && activeTrials+pendingTrials == 0 {
		r.clearTrialExpectations(req.NamespacedName)
		log.Info("All trials finished. Marking OptimizationJob Complete.")
		meta.SetStatusCondition(&optJob.Status.Conditions, metav1.Condition{
			Type:    constants.OptimizationJobComplete,
			Status:  metav1.ConditionTrue,
			Reason:  "OptimizationJobCompleted",
			Message: "All trials have completed successfully",
		})
		return ctrl.Result{}, nil
	}

	// 6. Only "Random" is supported for Phase1.
	if optJob.Spec.SearchAlgorithm == nil || optJob.Spec.SearchAlgorithm.Random == nil {
		r.clearTrialExpectations(req.NamespacedName)
		log.Info("Unsupported search algorithm. Only 'random' is supported in phase 1.")
		meta.SetStatusCondition(&optJob.Status.Conditions, metav1.Condition{
			Type:    constants.OptimizationJobFailed,
			Status:  metav1.ConditionTrue,
			Reason:  "UnsupportedSearchAlgorithm",
			Message: "Only 'random' search algorithm is supported in phase 1",
		})
		return ctrl.Result{}, nil
	}

	// 7. Ensure the Algorithm Service is Running
	isServiceReady, srvErr := r.createAlgorithmServiceIfNecessary(ctx, optJob)
	if srvErr != nil {
		return ctrl.Result{}, srvErr
	}
	if !isServiceReady {
		log.Info("Search algorithm service is not yet ready.")
		return ctrl.Result{}, nil
	}

	if !meta.IsStatusConditionTrue(optJob.Status.Conditions, constants.OptimizationJobCreated) {
		meta.SetStatusCondition(&optJob.Status.Conditions, metav1.Condition{
			Type:    constants.OptimizationJobCreated,
			Status:  metav1.ConditionTrue,
			Reason:  "AlgorithmServiceCreated",
			Message: "Search algorithm service is running",
		})
	}

	// 8. Scale Up New Trials based on Budget Constraints
	if pendingTrials > 0 {
		// Wait until the cache observes every successful Create before requesting more trials.
		return ctrl.Result{RequeueAfter: trialCacheRequeueDelay}, nil
	}
	if activeTrials+pendingTrials >= optJob.Spec.ParallelTrials {
		// We are at maximum concurrency, exit early.
		return ctrl.Result{}, nil
	}

	trialsToSpawn := optJob.Spec.ParallelTrials - activeTrials - pendingTrials
	trialsRemaining := optJob.Spec.NumTrials - totalTrials

	if trialsToSpawn > trialsRemaining {
		trialsToSpawn = trialsRemaining
	}

	if trialsToSpawn > 0 {
		if spawnErr := r.createTrainJobs(ctx, optJob, trainJobs, trialsToSpawn); spawnErr != nil {
			return ctrl.Result{}, spawnErr
		}
		// Recheck even if no watch event arrives for a newly created TrainJob.
		return ctrl.Result{RequeueAfter: trialCacheRequeueDelay}, nil
	}

	return ctrl.Result{}, nil
}

func (r *OptimizationJobReconciler) createAlgorithmServiceIfNecessary(ctx context.Context, optJob *trainer.OptimizationJob) (bool, error) {
	log := ctrl.LoggerFrom(ctx)

	// Create/Update Deployment via SSA
	deploy, err := r.constructAlgorithmDeployment(optJob)
	if err != nil {
		return false, err
	}
	//nolint:staticcheck // client.Apply is required for typed objects
	if err := r.Patch(ctx, deploy, client.Apply, client.ForceOwnership, client.FieldOwner("optimizationjob-controller")); err != nil {
		log.Error(err, "Failed to apply algorithm Deployment")
		return false, err
	}

	// Create/Update Service via SSA
	svc, err := r.constructAlgorithmService(optJob)
	if err != nil {
		return false, err
	}
	//nolint:staticcheck // client.Apply is required for typed objects
	if err := r.Patch(ctx, svc, client.Apply, client.ForceOwnership, client.FieldOwner("optimizationjob-controller")); err != nil {
		log.Error(err, "Failed to apply algorithm Service")
		return false, err
	}

	// Fetch current state to check readiness
	deploymentName := optimizationjob.GetAlgorithmServiceName(optJob)

	var currentDeploy appsv1.Deployment
	if err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: optJob.Namespace}, &currentDeploy); err != nil {
		return false, client.IgnoreNotFound(err)
	}

	if currentDeploy.Status.AvailableReplicas == 0 {
		return false, nil
	}

	return true, nil
}

func (r *OptimizationJobReconciler) cleanupAlgorithmService(ctx context.Context, optJob *trainer.OptimizationJob) error {
	log := ctrl.LoggerFrom(ctx)
	deploymentName := optimizationjob.GetAlgorithmServiceName(optJob)

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: optJob.Namespace},
	}
	if err := client.IgnoreNotFound(r.Delete(ctx, deploy)); err != nil {
		log.Error(err, "Failed to cleanup Algorithm Deployment")
		return err
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: optJob.Namespace},
	}
	if err := client.IgnoreNotFound(r.Delete(ctx, svc)); err != nil {
		log.Error(err, "Failed to cleanup Algorithm Service")
		return err
	}
	return nil
}

func (r *OptimizationJobReconciler) createTrainJobs(ctx context.Context, optJob *trainer.OptimizationJob, trainJobs []trainer.TrainJob, trialsToSpawn int32) error {
	log := ctrl.LoggerFrom(ctx)

	serviceName := optimizationjob.GetAlgorithmServiceName(optJob)
	serviceAddr := fmt.Sprintf("%s.%s.svc:%d", serviceName, optJob.Namespace, constants.SearchAlgorithmServicePort)

	// Pass trial history to Optuna. The service can record completed trials only
	// while it retains the assignments issued by this same service instance.
	suggestionReq, err := optimizationjob.BuildSuggestionRequest(optJob, trainJobs, trialsToSpawn)
	if err != nil {
		log.Error(err, "Failed to build suggestion request")
		return err
	}

	suggestions, err := r.SearchAlgorithmClient.GetSuggestions(ctx, serviceAddr, suggestionReq)
	if err != nil {
		log.Error(err, "Failed to fetch suggestions from gRPC service")
		// Exponential backoff for transient gRPC errors instead of terminating the job.
		return err
	}
	if len(suggestions) < int(trialsToSpawn) {
		return fmt.Errorf("search algorithm returned %d suggestions, requested %d", len(suggestions), trialsToSpawn)
	}

	// TODO: For scale (ParallelTrials > 1000), implement a WorkQueue pattern to spawn TrainJobs.
	var errMsgs []string
	for i, paramAssignments := range suggestions {
		if int32(i) >= trialsToSpawn {
			break
		}
		newTrainJob, err := r.constructTrainJob(optJob, paramAssignments)
		if err != nil {
			msg := fmt.Sprintf("failed to build TrainJob: %v", err)
			log.Error(err, msg)
			errMsgs = append(errMsgs, msg)
			continue
		}
		r.expectTrialCreation(client.ObjectKeyFromObject(optJob), optJob.UID, newTrainJob.Name)
		if err := r.Create(ctx, newTrainJob); err != nil {
			if errors.IsAlreadyExists(err) {
				r.forgetTrialCreation(client.ObjectKeyFromObject(optJob), newTrainJob.Name)
				continue
			}
			// An API timeout can occur after the Create was committed. Keep the
			// expectation unless a live read confirms this trial does not exist.
			if r.APIReader != nil {
				current := &trainer.TrainJob{}
				getErr := r.APIReader.Get(ctx, client.ObjectKeyFromObject(newTrainJob), current)
				if errors.IsNotFound(getErr) {
					r.forgetTrialCreation(client.ObjectKeyFromObject(optJob), newTrainJob.Name)
				} else if getErr == nil {
					if owner := metav1.GetControllerOf(current); owner == nil || owner.UID != optJob.UID {
						r.forgetTrialCreation(client.ObjectKeyFromObject(optJob), newTrainJob.Name)
					}
				}
			}
			msg := fmt.Sprintf("Failed to create TrainJob trial: %v", err)
			log.Error(err, msg)
			r.Recorder.Eventf(optJob, nil, corev1.EventTypeWarning, "TrainJobResourcesCreationFailed", "Creating", "%s", msg)
			errMsgs = append(errMsgs, msg)
		}
	}

	if len(errMsgs) > 0 {
		return fmt.Errorf("errors spawning trials:\n - %s", strings.Join(errMsgs, "\n - "))
	}

	return nil
}

func (r *OptimizationJobReconciler) constructTrainJob(optJob *trainer.OptimizationJob, params []trainer.ParameterAssignment) (*trainer.TrainJob, error) {
	labels := maps.Clone(optJob.Spec.TrainJobTemplate.Labels)
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[constants.OptimizationJobNameLabel] = optJob.Name

	tj := &trainer.TrainJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:        optimizationjob.GenerateTrialName(optJob.Name),
			Namespace:   optJob.Namespace,
			Labels:      labels,
			Annotations: maps.Clone(optJob.Spec.TrainJobTemplate.Annotations),
		},
		Spec: *optJob.Spec.TrainJobTemplate.Spec.DeepCopy(),
	}
	if tj.Spec.Trainer == nil {
		tj.Spec.Trainer = &trainer.Trainer{}
	}

	for _, param := range params {
		envName := fmt.Sprintf("%s%s", constants.EnvVarPrefix, param.Name)
		tj.Spec.Trainer.Env = append(tj.Spec.Trainer.Env, corev1.EnvVar{
			Name:  envName,
			Value: param.Value,
		})
	}

	if err := controllerutil.SetControllerReference(optJob, tj, r.Scheme); err != nil {
		r.Log.Error(err, "Failed to set controller reference")
		return nil, err
	}

	return tj, nil
}

func (r *OptimizationJobReconciler) SetupWithManager(mgr ctrl.Manager, options ctrlpkg.Options) error {
	if err := SetupIndexes(context.Background(), mgr.GetFieldIndexer()); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		Named("optimizationjob-controller").
		WithOptions(options).
		For(&trainer.OptimizationJob{}).
		Owns(&trainer.TrainJob{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

func isJobCompleted(optJob *trainer.OptimizationJob) bool {
	if optJob == nil || optJob.Status == nil || len(optJob.Status.Conditions) == 0 {
		return false
	}
	return meta.IsStatusConditionTrue(optJob.Status.Conditions, constants.OptimizationJobComplete) ||
		meta.IsStatusConditionTrue(optJob.Status.Conditions, constants.OptimizationJobFailed)
}

func (r *OptimizationJobReconciler) constructAlgorithmDeployment(optJob *trainer.OptimizationJob) (*appsv1.Deployment, error) {
	name := optimizationjob.GetAlgorithmServiceName(optJob)
	labels := map[string]string{
		constants.OptimizationJobNameLabel:      optJob.Name,
		constants.OptimizationJobComponentLabel: constants.SearchAlgorithmComponent,
	}

	// TODO: Allow users to configure the Optuna suggestion deployment (e.g., resources, tolerations, affinity).
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: optJob.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "search-algorithm",
							Image: constants.DefaultSearchAlgorithmImage,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: constants.SearchAlgorithmServicePort,
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									GRPC: &corev1.GRPCAction{
										Port:    constants.SearchAlgorithmServicePort,
										Service: ptr.To(constants.SearchAlgorithmServiceName),
									},
								},
								InitialDelaySeconds: 2,
								PeriodSeconds:       5,
							},
						},
					},
				},
			},
		},
	}
	deploy.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))

	if err := controllerutil.SetControllerReference(optJob, deploy, r.Scheme); err != nil {
		r.Log.Error(err, "Failed to set controller reference for Algorithm Deployment")
		return nil, err
	}

	return deploy, nil
}

func (r *OptimizationJobReconciler) constructAlgorithmService(optJob *trainer.OptimizationJob) (*corev1.Service, error) {
	name := optimizationjob.GetAlgorithmServiceName(optJob)
	labels := map[string]string{
		constants.OptimizationJobNameLabel:      optJob.Name,
		constants.OptimizationJobComponentLabel: constants.SearchAlgorithmComponent,
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: optJob.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:     "grpc",
					Port:     constants.SearchAlgorithmServicePort,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}
	svc.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Service"))

	if err := controllerutil.SetControllerReference(optJob, svc, r.Scheme); err != nil {
		r.Log.Error(err, "Failed to set controller reference for Algorithm Service")
		return nil, err
	}

	return svc, nil
}

type DefaultSearchAlgorithmClient struct{}

func (c *DefaultSearchAlgorithmClient) GetSuggestions(ctx context.Context, addr string, req *katibapi.GetSuggestionsRequest) ([][]trainer.ParameterAssignment, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	client := katibapi.NewSuggestionClient(conn)
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	reply, err := client.GetSuggestions(callCtx, req)
	if err != nil {
		return nil, err
	}

	var allAssignments [][]trainer.ParameterAssignment
	for _, pAssignments := range reply.ParameterAssignments {
		var assignments []trainer.ParameterAssignment
		for _, assignment := range pAssignments.Assignments {
			assignments = append(assignments, trainer.ParameterAssignment{
				Name:  assignment.Name,
				Value: assignment.Value,
			})
		}
		allAssignments = append(allAssignments, assignments)
	}

	return allAssignments, nil
}
