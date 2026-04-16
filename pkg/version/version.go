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

// Package version holds version information for the Kubeflow Trainer controller manager.
// Variables are set at build time via -ldflags:
//
//	-ldflags "-X github.com/kubeflow/trainer/v2/pkg/version.GitVersion=v1.0.0
//	          -X github.com/kubeflow/trainer/v2/pkg/version.GitCommit=abc123
//	          -X github.com/kubeflow/trainer/v2/pkg/version.BuildDate=2024-01-01T00:00:00Z"
package version

// These variables are populated at build time via -ldflags.
// They default to "unknown" so the build_info metric is always present.
var (
	GitVersion = "unknown"
	GitCommit  = "unknown"
	BuildDate  = "unknown"
)
