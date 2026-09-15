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

package crdschema

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// immutableRule is the CEL rule controller-gen emits for a +k8s:immutable field.
// It is what makes an unbounded list expensive enough for the API server to
// reject the whole CRD.
var immutableRule = []apiextensionsv1.ValidationRule{{
	Rule:    "self == oldSelf",
	Message: "field is immutable",
}}

// unboundedClaims mirrors the schema controller-gen generates for
// batch/v1.JobSchedulingConfiguration.ResourceClaims: a list with an
// immutability rule but no maxItems, because +k8s:maxItems is dropped.
func unboundedClaims() apiextensionsv1.JSONSchemaProps {
	return apiextensionsv1.JSONSchemaProps{
		Type: "array",
		Items: &apiextensionsv1.JSONSchemaPropsOrArray{
			Schema: &apiextensionsv1.JSONSchemaProps{
				Type: "object",
				Properties: map[string]apiextensionsv1.JSONSchemaProps{
					"name":                      {Type: "string"},
					"resourceClaimName":         {Type: "string"},
					"resourceClaimTemplateName": {Type: "string"},
				},
				Required: []string{"name"},
			},
		},
		XValidations: immutableRule,
	}
}

// crdWith builds a CRD whose spec has the given properties.
func crdWith(properties map[string]apiextensionsv1.JSONSchemaProps) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "tests.trainer.kubeflow.org"},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "trainer.kubeflow.org",
			Scope: apiextensionsv1.NamespaceScoped,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural: "tests", Singular: "test", Kind: "Test", ListKind: "TestList",
			},
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
				Name:    "v1alpha1",
				Served:  true,
				Storage: true,
				Schema: &apiextensionsv1.CustomResourceValidation{
					OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
						Type: "object",
						Properties: map[string]apiextensionsv1.JSONSchemaProps{
							"spec": {Type: "object", Properties: properties},
						},
					},
				},
			}},
		},
	}
}

func TestApplyBounds(t *testing.T) {
	boundedClaims := func() apiextensionsv1.JSONSchemaProps {
		claims := unboundedClaims()
		claims.MaxItems = ptr.To[int64](8)
		return claims
	}
	cases := map[string]struct {
		properties   map[string]apiextensionsv1.JSONSchemaProps
		wantAdded    int
		wantMaxItems *int64
	}{
		"bounds an unbounded list that carries a CEL rule": {
			properties:   map[string]apiextensionsv1.JSONSchemaProps{"resourceClaims": unboundedClaims()},
			wantAdded:    1,
			wantMaxItems: ptr.To[int64](4),
		},
		"leaves an existing bound alone": {
			properties:   map[string]apiextensionsv1.JSONSchemaProps{"resourceClaims": boundedClaims()},
			wantAdded:    0,
			wantMaxItems: ptr.To[int64](8),
		},
		"skips a list without CEL rules": {
			properties: map[string]apiextensionsv1.JSONSchemaProps{
				"resourceClaims": {Type: "array", Items: unboundedClaims().Items},
			},
			wantAdded:    0,
			wantMaxItems: nil,
		},
		"skips a property that is not a list": {
			properties:   map[string]apiextensionsv1.JSONSchemaProps{"resourceClaims": {Type: "object", XValidations: immutableRule}},
			wantAdded:    0,
			wantMaxItems: nil,
		},
		"skips an unrelated list": {
			properties:   map[string]apiextensionsv1.JSONSchemaProps{"tolerations": unboundedClaims()},
			wantAdded:    0,
			wantMaxItems: nil,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			crd := crdWith(tc.properties)
			if got := ApplyBounds(crd); got != tc.wantAdded {
				t.Errorf("ApplyBounds() = %d, want %d", got, tc.wantAdded)
			}
			spec := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"]
			for property, schema := range spec.Properties {
				if diff := cmp.Diff(tc.wantMaxItems, schema.MaxItems); diff != "" {
					t.Errorf("unexpected maxItems on %q (-want +got):\n%s", property, diff)
				}
			}
		})
	}
}

// replicatedJobsWith nests the given properties the way the generated CRDs do:
// inside the replicatedJobs list, whose unbounded cardinality multiplies the
// cost of every CEL rule below it. The list is correlatable, as JobSet declares
// it, so that oldSelf stays usable underneath.
func replicatedJobsWith(properties map[string]apiextensionsv1.JSONSchemaProps) map[string]apiextensionsv1.JSONSchemaProps {
	return map[string]apiextensionsv1.JSONSchemaProps{
		"replicatedJobs": {
			Type:         "array",
			XListType:    ptr.To("map"),
			XListMapKeys: []string{"name"},
			Items: &apiextensionsv1.JSONSchemaPropsOrArray{
				Schema: &apiextensionsv1.JSONSchemaProps{
					Type:     "object",
					Required: []string{"name"},
					Properties: map[string]apiextensionsv1.JSONSchemaProps{
						"name": {Type: "string"},
						"template": {
							Type:       "object",
							Properties: properties,
						},
					},
				},
			},
		},
	}
}

