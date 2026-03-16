# XGBoost Training (XGBoostJob)

Using XGBoostJob to train a model with XGBoost

:::{admonition} Old Version
:class: warning
This page is about **Kubeflow Training Operator V1**, for the latest information check
[the Kubeflow Trainer V2 documentation](../../overview/index.md).

Follow [this guide for migrating to Kubeflow Trainer V2](../../operator-guides/migration.md).
:::

This page describes the `XGBoostJob` for training a machine learning model with [XGBoost](https://github.com/dmlc/xgboost).

`XGBoostJob` is a Kubernetes [custom resource](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) to run XGBoost training jobs on Kubernetes. The Kubeflow implementation of `XGBoostJob` is in the [training-operator](https://github.com/kubeflow/training-operator).

Note: `XGBoostJob` doesn't work in a user namespace by default because of Istio [automatic sidecar injection](https://istio.io/v1.3/docs/setup/additional-setup/sidecar-injection/#automatic-sidecar-injection). In order to get it running, it needs the annotation `sidecar.istio.io/inject: "false"` to disable it for either the `PyTorchJob` pods or namespace. To view an example of how to add this annotation to your `yaml` file, see the [TFJob documentation](tensorflow.md).

## Creating a XGBoost training job

You can create a training job by defining an `XGboostJob` config file. See the manifests for the [IRIS example](https://github.com/kubeflow/training-operator/blob/release-1.9/examples/xgboost/xgboostjob.yaml). You may change the config file based on your requirements. E.g.: add `CleanPodPolicy` in Spec to `None` to retain pods after job termination.

```bash
cat xgboostjob.yaml
```

Deploy the `XGBoostJob` resource to start training:

```bash
kubectl create -f xgboostjob.yaml
```

You should now be able to see the created pods matching the specified number of replicas.

```bash
kubectl get pods -l job-name=xgboost-dist-iris-test-train
```

Training takes 5-10 minutes on a cpu cluster. Logs can be inspected to see its training progress.

```bash
PODNAME=$(kubectl get pods -l job-name=xgboost-dist-iris-test-train,replica-type=master,replica-index=0 -o name)
kubectl logs -f ${PODNAME}
```

## Monitoring a XGBoostJob

```bash
kubectl get -o yaml xgboostjobs xgboost-dist-iris-test-train
```

See the status section to monitor the job status. Here is sample output when the job is successfully completed.

```yaml
apiVersion: kubeflow.org/v1
kind: XGBoostJob
metadata:
  creationTimestamp: "2021-09-06T18:34:06Z"
  generation: 1
  name: xgboost-dist-iris-test-train
  namespace: default
  resourceVersion: "5844304"
  selfLink: /apis/kubeflow.org/v1/namespaces/default/xgboostjobs/xgboost-dist-iris-test-train
  uid: a1ea6675-3cb5-482b-95dd-68b2c99b8adc
spec:
  runPolicy:
    cleanPodPolicy: None
  xgbReplicaSpecs:
    Master:
      replicas: 1
      restartPolicy: Never
      template:
        spec:
          containers:
            - args:
                - --job_type=Train
                - --xgboost_parameter=objective:multi:softprob,num_class:3
                - --n_estimators=10
                - --learning_rate=0.1
                - --model_path=/tmp/xgboost-model
                - --model_storage_type=local
              image: docker.io/merlintang/xgboost-dist-iris:1.1
              imagePullPolicy: Always
              name: xgboost
              ports:
                - containerPort: 9991
                  name: xgboostjob-port
                  protocol: TCP
    Worker:
      replicas: 2
      restartPolicy: ExitCode
      template:
        spec:
          containers:
            - args:
                - --job_type=Train
                - --xgboost_parameter="objective:multi:softprob,num_class:3"
                - --n_estimators=10
                - --learning_rate=0.1
              image: docker.io/merlintang/xgboost-dist-iris:1.1
              imagePullPolicy: Always
              name: xgboost
              ports:
                - containerPort: 9991
                  name: xgboostjob-port
                  protocol: TCP
status:
  completionTime: "2021-09-06T18:34:23Z"
  conditions:
    - lastTransitionTime: "2021-09-06T18:34:06Z"
      lastUpdateTime: "2021-09-06T18:34:06Z"
      message: xgboostJob xgboost-dist-iris-test-train is created.
      reason: XGBoostJobCreated
      status: "True"
      type: Created
    - lastTransitionTime: "2021-09-06T18:34:06Z"
      lastUpdateTime: "2021-09-06T18:34:06Z"
      message: XGBoostJob xgboost-dist-iris-test-train is running.
      reason: XGBoostJobRunning
      status: "False"
      type: Running
    - lastTransitionTime: "2021-09-06T18:34:23Z"
      lastUpdateTime: "2021-09-06T18:34:23Z"
      message: XGBoostJob xgboost-dist-iris-test-train is successfully completed.
      reason: XGBoostJobSucceeded
      status: "True"
      type: Succeeded
  replicaStatuses:
    Master:
      succeeded: 1
    Worker:
      succeeded: 2
```
