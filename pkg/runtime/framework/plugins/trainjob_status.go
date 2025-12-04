package plugins

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
)

type DefaultTrainJobStatusPlugin struct{}

var _ framework.TrainJobStatusPlugin = (*DefaultTrainJobStatusPlugin)(nil)

func (p *DefaultTrainJobStatusPlugin) Name() string {
	return "default-trainjob-status"
}

func (p *DefaultTrainJobStatusPlugin) Status(
	ctx context.Context,
	trainJob *trainer.TrainJob,
) (*trainer.TrainJobStatus, error) {

	if len(trainJob.Status.JobsStatus) == 0 {
		return nil, nil
	}
	status := &trainer.TrainJobStatus{
		JobsStatus: trainJob.Status.JobsStatus,
	}

	var (
		hasActive    bool
		hasSucceeded bool
		hasFailed    bool
		hasSuspended bool
	)

	for _, js := range status.JobsStatus {
		if ptr.Deref(js.Active, 0) > 0 {
			hasActive = true
		}
		if ptr.Deref(js.Succeeded, 0) > 0 {
			hasSucceeded = true
		}
		if ptr.Deref(js.Failed, 0) > 0 {
			hasFailed = true
		}
		if ptr.Deref(js.Suspended, 0) > 0 {
			hasSuspended = true
		}
	}

	switch {
	case hasFailed:
		meta := metav1.Condition{
			Type:   trainer.TrainJobFailed,
			Status: metav1.ConditionTrue,
			Reason: "JobFailed",
		}
		status.Conditions = append(status.Conditions, meta)

	case hasSucceeded && !hasActive:
		meta := metav1.Condition{
			Type:   trainer.TrainJobComplete,
			Status: metav1.ConditionTrue,
			Reason: "JobCompleted",
		}
		status.Conditions = append(status.Conditions, meta)

	case hasActive:
		meta := metav1.Condition{
			Type:   trainer.TrainJobConditionRunning,
			Status: metav1.ConditionTrue,
			Reason: "JobRunning",
		}
		status.Conditions = append(status.Conditions, meta)

	default:
		meta := metav1.Condition{
			Type:   trainer.TrainJobConditionPending,
			Status: metav1.ConditionTrue,
			Reason: "WaitingForExecution",
		}
		status.Conditions = append(status.Conditions, meta)
	}

	if hasSuspended {
		meta := metav1.Condition{
			Type:   trainer.TrainJobSuspended,
			Status: metav1.ConditionTrue,
			Reason: "JobSuspended",
		}
		status.Conditions = append(status.Conditions, meta)
	}

	return status, nil
}
