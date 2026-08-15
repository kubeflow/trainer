/*
Copyright 2024 The Kubeflow Authors.

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

package xgboost

import (
	"context"
	"fmt"
	"slices"
	"strings"

	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trainingoperator "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v1"
	"github.com/kubeflow/training-operator/pkg/common/util"
)

var (
	specPath           = field.NewPath("spec")
	xgbReplicaSpecPath = specPath.Child("xgbReplicaSpecs")
)

type Webhook struct{}

func SetupWebhook(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &trainingoperator.XGBoostJob{}).
		WithCustomValidator(&Webhook{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-kubeflow-org-v1-xgboostjob,mutating=false,failurePolicy=fail,sideEffects=None,groups=kubeflow.org,resources=xgboostjobs,verbs=create;update,versions=v1,name=validator.xgboostjob.training-operator.kubeflow.org,admissionReviewVersions=v1

var _ webhook.CustomValidator = &Webhook{}

func (w *Webhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	job := obj.(*trainingoperator.XGBoostJob)
	log := ctrl.LoggerFrom(ctx).WithName("xgboostjob-webhook")
	log.V(5).Info("Validating create", "xgboostJob", klog.KObj(job))
	return nil, validateXGBoostJob(nil, job).ToAggregate()
}

func (w *Webhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldJob := oldObj.(*trainingoperator.XGBoostJob)
	newJob := newObj.(*trainingoperator.XGBoostJob)
	log := ctrl.LoggerFrom(ctx).WithName("xgboostjob-webhook")
	log.V(5).Info("Validating create", "xgboostJob", klog.KObj(newJob))
	return nil, validateXGBoostJob(oldJob, newJob).ToAggregate()
}

func (w *Webhook) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validateXGBoostJob(oldJob, newJob *trainingoperator.XGBoostJob) field.ErrorList {
	var allErrs field.ErrorList
	if errors := apimachineryvalidation.NameIsDNS1035Label(newJob.Name, false); len(errors) != 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("metadata").Child("name"), newJob.Name, fmt.Sprintf("should match: %v", strings.Join(errors, ","))))
	}
	if oldJob != nil {
		allErrs = append(allErrs, util.ValidateRunPolicyUpdate(&oldJob.Spec.RunPolicy, &newJob.Spec.RunPolicy)...)
	}
	allErrs = append(allErrs, util.ValidateRunPolicy(&newJob.Spec.RunPolicy)...)
	allErrs = append(allErrs, validateSpec(newJob.Spec)...)
	return allErrs
}

func validateSpec(spec trainingoperator.XGBoostJobSpec) field.ErrorList {
	return validateXGBReplicaSpecs(spec.XGBReplicaSpecs)
}

func validateXGBReplicaSpecs(rSpecs map[trainingoperator.ReplicaType]*trainingoperator.ReplicaSpec) field.ErrorList {
	var allErrs field.ErrorList

	if rSpecs == nil {
		allErrs = append(allErrs, field.Required(xgbReplicaSpecPath, "must be required"))
	}
	masterExists := false
	for rType, rSpec := range rSpecs {
		rolePath := xgbReplicaSpecPath.Key(string(rType))
		containersPath := rolePath.Child("template").Child("spec").Child("containers")

		// Make sure the replica type is valid.
		validReplicaTypes := []trainingoperator.ReplicaType{
			trainingoperator.XGBoostJobReplicaTypeMaster,
			trainingoperator.XGBoostJobReplicaTypeWorker,
		}
		if !slices.Contains(validReplicaTypes, rType) {
			allErrs = append(allErrs, field.NotSupported(rolePath, rType, validReplicaTypes))
		}

		if rSpec == nil || len(rSpec.Template.Spec.Containers) == 0 {
			allErrs = append(allErrs, field.Required(containersPath, "must be specified"))
		}

		// Make sure the image is defined in the container
		defaultContainerPresent := false
		for idx, container := range rSpec.Template.Spec.Containers {
			if container.Image == "" {
				allErrs = append(allErrs, field.Required(containersPath.Index(idx).Child("image"), "must be required"))
			}
			if container.Name == trainingoperator.XGBoostJobDefaultContainerName {
				defaultContainerPresent = true
			}
		}
		// Make sure there has at least one container named "xgboost"
		if !defaultContainerPresent {
			allErrs = append(allErrs, field.Required(containersPath, fmt.Sprintf("must have at least one container with name %s", trainingoperator.XGBoostJobDefaultContainerName)))
		}
		if rType == trainingoperator.XGBoostJobReplicaTypeMaster {
			masterExists = true
			if rSpec.Replicas == nil || int(*rSpec.Replicas) != 1 {
				allErrs = append(allErrs, field.Forbidden(rolePath.Child("replicas"), "must be 1"))
			}
		}
	}
	if !masterExists {
		allErrs = append(allErrs, field.Required(xgbReplicaSpecPath.Key(string(trainingoperator.XGBoostJobReplicaTypeMaster)), "must be present"))
	}
	return allErrs
}
