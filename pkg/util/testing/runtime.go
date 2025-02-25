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

package testing

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func ToObject(s *runtime.Scheme, objects ...any) ([]runtime.Object, error) {
	var objs []runtime.Object
	for _, obj := range objects {
		o, err := toObject(s, obj)
		if err != nil {
			return nil, err
		}
		objs = append(objs, o)
	}
	return objs, nil
}

func toObject(s *runtime.Scheme, obj any) (runtime.Object, error) {
	if o, ok := obj.(runtime.Object); ok {
		return o, nil
	}

	var u *unstructured.Unstructured
	o, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	u = &unstructured.Unstructured{Object: o}

	gvk := u.GetObjectKind().GroupVersionKind()
	if !s.Recognizes(gvk) {
		return nil, fmt.Errorf("%s is not a recognized schema", gvk.GroupVersion().String())
	}
	typed, err := s.New(gvk)
	if err != nil {
		return nil, fmt.Errorf("scheme recognizes %s but failed to produce an object for it: %w", gvk, err)
	}

	raw, err := json.Marshal(u)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize %T: %w", raw, err)
	}
	if err := json.Unmarshal(raw, typed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal the content of %T into %T: %w", u, typed, err)
	}

	return typed, nil
}
