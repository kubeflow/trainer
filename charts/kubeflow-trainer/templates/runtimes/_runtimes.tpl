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
runtimes are not pruned automatically. Delete them manually if required.
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
Renders every enabled built-in ClusterTrainingRuntime as a multi-document YAML
stream. The output is embedded into the installer ConfigMap and applied by the
installer Job with `kubectl apply --server-side`. Each runtime carries the
`trainer.kubeflow.org/managed-by: runtimes-installer` label so that the Job can
scope `--prune` to only the runtimes it owns.
*/}}
{{- define "trainer.runtimes.manifests" -}}
{{- if and .Values.dataCache.runtimes.torchDistributedWithCache.enabled (not .Values.dataCache.enabled) }}
{{- fail "dataCache.runtimes.torchDistributedWithCache.enabled requires dataCache.enabled to be true" }}
{{- end }}
{{- if or .Values.runtimes.torchDistributed.enabled .Values.runtimes.defaultEnabled }}
---
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-distributed
  labels:
    trainer.kubeflow.org/framework: torch
    trainer.kubeflow.org/managed-by: runtimes-installer
    {{- with .Values.runtimes.commonLabels }}
    {{- toYaml . | nindent 4 }}
    {{- end }}
    {{- include "trainer.labels" . | nindent 4 }}
spec:
  mlPolicy:
    numNodes: 1
    torch: {}
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: trainer
            spec:
              template:
                spec:
                  containers:
                    - name: node
                      image: pytorch/pytorch:2.10.0-cuda12.8-cudnn9-runtime
{{- end }}
{{- if or .Values.runtimes.deepspeedDistributed.enabled .Values.runtimes.defaultEnabled }}
---
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: deepspeed-distributed
  labels:
    trainer.kubeflow.org/framework: deepspeed
    trainer.kubeflow.org/managed-by: runtimes-installer
    {{- with .Values.runtimes.commonLabels }}
    {{- toYaml . | nindent 4 }}
    {{- end }}
    {{- include "trainer.labels" . | nindent 4 }}
spec:
  mlPolicy:
    numNodes: 1
    mpi:
      numProcPerNode: 1
      mpiImplementation: OpenMPI
      sshAuthMountPath: /home/mpiuser/.ssh
      runLauncherAsNode: true
  template:
    spec:
      network:
        publishNotReadyAddresses: true
      successPolicy:
        operator: All
        targetReplicatedJobs:
          - launcher
      replicatedJobs:
        - name: launcher
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: trainer
            spec:
              template:
                spec:
                  containers:
                    - name: node
                      image: {{ include "trainer.runtimeImage" (list .Values.runtimes.deepspeedDistributed.image .) }}
                      securityContext:
                        runAsUser: 1000
        - name: node
          template:
            spec:
              template:
                spec:
                  containers:
                    - name: node
                      image: {{ include "trainer.runtimeImage" (list .Values.runtimes.deepspeedDistributed.image .) }}
                      securityContext:
                        runAsUser: 1000
                      command:
                        - /usr/sbin/sshd
                      args:
                        - -De
                        - -f
                        - /home/mpiuser/.sshd_config
                      readinessProbe:
                        tcpSocket:
                          port: 2222
                        initialDelaySeconds: 5
{{- end }}
{{- if or .Values.runtimes.mlxDistributed.enabled .Values.runtimes.defaultEnabled }}
---
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: mlx-distributed
  labels:
    trainer.kubeflow.org/framework: mlx
    trainer.kubeflow.org/managed-by: runtimes-installer
    {{- with .Values.runtimes.commonLabels }}
    {{- toYaml . | nindent 4 }}
    {{- end }}
    {{- include "trainer.labels" . | nindent 4 }}
