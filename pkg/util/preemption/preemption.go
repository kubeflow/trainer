/*
Copyright 2025 The Kubeflow Authors.

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

package preemption

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
)

const (
	// PreemptionRestartCountAnnotation tracks the number of times a Pod has been
	// recreated due to scheduler preemption. This is used to enforce the max
	// preemption restart limit.
	PreemptionRestartCountAnnotation = "trainer.kubeflow.org/preemption-restart-count"

	// DefaultMaxPreemptionRestarts is the default maximum number of times a Pod
	// can be recreated after preemption. 0 means unlimited.
	DefaultMaxPreemptionRestarts = 3

	// PreemptionBySchedulerReason is the reason set by Kubernetes schedulers
	// (including Volcano) when preempting a pod.
	PreemptionBySchedulerReason = "PreemptionByScheduler"
)

// IsPodPreempted checks if a pod has been preempted by the scheduler.
// When Volcano or the default kube-scheduler preempts a pod, it sets a
// DisruptionTarget condition with reason "PreemptionByScheduler" on the pod.
// This is a standard Kubernetes mechanism (since K8s 1.26+).
func IsPodPreempted(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.DisruptionTarget &&
			condition.Status == corev1.ConditionTrue &&
			condition.Reason == PreemptionBySchedulerReason {
			return true
		}
	}
	return false
}

// GetPreemptionRestartCount returns the number of times a Pod has been
// recreated due to preemption, as tracked by the annotation.
func GetPreemptionRestartCount(pod *corev1.Pod) int32 {
	if pod.Annotations == nil {
		return 0
	}
	val, ok := pod.Annotations[PreemptionRestartCountAnnotation]
	if !ok {
		return 0
	}
	count, err := strconv.ParseInt(val, 10, 32)
	if err != nil {
		return 0
	}
	return int32(count)
}

// FilterPreemptedPods returns pods that have been preempted by the scheduler.
func FilterPreemptedPods(pods []corev1.Pod) []corev1.Pod {
	var result []corev1.Pod
	for i := range pods {
		if IsPodPreempted(&pods[i]) {
			result = append(result, pods[i])
		}
	}
	return result
}

// CountNonPreemptedFailedPods returns the count of failed pods that were NOT preempted.
// Preempted pods should not be counted as real failures since they will be recreated.
func CountNonPreemptedFailedPods(pods []corev1.Pod) int32 {
	var result int32
	for i := range pods {
		if pods[i].Status.Phase == corev1.PodFailed && !IsPodPreempted(&pods[i]) {
			result++
		}
	}
	return result
}
