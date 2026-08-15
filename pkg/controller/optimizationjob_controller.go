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
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlpkg "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	katibapi "github.com/kubeflow/katib/pkg/apis/manager/v1beta1"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/constants"
)

const (
	// TrainJob condition types
	TrainJobComplete string = "Complete"
	TrainJobFailed   string = "Failed"
)

const (
	// OptunaGRPCServicePort is the default port exposed by the Katib suggestion-optuna image
	OptunaGRPCServicePort int32 = 6789
)

// SuggestionProvider abstracts the gRPC call so we can mock it in unit tests
type SuggestionProvider interface {
	GetSuggestions(ctx context.Context, addr string, req *katibapi.GetSuggestionsRequest) ([][]trainer.ParameterAssignment, error)
}

// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=optimizationjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=optimizationjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=optimizationjobs/finalizers,verbs=update
// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=trainjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

type OptimizationJobReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	SuggestionClient SuggestionProvider
}

func NewOptimizationJobReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	suggestionClient SuggestionProvider,
) *OptimizationJobReconciler {
	// Fall back to the real client for production if nil is passed
	if suggestionClient == nil {
		suggestionClient = &RealSuggestionClient{}
	}

	return &OptimizationJobReconciler{
		Client:           client,
		Scheme:           scheme,
		SuggestionClient: suggestionClient,
	}
}