spec:
  mlPolicy:
    numNodes: 1
    mpi:
      numProcPerNode: 1
      mpiImplementation: OpenMPI
      sshAuthMountPath: /home/mpiuser/.ssh
      runLauncherAsNode: true
  template:
    spec:
      network:
        publishNotReadyAddresses: true
      successPolicy:
        operator: All
        targetReplicatedJobs:
          - launcher
      replicatedJobs:
        - name: launcher
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: trainer
            spec:
              template:
                spec:
                  containers:
                    - name: node
                      image: {{ include "trainer.runtimeImage" (list .Values.runtimes.mlxDistributed.image .) }}
                      securityContext:
                        runAsUser: 1000
        - name: node
          template:
            spec:
              template:
                spec:
                  containers:
                    - name: node
                      image: {{ include "trainer.runtimeImage" (list .Values.runtimes.mlxDistributed.image .) }}
                      securityContext:
                        runAsUser: 1000
                      command:
                        - /usr/sbin/sshd
                      args:
                        - -De
                        - -f
                        - /home/mpiuser/.sshd_config
                      readinessProbe:
                        tcpSocket:
                          port: 2222
                        initialDelaySeconds: 5
{{- end }}
{{- if or .Values.runtimes.jaxDistributed.enabled .Values.runtimes.defaultEnabled }}
---
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: jax-distributed
  labels:
    trainer.kubeflow.org/framework: jax
    trainer.kubeflow.org/managed-by: runtimes-installer
    {{- with .Values.runtimes.commonLabels }}
    {{- toYaml . | nindent 4 }}
    {{- end }}
    {{- include "trainer.labels" . | nindent 4 }}
spec:
  mlPolicy:
    numNodes: 1
    jax: {}
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: trainer
            spec:
              template:
                spec:
                  containers:
                    - name: node
                      image: nvcr.io/nvidia/jax:25.10-py3
{{- end }}
{{- if or .Values.runtimes.xgboostDistributed.enabled .Values.runtimes.defaultEnabled }}
---
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: xgboost-distributed
  labels:
    trainer.kubeflow.org/framework: xgboost
    trainer.kubeflow.org/managed-by: runtimes-installer
    {{- with .Values.runtimes.commonLabels }}
    {{- toYaml . | nindent 4 }}
    {{- end }}
    {{- include "trainer.labels" . | nindent 4 }}
spec:
  mlPolicy:
    numNodes: 1
    xgboost: {}
  template:
    spec:
      replicatedJobs:
        - name: node
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: trainer
            spec:
              template:
                spec:
                  containers:
                    - name: node
                      image: {{ include "trainer.runtimeImage" (list .Values.runtimes.xgboostDistributed.image .) }}
{{- end }}
{{- if or .Values.runtimes.torchtuneDistributed.llama3_2_1B.enabled .Values.runtimes.defaultEnabled }}
---
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torchtune-llama3.2-1b
  labels:
    trainer.kubeflow.org/framework: torchtune
    trainer.kubeflow.org/managed-by: runtimes-installer
    {{- with .Values.runtimes.commonLabels }}
    {{- toYaml . | nindent 4 }}
    {{- end }}
    {{- include "trainer.labels" . | nindent 4 }}
spec:
  mlPolicy:
    numNodes: 1
    torch: {}
  template:
    spec:
      volumeClaimPolicies:
        - templates:
            - metadata:
                name: initializer
              spec:
                accessModes: ["ReadWriteOnce"]
                resources:
                  requests:
                    storage: 20Gi
      replicatedJobs:
        - name: dataset-initializer
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: dataset-initializer
            spec:
              template:
                spec:
                  containers:
                    - name: dataset-initializer
                      image: ghcr.io/kubeflow/trainer/dataset-initializer
                      env:
                        - name: STORAGE_URI
                          value: hf://tatsu-lab/alpaca
                      volumeMounts:
                        - mountPath: /workspace
                          name: initializer
        - name: model-initializer
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: model-initializer
            spec:
              template:
                spec:
                  containers:
                    - name: model-initializer
                      image: ghcr.io/kubeflow/trainer/model-initializer
                      env:
                        - name: STORAGE_URI
                          value: hf://meta-llama/Llama-3.2-1B-Instruct
                      volumeMounts:
                        - name: initializer
                          mountPath: /workspace
        - name: node
          dependsOn:
            - name: dataset-initializer
              status: Complete
            - name: model-initializer
              status: Complete
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: trainer
            spec:
              template:
                spec:
                  containers:
                    - name: node
                      image: {{ include "trainer.runtimeImage" (list .Values.runtimes.torchtuneDistributed.image .) }}
                      command:
                        - tune
                        - run
                        - --rdzv_endpoint=localhost:29500
                        - full_finetune_distributed
                        - --config
                        - llama3_2/1B_full
                        - dataset=torchtune.datasets.instruct_dataset
                        - dataset.source=parquet
                        - dataset.data_dir=/workspace/dataset/data
                        - output_dir=/workspace/output
                        - tokenizer.path=/workspace/model/original/tokenizer.model
                        - checkpointer.checkpoint_dir=/workspace/model
                      resources:
                        limits:
                          nvidia.com/gpu: 2
                      volumeMounts:
                        - mountPath: /workspace
                          name: initializer
{{- end }}
{{- if or .Values.runtimes.torchtuneDistributed.llama3_2_3B.enabled .Values.runtimes.defaultEnabled }}
---
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torchtune-llama3.2-3b
  labels:
    trainer.kubeflow.org/framework: torchtune
    trainer.kubeflow.org/managed-by: runtimes-installer
    {{- with .Values.runtimes.commonLabels }}
    {{- toYaml . | nindent 4 }}
    {{- end }}
    {{- include "trainer.labels" . | nindent 4 }}
