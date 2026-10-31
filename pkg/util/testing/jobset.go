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

package testing

import (
	corev1 "k8s.io/api/core/v1"
	batchv1ac "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	jobsetv1alpha2ac "sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"

	"github.com/kubeflow/trainer/v2/pkg/constants"
)

// JobSetSpecApplyWithNodeResources returns the JobSetSpec ApplyConfiguration that Info carries for a
// trainer node container requesting res. The Runtime resources and the TrainJob
// `.spec.trainer.resourcesPerNode` override are already merged into it by the time plugins run.
func JobSetSpecApplyWithNodeResources(res corev1.ResourceList) *jobsetv1alpha2ac.JobSetSpecApplyConfiguration {
	return jobsetv1alpha2ac.JobSetSpec().WithReplicatedJobs(
		jobsetv1alpha2ac.ReplicatedJob().
			WithName(constants.Node).
			WithTemplate(batchv1ac.JobTemplateSpec().
				WithSpec(batchv1ac.JobSpec().
					WithTemplate(corev1ac.PodTemplateSpec().
						WithSpec(corev1ac.PodSpec().
							WithContainers(corev1ac.Container().
								WithName(constants.Node).
								WithResources(corev1ac.ResourceRequirements().WithRequests(res)),
							),
						),
					),
				),
			),
	)
}