func (r *OptimizationJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx).WithName("optimizationjob-controller")

	// 1. Fetch the OptimizationJob instance
	optJob := &trainer.OptimizationJob{}
	if err := r.Get(ctx, req.NamespacedName, optJob); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil // Job was deleted
		}
		return ctrl.Result{}, err
	}

	if optJob.Status == nil {
		optJob.Status = &trainer.OptimizationJobStatus{}
	}

	// 2. Ignore completed jobs
	if isJobCompleted(optJob) {
		return ctrl.Result{}, nil
	}

	// ========================================================================
	// 3. Ensure the Optuna Suggestion Service is Running
	// ========================================================================

	// 3a. Ensure Deployment exists
	deploymentName := fmt.Sprintf("%s-optuna", optJob.Name)
	var deploy appsv1.Deployment
	if err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: req.Namespace}, &deploy); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating Optuna Suggestion Deployment", "Deployment.Name", deploymentName)
			newDeploy := r.constructOptunaDeployment(optJob, deploymentName)
			if err := r.Create(ctx, newDeploy); err != nil {
				meta.SetStatusCondition(&optJob.Status.Conditions, metav1.Condition{
					Type:    "Created",
					Status:  metav1.ConditionFalse,
					Reason:  "AlgorithmServiceDeploymentCreationFailed",
					Message: fmt.Sprintf("Failed to create Optuna deployment: %v", err),
				})
				if updateErr := r.Status().Update(ctx, optJob); updateErr != nil {
					log.Error(updateErr, "Failed to update OptimizationJob status")
				}
				return ctrl.Result{}, err
			}
			// Requeue immediately after creation to let the status update
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	// Check for Running state of Optuna pod deployment before moving ahead.

	// 3b. Ensure Service exists
	var svc corev1.Service
	if err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: req.Namespace}, &svc); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating Optuna Suggestion Service", "Service.Name", deploymentName)
			newSvc := r.constructOptunaService(optJob, deploymentName)
			if err := r.Create(ctx, newSvc); err != nil {
				meta.SetStatusCondition(&optJob.Status.Conditions, metav1.Condition{
					Type:    "Created",
					Status:  metav1.ConditionFalse,
					Reason:  "AlgorithmServiceCreationFailed",
					Message: fmt.Sprintf("Failed to create Optuna service: %v", err),
				})
				if updateErr := r.Status().Update(ctx, optJob); updateErr != nil {
					log.Error(updateErr, "Failed to update OptimizationJob status")
				}
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	// Check if the Pod backing the deployment is running and ready.
	if deploy.Status.AvailableReplicas == 0 {
		log.Info("Optuna suggestion pod is not yet ready, waiting...")
		// The deployment exists, but the pod isn't ready. Requeue after 3 seconds.
		return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
	}

	// 3c. Mark Created = True once Optuna service resources are provisioned AND running
	if !meta.IsStatusConditionTrue(optJob.Status.Conditions, "Created") {
		meta.SetStatusCondition(&optJob.Status.Conditions, metav1.Condition{
			Type:    "Created",
			Status:  metav1.ConditionTrue,
			Reason:  "AlgorithmServiceCreated",
			Message: "Optuna suggestion service is running",
		})
		if err := r.Status().Update(ctx, optJob); err != nil {
			return ctrl.Result{}, err
		}
	}

	// ========================================================================
	// 4. Analyze Current Trial (TrainJob) States
	// ========================================================================

	var childTrainJobs trainer.TrainJobList
	if err := r.List(ctx, &childTrainJobs, client.InNamespace(req.Namespace), client.MatchingLabels{
		"trainer.kubeflow.org/optimizationjob-name": optJob.Name,
	}); err != nil {
		log.Error(err, "unable to list child TrainJobs")
		return ctrl.Result{}, err
	}

	var activeTrials, succeededTrials, failedTrials int32
	var validTrainJobs []trainer.TrainJob

	for _, tj := range childTrainJobs.Items {
		isOwner := false
		for _, ownerRef := range tj.GetOwnerReferences() {
			if ownerRef.UID == optJob.UID {
				isOwner = true
				break
			}
		}
		if !isOwner {
			continue
		}

		validTrainJobs = append(validTrainJobs, tj)

		if isTrainJobSucceeded(&tj) {
			succeededTrials++
		} else if isTrainJobFailed(&tj) {
			failedTrials++
		} else {
			activeTrials++
		}
	}
	totalTrials := activeTrials + succeededTrials + failedTrials

	log.V(5).Info("Trial stats", "total", totalTrials, "active", activeTrials, "succeeded", succeededTrials, "failed", failedTrials)

	// ========================================================================
	// 5. Check for Overall Experiment Completion
	// ========================================================================

	if optJob.Spec.NumTrials > 0 && totalTrials >= optJob.Spec.NumTrials && activeTrials == 0 {
		if failedTrials == totalTrials {
			log.Info("All trials failed. Marking OptimizationJob Failed.")
			meta.SetStatusCondition(&optJob.Status.Conditions, metav1.Condition{
				Type:    constants.OptimizationJobFailed,
				Status:  metav1.ConditionTrue,
				Reason:  "AllTrialsFailed",
				Message: fmt.Sprintf("%d out of %d trials failed", failedTrials, totalTrials),
			})
		} else {
			log.Info("All trials finished. Marking OptimizationJob Complete.")
			bestResult := r.extractBestResult(optJob, validTrainJobs)
			if bestResult != nil {
				optJob.Status.Result = *bestResult
			}
			markJobCompleted(optJob)
		}

		if err := r.Status().Update(ctx, optJob); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// ========================================================================
	// 6. Scale Up New Trials based on Budget Constraints (via Optuna gRPC)
	// ========================================================================

	var parallelLimit int32 = 1
	if optJob.Spec.ParallelTrials > 0 {
		parallelLimit = optJob.Spec.ParallelTrials
	}

	var totalLimit int32 = 1
	if optJob.Spec.NumTrials > 0 {
		totalLimit = optJob.Spec.NumTrials
	}

	trialsToSpawn := parallelLimit - activeTrials
	trialsRemaining := totalLimit - totalTrials

	// Prevent overshooting the total budget
	if trialsToSpawn > trialsRemaining {
		trialsToSpawn = trialsRemaining
	}

	if trialsToSpawn > 0 {
		suggestionReq := r.buildSuggestionRequest(optJob, childTrainJobs.Items, trialsToSpawn)
		serviceAddr := fmt.Sprintf("%s-optuna.%s.svc:%d", optJob.Name, optJob.Namespace, OptunaGRPCServicePort)

		suggestions, err := r.SuggestionClient.GetSuggestions(ctx, serviceAddr, suggestionReq)
		if err != nil {
			log.Error(err, "Failed to fetch suggestions from Optuna gRPC service")

			// Mark OptimizationJob as Failed (Terminal Condition)
			meta.SetStatusCondition(&optJob.Status.Conditions, metav1.Condition{
				Type:    constants.OptimizationJobFailed,
				Status:  metav1.ConditionTrue,
				Reason:  "SuggestionServiceFailed",
				Message: fmt.Sprintf("Optuna gRPC service error: %v", err),
			})
			if updateErr := r.Status().Update(ctx, optJob); updateErr != nil {
				return ctrl.Result{}, updateErr
			}
			return ctrl.Result{}, nil // Do not requeue terminal failure
		}

		// Spawn a TrainJob trial for each suggestion
		for i, paramAssignments := range suggestions {
			trialIndex := totalTrials + int32(i)
			newTrainJob := r.constructTrainJob(optJob, paramAssignments, trialIndex)

			log.Info("Creating a new TrainJob trial with Optuna suggestions", "TrainJob.Name", newTrainJob.Name)
			if err := r.Create(ctx, newTrainJob); err != nil {
				if errors.IsAlreadyExists(err) {
					log.Info("TrainJob already exists (cache race detected), skipping", "TrainJob.Name", newTrainJob.Name)
					continue
				}

				log.Error(err, "Failed to create new TrainJob", "TrainJob.Name", newTrainJob.Name)

				meta.SetStatusCondition(&optJob.Status.Conditions, metav1.Condition{
					Type:    "Created",
					Status:  metav1.ConditionFalse,
					Reason:  "TrainJobsCreationFailed", // Matches KEP Section 7.4
					Message: fmt.Sprintf("Failed to create TrainJob %s: %v", newTrainJob.Name, err),
				})
				if updateErr := r.Status().Update(ctx, optJob); updateErr != nil {
					log.Error(updateErr, "Failed to update OptimizationJob status")
				}
				return ctrl.Result{}, err
			}
		}
	}

	// ========================================================================
	// 7. Sync OptimizationJob Status (Active counts, etc.)
	// ========================================================================

	if activeTrials > 0 && !meta.IsStatusConditionTrue(optJob.Status.Conditions, "Running") {
		meta.SetStatusCondition(&optJob.Status.Conditions, metav1.Condition{
			Type:    "Running",
			Status:  metav1.ConditionTrue,
			Reason:  "TrialsRunning",
			Message: fmt.Sprintf("%d trial(s) actively running", activeTrials),
		})
		if err := r.Status().Update(ctx, optJob); err != nil {
			log.Error(err, "Failed to update Running condition")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// constructTrainJob merges the TrainJobTemplate with the suggested parameters
func (r *OptimizationJobReconciler) constructTrainJob(optJob *trainer.OptimizationJob, params []trainer.ParameterAssignment, index int32) *trainer.TrainJob {
	tj := &trainer.TrainJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-trial-%d", optJob.Name, index),
			Namespace: optJob.Namespace,
			Labels: map[string]string{
				"trainer.kubeflow.org/optimizationjob-name": optJob.Name,
			},
			Annotations: make(map[string]string),
		},
		Spec: *optJob.Spec.TrainJobTemplate.Spec.DeepCopy(),
	}

	// Copy original annotations from template
	for k, v := range optJob.Spec.TrainJobTemplate.Annotations {
		tj.Annotations[k] = v
	}

	// Ensure Trainer exists to prevent nil pointer panics when injecting envs
	if tj.Spec.Trainer == nil {
		tj.Spec.Trainer = &trainer.Trainer{}
	}

	// Inject parameters as Annotations and Environment Variables
	for _, param := range params {
		// Annotation for stateless history tracking (Optimization controller reads this)
		tj.Annotations[fmt.Sprintf("trainer.kubeflow.org/opt-param-%s", param.Name)] = param.Value

		// Env Var for SDK consumption (Training script reads this)
		envName := fmt.Sprintf("KUBEFLOW_TRAINER_OPT_%s", strings.ToUpper(param.Name))
		tj.Spec.Trainer.Env = append(tj.Spec.Trainer.Env, corev1.EnvVar{
			Name:  envName,
			Value: param.Value,
		})
	}

	// Set OptimizationJob as the owner
	if err := controllerutil.SetControllerReference(optJob, tj, r.Scheme); err != nil {
		klog.Error(err, "Failed to set controller reference")
	}

	return tj
}

// SetupWithManager registers the controller
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

// --- Helper Functions ---

func isJobCompleted(optJob *trainer.OptimizationJob) bool {
	if optJob == nil || optJob.Status == nil || len(optJob.Status.Conditions) == 0 {
		return false
	}
	return meta.IsStatusConditionTrue(optJob.Status.Conditions, constants.OptimizationJobComplete) ||
		meta.IsStatusConditionTrue(optJob.Status.Conditions, constants.OptimizationJobFailed)
}

func isTrainJobSucceeded(tj *trainer.TrainJob) bool {
	if tj == nil || len(tj.Status.Conditions) == 0 {
		return false
	}
	return meta.IsStatusConditionTrue(tj.Status.Conditions, TrainJobComplete)
}

func isTrainJobFailed(tj *trainer.TrainJob) bool {
	if tj == nil || len(tj.Status.Conditions) == 0 {
		return false
	}
	return meta.IsStatusConditionTrue(tj.Status.Conditions, TrainJobFailed)
}

func markJobCompleted(optJob *trainer.OptimizationJob) {
	meta.SetStatusCondition(&optJob.Status.Conditions, metav1.Condition{
		Type:    constants.OptimizationJobComplete,
		Status:  metav1.ConditionTrue,
		Reason:  "OptimizationJobCompleted",
		Message: "All trials have completed successfully",
	})
}

func (r *OptimizationJobReconciler) constructOptunaDeployment(optJob *trainer.OptimizationJob, name string) *appsv1.Deployment {
	labels := map[string]string{
		"app": name,
		"trainer.kubeflow.org/optimizationjob-name": optJob.Name,
	}

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
							Name: "optuna-suggestion",
							// Using the existing Katib Optuna image for phase 1
							Image: "docker.io/kubeflowkatib/suggestion-optuna:latest",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: OptunaGRPCServicePort,
								},
							},
						},
					},
				},
			},
		},
	}

	// Make the OptimizationJob the owner so this deployment is garbage collected when the job is deleted
	if err := controllerutil.SetControllerReference(optJob, deploy, r.Scheme); err != nil {
		klog.Error(err, "Failed to set controller reference for Optuna Deployment")
	}

	return deploy
}