spec:
  mlPolicy:
    numNodes: 1
    torch: {}
  template:
    spec:
      volumeClaimPolicies:
        - templates:
            - metadata:
                name: initializer
              spec:
                accessModes: ["ReadWriteOnce"]
                resources:
                  requests:
                    storage: 20Gi
      replicatedJobs:
        - name: dataset-initializer
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: dataset-initializer
            spec:
              template:
                spec:
                  containers:
                    - name: dataset-initializer
                      image: ghcr.io/kubeflow/trainer/dataset-initializer
                      env:
                        - name: STORAGE_URI
                          value: hf://tatsu-lab/alpaca
                      volumeMounts:
                        - mountPath: /workspace
                          name: initializer
        - name: model-initializer
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: model-initializer
            spec:
              template:
                spec:
                  containers:
                    - name: model-initializer
                      image: ghcr.io/kubeflow/trainer/model-initializer
                      env:
                        - name: STORAGE_URI
                          value: hf://meta-llama/Llama-3.2-3B-Instruct
                      volumeMounts:
                        - name: initializer
                          mountPath: /workspace
        - name: node
          dependsOn:
            - name: dataset-initializer
              status: Complete
            - name: model-initializer
              status: Complete
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: trainer
            spec:
              template:
                spec:
                  containers:
                    - name: node
                      image: {{ include "trainer.runtimeImage" (list .Values.runtimes.torchtuneDistributed.image .) }}
                      command:
                        - tune
                        - run
                        - --rdzv_endpoint=localhost:29500
                        - full_finetune_distributed
                        - --config
                        - llama3_2/3B_full
                        - dataset=torchtune.datasets.instruct_dataset
                        - dataset.source=parquet
                        - dataset.data_dir=/workspace/dataset/data
                        - output_dir=/workspace/output
                        - tokenizer.path=/workspace/model/original/tokenizer.model
                        - checkpointer.checkpoint_dir=/workspace/model
                      resources:
                        limits:
                          nvidia.com/gpu: 2
                      volumeMounts:
                        - mountPath: /workspace
                          name: initializer
{{- end }}
{{- if or .Values.runtimes.torchtuneDistributed.qwen2_5_1_5B.enabled .Values.runtimes.defaultEnabled }}
---
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torchtune-qwen2.5-1.5b
  labels:
    trainer.kubeflow.org/framework: torchtune
    trainer.kubeflow.org/managed-by: runtimes-installer
    {{- with .Values.runtimes.commonLabels }}
    {{- toYaml . | nindent 4 }}
    {{- end }}
    {{- include "trainer.labels" . | nindent 4 }}
