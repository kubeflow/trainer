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
	"fmt"

	"github.com/go-logr/logr"
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
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlpkg "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	katibapi "github.com/kubeflow/katib/pkg/apis/manager/v1beta1"
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
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

type OptimizationJobReconciler struct {
	client.Client
	Scheme                *runtime.Scheme
	Log                   logr.Logger
	Recorder              record.EventRecorder
	SearchAlgorithmClient SearchAlgorithmClient
}

func NewOptimizationJobReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	recorder record.EventRecorder,
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

func (r *OptimizationJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("optimizationJob", req.NamespacedName)
	ctx = ctrl.LoggerInto(ctx, log)

	optJob := &trainer.OptimizationJob{}
	if err := r.Get(ctx, req.NamespacedName, optJob); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
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
			}
		}
	}()

	// 1. Fetch Owned TrainJobs
	var allTrainJobs trainer.TrainJobList
	if listErr := r.List(ctx, &allTrainJobs, client.InNamespace(req.Namespace)); listErr != nil {
		log.Error(listErr, "Unable to list TrainJobs")
		return ctrl.Result{}, listErr
	}

	var validTrainJobs []trainer.TrainJob
	var activeTrials, succeededTrials, failedTrials int32

	for _, tj := range allTrainJobs.Items {
		if owner := metav1.GetControllerOf(&tj); owner == nil || owner.UID != optJob.UID {
			continue
		}
		validTrainJobs = append(validTrainJobs, tj)

		if trainjob.IsTrainJobFinished(&tj) {
			// Determine if it was successful or failed
			if meta.IsStatusConditionTrue(tj.Status.Conditions, trainer.TrainJobFailed) {
				failedTrials++
			} else {
				succeededTrials++
			}
		} else {
			activeTrials++
		}
	}
	totalTrials := activeTrials + succeededTrials + failedTrials

	// 2. Continuous Best Result Tracking
	if bestResult := optimizationjob.ExtractBestResult(optJob, validTrainJobs); bestResult != nil {
		optJob.Status.Result = *bestResult
	}

	// 3. Ignore completed jobs and trigger cleanup
	if isJobCompleted(optJob) {
		return ctrl.Result{}, r.cleanupAlgorithmService(ctx, optJob)
	}

	// 4. Check for Overall Experiment Failure (Phase 1 logic)
	if failedTrials > 0 {
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
	if totalTrials >= optJob.Spec.NumTrials && activeTrials == 0 {
		log.Info("All trials finished. Marking OptimizationJob Complete.")
		meta.SetStatusCondition(&optJob.Status.Conditions, metav1.Condition{
			Type:    constants.OptimizationJobComplete,
			Status:  metav1.ConditionTrue,
			Reason:  "OptimizationJobCompleted",
			Message: "All trials have completed successfully",
		})
		return ctrl.Result{}, nil
	}

	// 6. Ensure the Algorithm Service is Running
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

	// 7. Scale Up New Trials based on Budget Constraints
	if activeTrials >= optJob.Spec.ParallelTrials {
		// We are at maximum concurrency, exit early.
		return ctrl.Result{}, nil
	}

	trialsToSpawn := optJob.Spec.ParallelTrials - activeTrials
	trialsRemaining := optJob.Spec.NumTrials - totalTrials

	if trialsToSpawn > trialsRemaining {
		trialsToSpawn = trialsRemaining
	}

	// TODO: Implement cache Expectations to prevent over-spawning trials due to cache sync delays.
	if trialsToSpawn > 0 {
		if spawnErr := r.createTrainJobs(ctx, optJob, validTrainJobs, trialsToSpawn); spawnErr != nil {
			return ctrl.Result{}, spawnErr
		}
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
	if err := r.Patch(ctx, deploy, client.Apply, client.ForceOwnership, client.FieldOwner("optimizationjob-controller")); err != nil {
		log.Error(err, "Failed to apply algorithm Deployment")
		return false, err
	}

	// Create/Update Service via SSA
	svc, err := r.constructAlgorithmService(optJob)
	if err != nil {
		return false, err
	}
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

	// Pass both completed and in-flight trials to rebuild the stateless Optuna study
	suggestionReq := optimizationjob.BuildSuggestionRequest(optJob, trainJobs, trialsToSpawn)

	suggestions, err := r.SearchAlgorithmClient.GetSuggestions(ctx, serviceAddr, suggestionReq)
	if err != nil {
		log.Error(err, "Failed to fetch suggestions from gRPC service")
		// Exponential backoff for transient gRPC errors instead of terminating the job.
		return err
	}

	// TODO: For extreme scale (ParallelTrials > 100), implement a WorkQueue pattern to spawn TrainJobs.
	for _, paramAssignments := range suggestions {
		newTrainJob, err := r.constructTrainJob(optJob, paramAssignments)
		if err != nil {
			return err
		}
		if err := r.Create(ctx, newTrainJob); err != nil {
			if errors.IsAlreadyExists(err) {
				continue
			}
			msg := fmt.Sprintf("Failed to create TrainJob trial: %v", err)
			log.Error(err, msg)
			r.Recorder.Eventf(optJob, corev1.EventTypeWarning, "TrainJobResourcesCreationFailed", msg)
			return err
		}
	}
	return nil
}

func (r *OptimizationJobReconciler) constructTrainJob(optJob *trainer.OptimizationJob, params []trainer.ParameterAssignment) (*trainer.TrainJob, error) {
	tj := &trainer.TrainJob{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-trial-", optJob.Name),
			Namespace:    optJob.Namespace,
			Annotations:  make(map[string]string),
			Labels: map[string]string{
				constants.OptimizationJobNameLabel: optJob.Name,
			},
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
		constants.OptimizationJobNameLabel: optJob.Name,
	}

	// TODO: Allow users to configure the Optuna suggestion deployment (e.g., resources, tolerations, affinity).
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: optJob.Namespace,
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
										Port: constants.SearchAlgorithmServicePort,
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

	if err := controllerutil.SetControllerReference(optJob, deploy, r.Scheme); err != nil {
		r.Log.Error(err, "Failed to set controller reference for Algorithm Deployment")
		return nil, err
	}

	return deploy, nil
}

func (r *OptimizationJobReconciler) constructAlgorithmService(optJob *trainer.OptimizationJob) (*corev1.Service, error) {
	name := optimizationjob.GetAlgorithmServiceName(optJob)
	labels := map[string]string{
		constants.OptimizationJobNameLabel: optJob.Name,
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: optJob.Namespace,
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
	defer conn.Close()

	client := katibapi.NewSuggestionClient(conn)

	reply, err := client.GetSuggestions(ctx, req)
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
