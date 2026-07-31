/*
Copyright 2026 The Kubeflow Authors.
...
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
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	katibapi "github.com/kubeflow/katib/pkg/apis/manager/v1beta1"
	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

const (
	// OptimizationJob condition types
	OptimizationJobComplete string = "Complete"
	OptimizationJobFailed   string = "Failed"

	// TrainJob condition types (Assumed based on standard Kubeflow Trainer v2 patterns)
	TrainJobComplete string = "Complete"
	TrainJobFailed   string = "Failed"
)

// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=optimizationjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=optimizationjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=trainer.kubeflow.org,resources=trainjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// SuggestionProvider abstracts the gRPC call so we can mock it in unit tests
type SuggestionProvider interface {
	GetSuggestions(ctx context.Context, addr string, req *katibapi.GetSuggestionsRequest) ([][]trainer.ParameterAssignment, error)
}

// OptimizationJobReconciler reconciles a OptimizationJob object
type OptimizationJobReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	SuggestionClient SuggestionProvider
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
				return ctrl.Result{}, err
			}
			// Requeue immediately to wait for the Deployment to spin up
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	// 3b. Ensure Service exists
	var svc corev1.Service
	if err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: req.Namespace}, &svc); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating Optuna Suggestion Service", "Service.Name", deploymentName)
			newSvc := r.constructOptunaService(optJob, deploymentName)
			if err := r.Create(ctx, newSvc); err != nil {
				return ctrl.Result{}, err
			}
			// Requeue immediately to wait for the Service networking to establish
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
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
	for _, tj := range childTrainJobs.Items {
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

	if optJob.Spec.NumTrials != nil && totalTrials >= *optJob.Spec.NumTrials && activeTrials == 0 {
		log.Info("All trials finished. Marking OptimizationJob Complete.")

		optJob.Status.Result = r.extractBestResult(optJob, childTrainJobs.Items)
		markJobCompleted(optJob)

		if err := r.Status().Update(ctx, optJob); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// ========================================================================
	// 6. Scale Up New Trials based on Budget Constraints (via Optuna gRPC)
	// ========================================================================

	var parallelLimit int32 = 1
	if optJob.Spec.ParallelTrials != nil {
		parallelLimit = *optJob.Spec.ParallelTrials
	}

	var totalLimit int32 = 1
	if optJob.Spec.NumTrials != nil {
		totalLimit = *optJob.Spec.NumTrials
	}

	trialsToSpawn := parallelLimit - activeTrials
	trialsRemaining := totalLimit - totalTrials

	// Prevent overshooting the total budget
	if trialsToSpawn > trialsRemaining {
		trialsToSpawn = trialsRemaining
	}

	if trialsToSpawn > 0 {
		// 1. Build the Katib-compatible GetSuggestionsRequest using historical trial data
		suggestionReq := r.buildSuggestionRequest(optJob, childTrainJobs.Items, trialsToSpawn)

		// 2. Call the Optuna gRPC Suggestion Service
		serviceAddr := fmt.Sprintf("%s-optuna.%s.svc.cluster.local:6789", optJob.Name, optJob.Namespace)

		// Inject the real client if one isn't provided (e.g., in production)
		if r.SuggestionClient == nil {
			r.SuggestionClient = &RealSuggestionClient{}
		}

		suggestions, err := r.SuggestionClient.GetSuggestions(ctx, serviceAddr, suggestionReq)
		if err != nil {
			log.Error(err, "Failed to fetch suggestions from Optuna gRPC service")
			return ctrl.Result{RequeueAfter: time.Second * 5}, err
		}

		// 3. Spawn a TrainJob trial for each suggestion returned by Optuna
		for _, paramAssignments := range suggestions {
			newTrainJob := r.constructTrainJob(optJob, paramAssignments)

			log.Info("Creating a new TrainJob trial with Optuna suggestions", "TrainJob.Name", newTrainJob.Name)
			if err := r.Create(ctx, newTrainJob); err != nil {
				log.Error(err, "Failed to create new TrainJob", "TrainJob.Name", newTrainJob.Name)
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
func (r *OptimizationJobReconciler) constructTrainJob(optJob *trainer.OptimizationJob, params []trainer.ParameterAssignment) *trainer.TrainJob {
	tj := &trainer.TrainJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-trial-%s", optJob.Name, rand.String(5)),
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
func (r *OptimizationJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&trainer.OptimizationJob{}).
		Owns(&trainer.TrainJob{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

// --- Helper Functions ---

func isJobCompleted(optJob *trainer.OptimizationJob) bool {
	// Returns true if either Complete or Failed condition is currently True
	return meta.IsStatusConditionTrue(optJob.Status.Conditions, OptimizationJobComplete) ||
		meta.IsStatusConditionTrue(optJob.Status.Conditions, OptimizationJobFailed)
}

func isTrainJobSucceeded(tj *trainer.TrainJob) bool {
	return meta.IsStatusConditionTrue(tj.Status.Conditions, TrainJobComplete)
}

func isTrainJobFailed(tj *trainer.TrainJob) bool {
	return meta.IsStatusConditionTrue(tj.Status.Conditions, TrainJobFailed)
}

func markJobCompleted(optJob *trainer.OptimizationJob) {
	meta.SetStatusCondition(&optJob.Status.Conditions, metav1.Condition{
		Type:    OptimizationJobComplete,
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
							// Using the existing Katib Optuna image for the MVP phase
							Image: "docker.io/kubeflowkatib/suggestion-optuna:latest",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 6789, // Standard gRPC port
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
					Port:     6789,
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
	if len(optJob.Spec.Objectives) > 0 && optJob.Spec.Objectives[0].Metric != nil {
		targetMetric = *optJob.Spec.Objectives[0].Metric
		if optJob.Spec.Objectives[0].Direction != nil && *optJob.Spec.Objectives[0].Direction == trainer.ObjectiveDirectionMaximize {
			optunaObjType = katibapi.ObjectiveType_MAXIMIZE
		} else {
			optunaObjType = katibapi.ObjectiveType_MINIMIZE
		}
	}

	// 2. Map Search Space Parameters
	var grpcParams []*katibapi.ParameterSpec
	for _, p := range optJob.Spec.Parameters {
		paramSpec := &katibapi.ParameterSpec{
			Name:          p.Name,
			ParameterType: katibapi.ParameterType_DOUBLE, // Katib float representation
		}
		if p.SearchSpace.Uniform != nil {
			paramSpec.FeasibleSpace = &katibapi.FeasibleSpace{
				Max: fmt.Sprintf("%v", p.SearchSpace.Uniform.Max),
				Min: fmt.Sprintf("%v", p.SearchSpace.Uniform.Min),
			}
		} else if p.SearchSpace.Categorical != nil {
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
		if tj.Status.TrainerStatus != nil {
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
	if len(optJob.Spec.Objectives) == 0 || optJob.Spec.Objectives[0].Metric == nil {
		return nil
	}

	targetMetric := *optJob.Spec.Objectives[0].Metric
	isMaximize := optJob.Spec.Objectives[0].Direction != nil && *optJob.Spec.Objectives[0].Direction == trainer.ObjectiveDirectionMaximize

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
