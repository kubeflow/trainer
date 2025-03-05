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

package main

import (
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strings"

	"k8s.io/klog/v2"
	"k8s.io/kube-openapi/pkg/common"
	"k8s.io/kube-openapi/pkg/validation/spec"

	trainer "github.com/kubeflow/trainer/pkg/apis/trainer/v1alpha1"
)

// Generate Kubeflow Training OpenAPI specification.
func main() {
	// Get Kubernetes and JobSet version
	var k8sVersion string
	var jobSetVersion string

	info, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Println("Failed to read build info")
		return
	}

	for _, dep := range info.Deps {
		if dep.Path == "k8s.io/api" {
			k8sVersion = strings.Replace(dep.Version, "v0.", "v1.", -1)
		} else if dep.Path == "sigs.k8s.io/jobset" {
			jobSetVersion = dep.Version
		}
	}
	if k8sVersion == "" || jobSetVersion == "" {
		fmt.Println("OpenAPI spec generation failed. Unable to get Kubernetes and JobSet version")
		return
	}

	k8sOpenAPISpec := fmt.Sprintf("https://raw.githubusercontent.com/kubernetes/kubernetes/refs/tags/%s/api/openapi-spec/swagger.json", k8sVersion)
	// TODO (andreyvelich): Use the release version once this JobSet commit is released: d5c7bce.
	// jobSetOpenAPISpec := fmt.Sprintf("https://raw.githubusercontent.com/kubernetes-sigs/jobset/refs/tags/%s/hack/python-sdk/swagger.json", jobSetVersion)
	jobSetOpenAPISpec := "https://raw.githubusercontent.com/kubernetes-sigs/jobset/d5c7bcebe739a4577e30944370c2d7a68321a929/hack/python-sdk/swagger.json"

	var oAPIDefs = map[string]common.OpenAPIDefinition{}
	defs := spec.Definitions{}

	refCallback := func(name string) spec.Ref {
		if strings.HasPrefix(name, "k8s.io") {
			return spec.MustCreateRef(k8sOpenAPISpec + "#/definitions/" + swaggify(name))
		} else if strings.HasPrefix(name, "sigs.k8s.io/jobset") {
			return spec.MustCreateRef(jobSetOpenAPISpec + "#/definitions/" + swaggify(name))
		}
		return spec.MustCreateRef("#/definitions/" + swaggify(name))

	}

	for k, v := range trainer.GetOpenAPIDefinitions(refCallback) {
		oAPIDefs[k] = v
	}

	for defName, val := range oAPIDefs {
		defs[swaggify(defName)] = val.Schema
	}
	swagger := spec.Swagger{
		SwaggerProps: spec.SwaggerProps{
			Swagger:     "2.0",
			Definitions: defs,
			Paths:       &spec.Paths{Paths: map[string]spec.PathItem{}},
			Info: &spec.Info{
				InfoProps: spec.InfoProps{
					Title: "Kubeflow Trainer OpenAPI Spec",
				},
			},
		},
	}
	jsonBytes, err := json.MarshalIndent(swagger, "", "  ")
	if err != nil {
		klog.Fatal(err.Error())
	}
	fmt.Println(string(jsonBytes))
}

func swaggify(name string) string {
	name = strings.Replace(name, "github.com/kubeflow/trainer/pkg/apis/", "", -1)
	name = strings.Replace(name, "sigs.k8s.io/jobset/api/", "", -1)
	name = strings.Replace(name, "k8s.io", "io.k8s", -1)
	name = strings.Replace(name, "/", ".", -1)
	return name
}
