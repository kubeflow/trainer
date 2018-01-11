// +build !ignore_autogenerated

/*
Copyright 2018 The Kubernetes Authors.

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

// This file was autogenerated by defaulter-gen. Do not edit it manually!

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// RegisterDefaults adds defaulters functions to the given scheme.
// Public to allow building arbitrary schemes.
// All generated defaulters are covering - they call all nested defaulters.
func RegisterDefaults(scheme *runtime.Scheme) error {
	scheme.AddTypeDefaultingFunc(&TfJob{}, func(obj interface{}) { SetObjectDefaults_TfJob(obj.(*TfJob)) })
	scheme.AddTypeDefaultingFunc(&TfJobList{}, func(obj interface{}) { SetObjectDefaults_TfJobList(obj.(*TfJobList)) })
	return nil
}

func SetObjectDefaults_TfJob(in *TfJob) {
	SetDefaults_TfJob(in)
}

func SetObjectDefaults_TfJobList(in *TfJobList) {
	for i := range in.Items {
		a := &in.Items[i]
		SetObjectDefaults_TfJob(a)
	}
}
