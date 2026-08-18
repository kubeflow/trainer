/*
Copyright 2026 The Kubeflow Authors.

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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
)

const (
	// OptimizationJobCreated means that the OptimizationJob has been created and initialized.
	OptimizationJobCreated string = "Created"
	// OptimizationJobComplete means that all trials for the OptimizationJob have finished.
	OptimizationJobComplete string = "Complete"
	// OptimizationJobFailed means that trial execution encountered an error.
	OptimizationJobFailed string = "Failed"
	// OptimizationJobSuspended means that trial execution is suspended (e.g. by Kueue).
	OptimizationJobSuspended string = "Suspended"

	// EnvOptParamPrefix is the prefix used for injected hyperparameter environment variables.
	EnvOptParamPrefix = "KUBEFLOW_TRAINER_OPT_"
	// AnnotationOptParams is the metadata annotation key storing trial hyperparameter assignments.
	AnnotationOptParams = "trainer.kubeflow.org/optimization-parameters"
)

type OptimizationJobReconciler struct {
	log      logr.Logger
	client   client.Client
	recorder events.EventRecorder
}

var _ reconcile.Reconciler = (*OptimizationJobReconciler)(nil)

func NewOptimizationJobReconciler(
	client client.Client,
	recorder events.EventRecorder,
) *OptimizationJobReconciler {
	return &OptimizationJobReconciler{
		log:      ctrl.Log.WithName("optimizationjob-controller"),
		client:   client,
		recorder: recorder,
	}
}

func (r *OptimizationJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.V(2).Info("Reconciling OptimizationJob", "request", req)

	var optJob trainer.OptimizationJob
	if err := r.client.Get(ctx, req.NamespacedName, &optJob); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !optJob.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Skip jobs managed by external controllers (e.g. MultiKueue)
	if optJob.Spec.ManagedBy != nil && *optJob.Spec.ManagedBy != "trainer.kubeflow.org/optimizationjob-controller" {
		r.log.V(2).Info("Skipping OptimizationJob managed by external controller", "managedBy", *optJob.Spec.ManagedBy)
		return ctrl.Result{}, nil
	}

	// Keep track of original status for SSA / MergeFrom patch calculation
	prevOptJob := optJob.DeepCopy()

	if optJob.Status == nil {
		optJob.Status = &trainer.OptimizationJobStatus{}
	}

	// Handle Kueue suspension gating
	isSuspended := ptr.Deref(optJob.Spec.Suspend, false)
	if isSuspended {
		meta.SetStatusCondition(&optJob.Status.Conditions, metav1.Condition{
			Type:               OptimizationJobSuspended,
			Status:             metav1.ConditionTrue,
			Reason:             "OptimizationJobSuspended",
			Message:            "OptimizationJob trial creation is suspended by Kueue",
			LastTransitionTime: metav1.Now(),
		})
	} else {
		meta.RemoveStatusCondition(&optJob.Status.Conditions, OptimizationJobSuspended)
	}

	// 1. Initialize status conditions if empty.
	if len(optJob.Status.Conditions) == 0 {
		meta.SetStatusCondition(&optJob.Status.Conditions, metav1.Condition{
			Type:               OptimizationJobCreated,
			Status:             metav1.ConditionTrue,
			Reason:             "OptimizationJobCreated",
			Message:            "OptimizationJob is initializing trials",
			LastTransitionTime: metav1.Now(),
		})
	}

	// 2. Fetch all child TrainJobs owned by this OptimizationJob.
	var trainJobList trainer.TrainJobList
	if err := r.client.List(ctx, &trainJobList, client.InNamespace(optJob.Namespace)); err != nil {
		return ctrl.Result{}, err
	}

	var childTrainJobs []trainer.TrainJob
	for _, tj := range trainJobList.Items {
		if metav1.IsControlledBy(&tj, &optJob) {
			childTrainJobs = append(childTrainJobs, tj)
		}
	}

	totalTrials := optJob.Spec.NumTrials
	if totalTrials <= 0 {
		totalTrials = 1
	}
	parallelTrials := optJob.Spec.ParallelTrials
	if parallelTrials <= 0 {
		parallelTrials = 1
	}

	activeCount := int32(0)
	completedCount := int32(0)
	failedCount := int32(0)
	var bestResult *trainer.Result

	for _, tj := range childTrainJobs {
		if isTrainJobComplete(&tj) {
			completedCount++
			if bestResult == nil {
				bestResult = buildResultFromTrainJob(&tj)
			}
		} else if isTrainJobFailed(&tj) {
			failedCount++
		} else {
			activeCount++
		}
	}

	// Dynamically track best trial result as trials finish
	if bestResult != nil {
		optJob.Status.Result = *bestResult
	}

	// 3. Check if all trials are finished.
	allFinished := (completedCount+failedCount >= totalTrials)
	if allFinished {
		meta.SetStatusCondition(&optJob.Status.Conditions, metav1.Condition{
			Type:               OptimizationJobComplete,
			Status:             metav1.ConditionTrue,
			Reason:             "MaxTrialsReached",
			Message:            fmt.Sprintf("Completed %d of %d trials", completedCount, totalTrials),
			LastTransitionTime: metav1.Now(),
		})
	}

	// Patch status cleanly if status changed (matching TrainJob controller & PR #3362 pattern)
	if prevOptJob.Status == nil || !equality.Semantic.DeepEqual(optJob.Status, prevOptJob.Status) {
		if err := r.client.Status().Patch(ctx, &optJob, client.MergeFrom(prevOptJob)); err != nil {
			return ctrl.Result{}, err
		}
	}

	if allFinished {
		return ctrl.Result{}, nil
	}

	if isSuspended {
		r.log.V(2).Info("OptimizationJob is suspended, skipping trial creation", "optimizationJob", optJob.Name)
		return ctrl.Result{}, nil
	}

	// 4. Provision new child TrainJobs up to parallelTrials limit.
	createdCount := int32(len(childTrainJobs))
	if activeCount < parallelTrials && createdCount < totalTrials {
		newTrialIndex := createdCount + 1
		trialJob, err := r.constructTrialTrainJob(&optJob, newTrialIndex)
		if err != nil {
			r.log.Error(err, "Failed to construct trial TrainJob", "trialIndex", newTrialIndex)
			return ctrl.Result{}, err
		}

		if err := r.client.Create(ctx, trialJob); err != nil {
			r.log.Error(err, "Failed to create trial TrainJob", "trialJobName", trialJob.Name)
			return ctrl.Result{}, err
		}
		r.log.Info("Created trial TrainJob", "trialJobName", trialJob.Name, "optimizationJob", optJob.Name)
	}

	return ctrl.Result{}, nil
}

func (r *OptimizationJobReconciler) constructTrialTrainJob(optJob *trainer.OptimizationJob, trialIndex int32) (*trainer.TrainJob, error) {
	trialName := fmt.Sprintf("%s-trial-%d", optJob.Name, trialIndex)

	assignments := make([]trainer.ParameterAssignment, 0, len(optJob.Spec.Parameters))
	envVars := make([]corev1.EnvVar, 0, len(optJob.Spec.Parameters))

	for _, param := range optJob.Spec.Parameters {
		val := sampleParameterValue(param, trialIndex)
		assignments = append(assignments, trainer.ParameterAssignment{
			Name:  param.Name,
			Value: val,
		})
		envName := EnvOptParamPrefix + strings.ToUpper(strings.ReplaceAll(param.Name, "-", "_"))
		envVars = append(envVars, corev1.EnvVar{
			Name:  envName,
			Value: val,
		})
	}

	paramJSON, _ := json.Marshal(assignments)

	trainJob := &trainer.TrainJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      trialName,
			Namespace: optJob.Namespace,
			Annotations: map[string]string{
				AnnotationOptParams: string(paramJSON),
			},
			Labels: map[string]string{
				"trainer.kubeflow.org/optimization-job": optJob.Name,
			},
		},
		Spec: optJob.Spec.TrainJobTemplate.Spec,
	}

	if trainJob.Spec.Trainer != nil {
		trainJob.Spec.Trainer.Env = append(trainJob.Spec.Trainer.Env, envVars...)
	}

	if err := ctrl.SetControllerReference(optJob, trainJob, r.client.Scheme()); err != nil {
		return nil, err
	}

	return trainJob, nil
}

func sampleParameterValue(param trainer.Parameter, seed int32) string {
	space := param.SearchSpace
	if space == nil {
		return "0"
	}
	if len(space.Categorical.Choices) > 0 {
		idx := int(seed-1) % len(space.Categorical.Choices)
		return space.Categorical.Choices[idx]
	}
	if space.Uniform.Min != "" && space.Uniform.Max != "" {
		minVal, _ := strconv.ParseFloat(string(space.Uniform.Min), 64)
		maxVal, _ := strconv.ParseFloat(string(space.Uniform.Max), 64)
		val := minVal + (maxVal-minVal)*0.5
		return fmt.Sprintf("%.4f", val)
	}
	if space.LogUniform.Min != "" && space.LogUniform.Max != "" {
		minVal, _ := strconv.ParseFloat(string(space.LogUniform.Min), 64)
		maxVal, _ := strconv.ParseFloat(string(space.LogUniform.Max), 64)
		val := minVal + (maxVal-minVal)*0.5
		return fmt.Sprintf("%.4f", val)
	}
	return "0"
}

func isTrainJobComplete(tj *trainer.TrainJob) bool {
	for _, c := range tj.Status.Conditions {
		if c.Type == trainer.TrainJobComplete && c.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

func isTrainJobFailed(tj *trainer.TrainJob) bool {
	for _, c := range tj.Status.Conditions {
		if c.Type == trainer.TrainJobFailed && c.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

func buildResultFromTrainJob(tj *trainer.TrainJob) *trainer.Result {
	res := &trainer.Result{
		TrainJobName: tj.Name,
	}
	if ann, ok := tj.Annotations[AnnotationOptParams]; ok {
		var assignments []trainer.ParameterAssignment
		if err := json.Unmarshal([]byte(ann), &assignments); err == nil {
			res.Parameters = assignments
		}
	}
	return res
}

func (r *OptimizationJobReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&trainer.OptimizationJob{}).
		Owns(&trainer.TrainJob{}).
		WithOptions(options).
		Complete(r)
}
