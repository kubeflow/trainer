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
	"iter"
	"slices"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/kubeflow/trainer/pkg/constants"
	corev1 "k8s.io/api/core/v1"
)

var (
	PodSetEndpointsCmpOpts = cmp.Transformer("Seq", func(a iter.Seq[string]) []string { return slices.Collect(a) })
)

func MPISecretDataComparer(a, b map[string][]byte) bool {
	isKeysEqual := true
	if (a != nil && b != nil) &&
		((len(a[constants.MPISSHPublicKey]) > 0) != (len(b[constants.MPISSHPublicKey]) > 0) ||
			(len(a[corev1.SSHAuthPrivateKey]) > 0) != (len(b[corev1.SSHAuthPrivateKey]) > 0)) {
		isKeysEqual = false
	}
	return isKeysEqual && cmp.Equal(a, b, cmpopts.IgnoreMapEntries(func(k string, _ []byte) bool {
		return k == constants.MPISSHPublicKey || k == corev1.SSHAuthPrivateKey
	}))
}
