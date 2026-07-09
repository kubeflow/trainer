{{- /*
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
*/ -}}

{{/*
Name of the resources used by the built-in runtimes installer.
*/}}
{{- define "trainer.runtimes.installer.name" -}}
{{ include "trainer.fullname" . }}-runtimes-installer
{{- end -}}

{{/*
Labels for the built-in runtimes installer resources.
*/}}
{{- define "trainer.runtimes.installer.labels" -}}
{{ include "trainer.labels" . }}
app.kubernetes.io/part-of: kubeflow
app.kubernetes.io/component: runtimes-installer
{{- end -}}

{{/*
Returns "true" if at least one built-in runtime is enabled. Used to gate the
installer resources (ConfigMap, RBAC and Job) so that a minimal control-plane
install does not create unused objects.
NOTE: disabling every runtime removes the installer, so the last built-in
runtimes are not removed automatically. Delete them manually if required.
*/}}
{{- define "trainer.runtimes.enabled" -}}
{{- or .Values.runtimes.defaultEnabled
       .Values.runtimes.torchDistributed.enabled
       .Values.runtimes.deepspeedDistributed.enabled
       .Values.runtimes.mlxDistributed.enabled
       .Values.runtimes.jaxDistributed.enabled
       .Values.runtimes.xgboostDistributed.enabled
       .Values.runtimes.torchtuneDistributed.llama3_2_1B.enabled
       .Values.runtimes.torchtuneDistributed.llama3_2_3B.enabled
       .Values.runtimes.torchtuneDistributed.qwen2_5_1_5B.enabled
       .Values.dataCache.runtimes.torchDistributedWithCache.enabled -}}
{{- end -}}

{{/*
Concatenates every enabled built-in runtime (sourced from files/runtimes/*.yaml)
into a multi-document YAML stream. Each file is rendered with `tpl` so that the
runtime image overrides resolve. The result is embedded into the installer
ConfigMap and applied by the installer Job.
*/}}
{{- define "trainer.runtimes.manifests" -}}
{{- if and .Values.dataCache.runtimes.torchDistributedWithCache.enabled (not .Values.dataCache.enabled) }}
{{- fail "dataCache.runtimes.torchDistributedWithCache.enabled requires dataCache.enabled to be true" }}
{{- end }}
{{- if or .Values.runtimes.torchDistributed.enabled .Values.runtimes.defaultEnabled }}
---
{{ tpl (.Files.Get "files/runtimes/torch-distributed.yaml") . }}
{{- end }}
{{- if or .Values.runtimes.deepspeedDistributed.enabled .Values.runtimes.defaultEnabled }}
---
{{ tpl (.Files.Get "files/runtimes/deepspeed-distributed.yaml") . }}
{{- end }}
{{- if or .Values.runtimes.mlxDistributed.enabled .Values.runtimes.defaultEnabled }}
---
{{ tpl (.Files.Get "files/runtimes/mlx-distributed.yaml") . }}
{{- end }}
{{- if or .Values.runtimes.jaxDistributed.enabled .Values.runtimes.defaultEnabled }}
---
{{ tpl (.Files.Get "files/runtimes/jax-distributed.yaml") . }}
{{- end }}
{{- if or .Values.runtimes.xgboostDistributed.enabled .Values.runtimes.defaultEnabled }}
---
{{ tpl (.Files.Get "files/runtimes/xgboost-distributed.yaml") . }}
{{- end }}
{{- if or .Values.runtimes.torchtuneDistributed.llama3_2_1B.enabled .Values.runtimes.defaultEnabled }}
---
{{ tpl (.Files.Get "files/runtimes/torchtune-llama3.2-1b.yaml") . }}
{{- end }}
{{- if or .Values.runtimes.torchtuneDistributed.llama3_2_3B.enabled .Values.runtimes.defaultEnabled }}
---
{{ tpl (.Files.Get "files/runtimes/torchtune-llama3.2-3b.yaml") . }}
{{- end }}
{{- if or .Values.runtimes.torchtuneDistributed.qwen2_5_1_5B.enabled .Values.runtimes.defaultEnabled }}
---
{{ tpl (.Files.Get "files/runtimes/torchtune-qwen2.5-1.5b.yaml") . }}
{{- end }}
{{- if and .Values.dataCache.enabled .Values.dataCache.runtimes.torchDistributedWithCache.enabled }}
---
{{ tpl (.Files.Get "files/runtimes/torch-distributed-with-cache.yaml") . }}
{{- end }}
{{- end -}}