func (r *OptimizationJobReconciler) constructOptunaService(optJob *trainer.OptimizationJob, name string) *corev1.Service {
	labels := map[string]string{
		"app": name,
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
					Port:     OptunaGRPCServicePort,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}

	// Make the OptimizationJob the owner
	if err := controllerutil.SetControllerReference(optJob, svc, r.Scheme); err != nil {
		klog.Error(err, "Failed to set controller reference for Optuna Service")
	}

	return svc
}

func (r *OptimizationJobReconciler) buildSuggestionRequest(optJob *trainer.OptimizationJob, trainJobs []trainer.TrainJob, trialsToSpawn int32) *katibapi.GetSuggestionsRequest {

	// 1. Map Objective Direction & Metric
	var targetMetric string
	var optunaObjType katibapi.ObjectiveType
	if len(optJob.Spec.Objectives) > 0 && optJob.Spec.Objectives[0].Metric != "" {
		targetMetric = optJob.Spec.Objectives[0].Metric
		if optJob.Spec.Objectives[0].Direction == trainer.ObjectiveDirectionMaximize {
			optunaObjType = katibapi.ObjectiveType_MAXIMIZE
		} else {
			optunaObjType = katibapi.ObjectiveType_MINIMIZE
		}
	}

	// 2. Map Search Space Parameters
	var grpcParams []*katibapi.ParameterSpec
	for _, p := range optJob.Spec.Parameters {
		if p.SearchSpace == nil {
			continue
		}

		paramType := katibapi.ParameterType_DOUBLE

		if p.SearchSpace.Uniform.Type == trainer.ParameterTypeInt ||
			p.SearchSpace.LogUniform.Type == trainer.ParameterTypeInt {
			paramType = katibapi.ParameterType_INT
		}

		paramSpec := &katibapi.ParameterSpec{
			Name:          p.Name,
			ParameterType: paramType,
		}

		if p.SearchSpace.Uniform.Min != "" || p.SearchSpace.Uniform.Max != "" {

			paramSpec.FeasibleSpace = &katibapi.FeasibleSpace{
				Min: string(p.SearchSpace.Uniform.Min),
				Max: string(p.SearchSpace.Uniform.Max),
			}

		} else if p.SearchSpace.LogUniform.Min != "" || p.SearchSpace.LogUniform.Max != "" {

			paramSpec.FeasibleSpace = &katibapi.FeasibleSpace{
				Min: string(p.SearchSpace.LogUniform.Min),
				Max: string(p.SearchSpace.LogUniform.Max),
			}

		} else if len(p.SearchSpace.Categorical.Choices) > 0 {

			paramSpec.ParameterType = katibapi.ParameterType_CATEGORICAL
			paramSpec.FeasibleSpace = &katibapi.FeasibleSpace{
				List: p.SearchSpace.Categorical.Choices,
			}
		}

		grpcParams = append(grpcParams, paramSpec)
	}

	req := &katibapi.GetSuggestionsRequest{
		Experiment: &katibapi.Experiment{
			Name: optJob.Name,
			Spec: &katibapi.ExperimentSpec{
				Algorithm: &katibapi.AlgorithmSpec{
					AlgorithmName: getAlgorithmName(optJob),
				},
				Objective: &katibapi.ObjectiveSpec{
					Type:                optunaObjType,
					ObjectiveMetricName: targetMetric,
				},
				ParameterSpecs: &katibapi.ExperimentSpec_ParameterSpecs{
					Parameters: grpcParams,
				},
			},
		},
		CurrentRequestNumber: trialsToSpawn,
		TotalRequestNumber:   0,
	}

	// Iterate through all TrainJobs owned by this OptimizationJob
	for _, tj := range trainJobs {
		req.TotalRequestNumber++

		trial := &katibapi.Trial{
			Name: tj.Name,
			Spec: &katibapi.TrialSpec{
				ParameterAssignments: &katibapi.TrialSpec_ParameterAssignments{
					Assignments: []*katibapi.ParameterAssignment{},
				},
			},
		}

		// 1. Reconstruct ParameterAssignments from the stateless Annotations
		prefix := "trainer.kubeflow.org/opt-param-"
		for k, v := range tj.Annotations {
			if strings.HasPrefix(k, prefix) {
				paramName := strings.TrimPrefix(k, prefix)
				trial.Spec.ParameterAssignments.Assignments = append(
					trial.Spec.ParameterAssignments.Assignments,
					&katibapi.ParameterAssignment{
						Name:  paramName,
						Value: v,
					},
				)
			}
		}

		// 2. Reconstruct Trial metrics from TrainJobStatus
		if tj.Status.TrainerStatus != nil && len(tj.Status.TrainerStatus.Metrics) > 0 {
			for _, m := range tj.Status.TrainerStatus.Metrics {
				if m.Name == targetMetric {
					trial.Status = &katibapi.TrialStatus{
						Observation: &katibapi.Observation{
							Metrics: []*katibapi.Metric{
								{
									Name:  m.Name,
									Value: m.Value,
								},
							},
						},
					}
					break // Found the target objective metric
				}
			}
		}

		req.Trials = append(req.Trials, trial)
	}

	return req
}