// TestApplyBoundsNested checks that the walk reaches schemas nested below list
// items and additionalProperties, which is where the embedded JobSet spec puts
// the affected fields.
func TestApplyBoundsNested(t *testing.T) {
	cases := map[string]struct {
		properties map[string]apiextensionsv1.JSONSchemaProps
		get        func(spec apiextensionsv1.JSONSchemaProps) *int64
	}{
		"below list items": {
			properties: replicatedJobsWith(map[string]apiextensionsv1.JSONSchemaProps{
				"resourceClaims": unboundedClaims(),
			}),
			get: func(spec apiextensionsv1.JSONSchemaProps) *int64 {
				return spec.Properties["replicatedJobs"].Items.Schema.
					Properties["template"].Properties["resourceClaims"].MaxItems
			},
		},
		"below additionalProperties": {
			properties: map[string]apiextensionsv1.JSONSchemaProps{
				"overrides": {
					Type: "object",
					AdditionalProperties: &apiextensionsv1.JSONSchemaPropsOrBool{
						Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"resourceClaims": unboundedClaims(),
							},
						},
					},
				},
			},
			get: func(spec apiextensionsv1.JSONSchemaProps) *int64 {
				return spec.Properties["overrides"].AdditionalProperties.Schema.
					Properties["resourceClaims"].MaxItems
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			crd := crdWith(tc.properties)
			if got := ApplyBounds(crd); got != 1 {
				t.Fatalf("ApplyBounds() = %d, want 1", got)
			}
			spec := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"]
			if diff := cmp.Diff(ptr.To[int64](4), tc.get(spec)); diff != "" {
				t.Errorf("unexpected nested maxItems (-want +got):\n%s", diff)
			}
		})
	}
}

// TestValidate pins the behaviour this whole package exists for: an unbounded
// list with an immutability rule blows the API server's CEL cost budget, and
// restoring the bound brings it back under.
func TestValidate(t *testing.T) {
	crd := crdWith(replicatedJobsWith(map[string]apiextensionsv1.JSONSchemaProps{
		"resourceClaims": unboundedClaims(),
	}))
	err := Validate(crd)
	if err == nil {
		t.Fatal("Validate() = nil, want a CEL cost budget error for the unbounded list")
	}
	if !strings.Contains(err.Error(), "cost") {
		t.Fatalf("Validate() = %v, want a CEL cost budget error", err)
	}
	if ApplyBounds(crd) != 1 {
		t.Fatal("ApplyBounds() did not bound the list")
	}
	if err = Validate(crd); err != nil {
		t.Errorf("Validate() after ApplyBounds() = %v, want nil", err)
	}
}

// TestPatchGeneratedManifests is the regression guard for the checked-in CRDs:
// they must be installable, and re-encoding them must not rewrite the file. A
// non-empty diff here means the generated manifests are stale, so running
// `make manifests` is the fix.
func TestPatchGeneratedManifests(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "..", "manifests", "base", "crds", "trainer.kubeflow.org_*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no generated CRD manifests found")
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			manifest, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			patched, added, err := Patch(manifest)
			if err != nil {
				t.Fatalf("Patch() = %v, want the CRD to be installable", err)
			}
			if added != 0 {
				t.Errorf("Patch() restored %d bound(s); run `make manifests`", added)
			}
			if !bytes.Equal(manifest, patched) {
				t.Errorf("Patch() rewrote the manifest; run `make manifests`")
			}
		})
	}
}

// TestPatchPreservesPreamble checks the license header and document separator
// the Makefile relies on survive a patch.
func TestPatchPreservesPreamble(t *testing.T) {
	crd := crdWith(map[string]apiextensionsv1.JSONSchemaProps{"resourceClaims": unboundedClaims()})
	body, err := encode(crd)
	if err != nil {
		t.Fatal(err)
	}
	preamble := "# Copyright The Kubeflow Authors.\n\n---\n"
	patched, added, err := Patch(append([]byte(preamble), body...))
	if err != nil {
		t.Fatal(err)
	}
	if added != 1 {
		t.Errorf("Patch() restored %d bound(s), want 1", added)
	}
	if !bytes.HasPrefix(patched, []byte(preamble)) {
		t.Errorf("Patch() dropped the preamble, got:\n%s", string(patched[:min(len(patched), 120)]))
	}
}
