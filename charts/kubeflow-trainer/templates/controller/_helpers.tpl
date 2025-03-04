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
Create the name of the controller.
*/}}
{{- define "trainer.controller.name" -}}
{{ include "trainer.fullname" . }}-controller-manager
{{- end -}}

{{/*
Common labels for the controller.
*/}}
{{- define "trainer.controller.labels" -}}
{{ include "trainer.labels" . }}
app.kubernetes.io/part-of: kubeflow
app.kubernetes.io/component: controller
{{- with .Values.controller.labels }}
{{- toYaml . | nindent 0 }}
{{- end }}
{{- end -}}

{{/*
Selector labels for the controller.
*/}}
{{- define "trainer.controller.selectorLabels" -}}
{{ include "trainer.selectorLabels" . }}
app.kubernetes.io/part-of: kubeflow
app.kubernetes.io/component: controller
{{- with .Values.controller.labels }}
{{- toYaml . | nindent 0 }}
{{- end }}
{{- end -}}

{{/*
Create the name of the controller deployment.
*/}}
{{- define "trainer.controller.deployment.name" -}}
{{ include "trainer.controller.name" . }}
{{- end -}}

{{- define "trainer.controller.service.name" -}}
{{ include "trainer.controller.name" . }}
{{- end -}}