// RealSuggestionClient implements SuggestionProvider to connect to a real Optuna pod
type RealSuggestionClient struct{}

// GetSuggestions establishes a gRPC connection to the Optuna pod and fetches parameter assignments
func (c *RealSuggestionClient) GetSuggestions(ctx context.Context, addr string, req *katibapi.GetSuggestionsRequest) ([][]trainer.ParameterAssignment, error) {
	// Establish insecure gRPC connection to the Optuna Service inside the cluster
	conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock(), grpc.WithTimeout(time.Second*3))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := katibapi.NewSuggestionClient(conn)

	// Call the gRPC method
	reply, err := client.GetSuggestions(ctx, req)
	if err != nil {
		return nil, err
	}

	var allAssignments [][]trainer.ParameterAssignment

	// Map Katib's protobuf reply back to our native v1alpha1 ParameterAssignment structs
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

// Helper function to map the new API algorithm block to the legacy Katib string name
func getAlgorithmName(optJob *trainer.OptimizationJob) string {
	if optJob.Spec.SearchAlgorithm != nil {
		if optJob.Spec.SearchAlgorithm.Random != nil {
			return "random"
		}
		if optJob.Spec.SearchAlgorithm.Grid != nil {
			return "grid"
		}
		// For other Optuna specific algorithms (like TPE), to be added later.
	}
	// Default fallback
	return "random"
}