spec:
  mlPolicy:
    numNodes: 1
    torch: {}
  template:
    spec:
      volumeClaimPolicies:
        - templates:
            - metadata:
                name: initializer
              spec:
                accessModes: ["ReadWriteOnce"]
                resources:
                  requests:
                    storage: 20Gi
      replicatedJobs:
        - name: dataset-initializer
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: dataset-initializer
            spec:
              template:
                spec:
                  containers:
                    - name: dataset-initializer
                      image: ghcr.io/kubeflow/trainer/dataset-initializer
                      env:
                        - name: STORAGE_URI
                          value: hf://tatsu-lab/alpaca
                      volumeMounts:
                        - mountPath: /workspace
                          name: initializer
        - name: model-initializer
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: model-initializer
            spec:
              template:
                spec:
                  containers:
                    - name: model-initializer
                      image: ghcr.io/kubeflow/trainer/model-initializer
                      env:
                        - name: STORAGE_URI
                          value: hf://Qwen/Qwen2.5-1.5B-Instruct
                      volumeMounts:
                        - name: initializer
                          mountPath: /workspace
        - name: node
          dependsOn:
            - name: dataset-initializer
              status: Complete
            - name: model-initializer
              status: Complete
          template:
            metadata:
              labels:
                trainer.kubeflow.org/trainjob-ancestor-step: trainer
            spec:
              template:
                spec:
                  containers:
                    - name: node
                      image: {{ include "trainer.runtimeImage" (list .Values.runtimes.torchtuneDistributed.image .) }}
                      command:
                        - tune
                        - run
                        - --rdzv_endpoint=localhost:29500
                        - full_finetune_distributed
                        - --config
                        - qwen2_5/1.5B_full
                        - dataset=torchtune.datasets.instruct_dataset
                        - dataset.source=parquet
                        - dataset.data_dir=/workspace/dataset/data
                        - output_dir=/workspace/output
                        - tokenizer.path=/workspace/model/vocab.json
                        - tokenizer.merges_file=/workspace/model/merges.txt
                        - checkpointer.checkpoint_dir=/workspace/model
                      resources:
                        limits:
                          nvidia.com/gpu: 2
                      volumeMounts:
                        - mountPath: /workspace
                          name: initializer
{{- end }}
{{- if and .Values.dataCache.enabled .Values.dataCache.runtimes.torchDistributedWithCache.enabled }}
---
apiVersion: trainer.kubeflow.org/v1alpha1
kind: ClusterTrainingRuntime
metadata:
  name: torch-distributed-with-cache
  labels:
    trainer.kubeflow.org/framework: torch
    trainer.kubeflow.org/managed-by: runtimes-installer
    {{- with .Values.runtimes.commonLabels }}
    {{- toYaml . | nindent 4 }}
    {{- end }}
    {{- include "trainer.labels" . | nindent 4 }}
spec:
  mlPolicy:
    numNodes: 1
    torch: {}
  template:
    spec:
      replicatedJobs:
      - name: dataset-initializer
        replicas: 1
        template:
          metadata:
            labels:
              trainer.kubeflow.org/trainjob-ancestor-step: dataset-initializer
          spec:
            template:
              spec:
                serviceAccountName: kubeflow-trainer-cache-initializer
                containers:
                  - name: dataset-initializer
                    image: {{ printf "ghcr.io/kubeflow/trainer/dataset-initializer:%s" (include "trainer.defaultImageTag" .) }}
                    env:
                      - name: CACHE_IMAGE
                        value: {{ include "trainer.runtimeImage" (list .Values.dataCache.cacheImage .) | quote }}
                      - name: TRAIN_JOB_NAME
                        valueFrom:
                          fieldRef:
                            apiVersion: v1
                            fieldPath: metadata.labels['jobset.sigs.k8s.io/jobset-name']
      - name: node
        dependsOn:
          - name: dataset-initializer
            status: Complete
        template:
          metadata:
            labels:
              trainer.kubeflow.org/trainjob-ancestor-step: trainer
          spec:
            template:
              spec:
                containers:
                  - name: node
                    image: pytorch/pytorch:2.10.0-cuda12.8-cudnn9-runtime
                    env:
                      - name: TRAIN_JOB_NAME
                        valueFrom:
                          fieldRef:
                            apiVersion: v1
                            fieldPath: metadata.labels['jobset.sigs.k8s.io/jobset-name']
{{- end }}
{{- end -}}
