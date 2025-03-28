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

{{/*
Create the name of the controller service account.
*/}}
{{- define "trainer.manager.serviceAccount.name" -}}
{{ include "trainer.manager.name" . }}
{{- end -}}

{{/*
Create the name of the controller cluster role.
*/}}
{{- define "trainer.manager.clusterRole.name" -}}
{{ include "trainer.manager.name" . }}
{{- end -}}

{{/*
Create the name of the controller cluster role binding.
*/}}
{{- define "trainer.manager.clusterRoleBinding.name" -}}
{{ include "trainer.manager.name" . }}
{{- end -}}

{{/*
Create the name of the controller role.
*/}}
{{- define "trainer.manager.role.name" -}}
{{ include "trainer.manager.name" . }}
{{- end -}}

{{/*
Create the name of the controller role binding.
*/}}
{{- define "trainer.manager.roleBinding.name" -}}
{{ include "trainer.manager.name" . }}
{{- end -}}