func (r *OptimizationJobReconciler) extractBestResult(optJob *trainer.OptimizationJob, trainJobs []trainer.TrainJob) *trainer.Result {
	if len(optJob.Spec.Objectives) == 0 || optJob.Spec.Objectives[0].Metric == "" {
		return nil
	}

	targetMetric := optJob.Spec.Objectives[0].Metric
	isMaximize := optJob.Spec.Objectives[0].Direction == trainer.ObjectiveDirectionMaximize

	var bestJob *trainer.TrainJob
	var bestVal float64

	for i, tj := range trainJobs {
		if tj.Status.TrainerStatus == nil {
			continue
		}
		for _, m := range tj.Status.TrainerStatus.Metrics {
			if m.Name == targetMetric {
				var val float64
				if _, err := fmt.Sscanf(m.Value, "%f", &val); err != nil {
					continue
				}
				if bestJob == nil {
					bestJob = &trainJobs[i]
					bestVal = val
				} else if isMaximize && val > bestVal {
					bestJob = &trainJobs[i]
					bestVal = val
				} else if !isMaximize && val < bestVal {
					bestJob = &trainJobs[i]
					bestVal = val
				}
			}
		}
	}

	if bestJob == nil {
		return nil
	}

	res := &trainer.Result{
		TrainJobName: bestJob.Name,
	}

	prefix := "trainer.kubeflow.org/opt-param-"
	for k, v := range bestJob.Annotations {
		if strings.HasPrefix(k, prefix) {
			res.Parameters = append(res.Parameters, trainer.ParameterAssignment{
				Name:  strings.TrimPrefix(k, prefix),
				Value: v,
			})
		}
	}

	return res
}
