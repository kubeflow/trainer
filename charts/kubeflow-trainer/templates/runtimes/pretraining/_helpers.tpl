{{/*
Copyright 2024 The Kubeflow authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/}}

{{- define "trainer.runtime.preTraining.torchDistributed.image" -}}
{{- $imageRegistry := .Values.runtime.preTraining.torchDistributed.image.registry | default "docker.io" }}
{{- $imageRepository := .Values.runtime.preTraining.torchDistributed.image.repository }}
{{- $imageTag := .Values.runtime.preTraining.torchDistributed.image.tag | default "latest" }}
{{- if eq $imageRepository "docker.io" }}
{{- printf "%s:%s" $imageRepository $imageTag }}
{{- else }}
{{- printf "%s/%s:%s" $imageRegistry $imageRepository $imageTag }}
{{- end }}
{{- end -}}

{{- define "trainer.runtime.preTraining.mpiDistributed.image" -}}
{{- $imageRegistry := .Values.runtime.preTraining.mpiDistributed.image.registry | default "docker.io" }}
{{- $imageRepository := .Values.runtime.preTraining.mpiDistributed.image.repository }}
{{- $imageTag := .Values.runtime.preTraining.mpiDistributed.image.tag | default "latest" }}
{{- if eq $imageRepository "docker.io" }}
{{- printf "%s:%s" $imageRepository $imageTag }}
{{- else }}
{{- printf "%s/%s:%s" $imageRegistry $imageRepository $imageTag }}
{{- end }}
{{- end -}}
