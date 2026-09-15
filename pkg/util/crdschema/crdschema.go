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

// Package crdschema post-processes the CRD manifests emitted by controller-gen.
//
// TrainingRuntime and ClusterTrainingRuntime embed the whole JobSet spec, which
// transitively embeds batch/v1.JobSpec. Upstream Kubernetes types bound their
// lists with the declarative validation marker +k8s:maxItems, but controller-gen
// does not implement that marker (verified through controller-tools v0.22.0), so
// the bound is silently dropped from the generated schema. It does implement
// +k8s:immutable, so the generated CRD ends up with a `self == oldSelf` CEL rule
// on an unbounded list. The API server estimates the cost of such a rule from the
// declared bounds and rejects the CRD outright:
//
//	x-kubernetes-validations estimated rule cost total for entire OpenAPIv3 schema
//	exceeds budget by factor of more than 100x
//
// ApplyBounds restores the dropped bounds so the CRDs stay installable, and
// Validate runs the API server's own CRD validation over the result so a future
// upstream field that repeats this pattern fails at generation time rather than
// at install time.
package crdschema

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/install"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"
)

// docSeparator is the YAML document separator controller-gen writes ahead of
// every generated CRD. Everything up to and including it is preserved verbatim,
// so the tool is safe to run both on fresh controller-gen output and on a
// manifest that already carries the license header the Makefile prepends.
var docSeparator = []byte("---\n")

// missingArrayBounds maps a schema property name to the maxItems bound the
// upstream Kubernetes type declares with +k8s:maxItems. Only array properties
// that already carry a CEL rule are bounded, since those are the ones that
// contribute to the API server's cost budget.
//
// Remove an entry once controller-gen emits the bound on its own. Tracked
// upstream in kubernetes-sigs/controller-tools#1473.
var missingArrayBounds = map[string]int64{
	// batch/v1.JobSchedulingConfiguration.ResourceClaims, added in Kubernetes
	// 1.37 and reachable through JobSet's replicated job template.
	"resourceClaims": 4,
}

var scheme = runtime.NewScheme()

func init() {
	install.Install(scheme)
}

// ApplyBounds restores the array bounds controller-gen dropped from every schema
// in the CRD, and reports how many it added.
func ApplyBounds(crd *apiextensionsv1.CustomResourceDefinition) int {
	added := 0
	for i := range crd.Spec.Versions {
		if s := crd.Spec.Versions[i].Schema; s != nil {
			added += applyBounds(s.OpenAPIV3Schema)
		}
	}
	return added
}

func applyBounds(schema *apiextensionsv1.JSONSchemaProps) int {
	if schema == nil {
		return 0
	}
	added := 0
	for name, prop := range schema.Properties {
		if maxItems, ok := missingArrayBounds[name]; ok &&
			prop.Type == "array" && prop.MaxItems == nil && len(prop.XValidations) > 0 {
			prop.MaxItems = ptr.To(maxItems)
			added++
		}
		added += applyBounds(&prop)
		schema.Properties[name] = prop
	}
	if schema.Items != nil {
		added += applyBounds(schema.Items.Schema)
		for i := range schema.Items.JSONSchemas {
			added += applyBounds(&schema.Items.JSONSchemas[i])
		}
	}
	if schema.AdditionalProperties != nil {
		added += applyBounds(schema.AdditionalProperties.Schema)
	}
	return added
}

// Validate reports whether the API server would accept the CRD, so schema
// regressions surface during code generation instead of during installation.
func Validate(crd *apiextensionsv1.CustomResourceDefinition) error {
	internal := &apiextensions.CustomResourceDefinition{}
	if err := scheme.Convert(crd, internal, nil); err != nil {
		return fmt.Errorf("converting %q to the internal version: %w", crd.Name, err)
	}
	// Generated manifests carry no status, but the API server validates a CRD
	// that already has one. Seed the stored version from the spec so validation
	// exercises the schema rather than tripping over the empty status.
	for _, version := range internal.Spec.Versions {
		if version.Storage {
			internal.Status.StoredVersions = append(internal.Status.StoredVersions, version.Name)
		}
	}
	internal.Status.AcceptedNames = internal.Spec.Names
	if errs := validation.ValidateCustomResourceDefinition(context.Background(), internal); len(errs) > 0 {
		return fmt.Errorf("%q would be rejected by the API server: %w", crd.Name, errs.ToAggregate())
	}
	return nil
}

// Patch applies the missing bounds to a generated CRD manifest and validates the
// result. It round-trips through the same encoder controller-gen uses, so a
// manifest that needs no bounds is returned byte for byte unchanged.
func Patch(manifest []byte) ([]byte, int, error) {
	preamble, body := split(manifest)
	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(body, &crd); err != nil {
		return nil, 0, fmt.Errorf("parsing CRD manifest: %w", err)
	}
	added := ApplyBounds(&crd)
	if err := Validate(&crd); err != nil {
		return nil, 0, err
	}
	encoded, err := encode(&crd)
	if err != nil {
		return nil, 0, err
	}
	return append(preamble, encoded...), added, nil
}

// split separates the license header and document separator that precede the
// CRD body from the body itself.
func split(manifest []byte) (preamble, body []byte) {
	if i := bytes.Index(manifest, docSeparator); i >= 0 && (i == 0 || manifest[i-1] == '\n') {
		end := i + len(docSeparator)
		return bytes.Clone(manifest[:end]), manifest[end:]
	}
	return nil, manifest
}

// encode marshals the CRD the way controller-gen does, minus the always-empty
// status that marshalling the typed object would otherwise introduce.
func encode(crd *apiextensionsv1.CustomResourceDefinition) ([]byte, error) {
	encoded, err := json.Marshal(crd)
	if err != nil {
		return nil, fmt.Errorf("encoding CRD %q: %w", crd.Name, err)
	}
	// Decode with UseNumber so integer bounds such as maxItems do not round-trip
	// through float64 and change how they are rendered.
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var object map[string]any
	if err = decoder.Decode(&object); err != nil {
		return nil, fmt.Errorf("decoding CRD %q: %w", crd.Name, err)
	}
	delete(object, "status")
	if encoded, err = json.Marshal(object); err != nil {
		return nil, fmt.Errorf("re-encoding CRD %q: %w", crd.Name, err)
	}
	out, err := yaml.JSONToYAML(encoded)
	if err != nil {
		return nil, fmt.Errorf("converting CRD %q to YAML: %w", crd.Name, err)
	}
	return out, nil
}
