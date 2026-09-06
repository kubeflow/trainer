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

// Command crdschema restores the schema bounds controller-gen drops from the
// generated CRDs and fails if the result would be rejected by the API server.
// It runs from the manifests Makefile target, right after controller-gen.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/kubeflow/trainer/v2/pkg/util/crdschema"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		log.Fatal("usage: crdschema <crd-manifest>...")
	}
	for _, path := range os.Args[1:] {
		manifest, err := os.ReadFile(path)
		if err != nil {
			log.Fatal(err)
		}
		patched, added, err := crdschema.Patch(manifest)
		if err != nil {
			log.Fatalf("%s: %v", path, err)
		}
		if added == 0 {
			continue
		}
		if err = os.WriteFile(path, patched, 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s: restored %d schema bound(s) dropped by controller-gen\n", path, added)
	}
}
