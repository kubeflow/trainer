- [Background](#background)
- [Motivation](#motivation)
- [Goals](#goals)
- [Non-Goals](#non-goals)
- [Design](#design)
  - [Container images and deploying FATE cluster on
    Kubernetes](#container-images-and-deploying-fate-cluster-on-kubernetes)
  - [Custom Resource Definition](#custom-resource-definition)
    - [Kubefate](#kubefate)
    - [FATECluster](#fatecluster)
    - [FateJob](#fatejob)
  - [Controller design](#controller-design)
- [Reference](#reference)

_Status_

- 2020-6-3 - Draft v3
- 2020-5-28 - Draft v2
- 2020-5-14 – Draft v1

# KEP-335: Fate Operator

## Background

Federated machine learning (FML) is a machine learning setting where many clients (e.g. mobile devices or organizations) collaboratively train a model under the coordination of a central server while keeping the training data decentralized. Only the encrypted mediate parameters are exchanged between clients with MPC or homomorphic encryption.
![Federated Machine Learning](fate-operator-fl.png)

FML has received significant interest recently, because of its effectiveness to solve data silos and data privacy preserving problems. Companies participated in federated machine learning include 4Paradigm, Ant Financial, Data Republic, Google, Huawei, Intel, JD.com, Microsoft, Nvidia, OpenMind, Pingan Technology, Sharemind, Tencent, VMware, Webank etc.

Depending on the differences in features and sample data space, federated machine learning can be classified into _horizontal federated machine learning_, _vertical federated machine learning_ and _federated transfer learning_. Horizontal federated machine learning is also called sample-based federated machine learning, which means data sets share the same feature space but have different samples. With horizontal federated machine learning, we can gather the relatively small or partial datasets into a big one to increase the performance of trained models. Vertical federated machine learning is applicable to the cases where there are two datasets with different feature space but share same sample ID. With vertical federated machine learning we can train a model with attributes from different organizations for a full profile. Vertical federated machine learning is required to redesign most of machine learning algorithms. Federated transfer learning applies to scenarios where there are two datasets with different features space but also different samples.

[FATE (Federated AI Technology Enabler)](https://fate.fedai.org) is an open source project initialized by Webank, [now hosted at the Linux Foundation](https://fate.fedai.org/2019/09/18/first-digital-only-bank-in-china-joins-linux-foundation/). FATE is the only open source FML framework that supports both horizontal and vertical FML currently. The architecture design of FATE is focused on providing FML platform for enterprises. [KubeFATE](https://github.com/FederatedAI/KubeFATE) is an open source project to deploy FATE on Kubernetes and is a proven effective solution for FML use cases.

More technologies of Federated machine learning, please refer to [Reference section](#reference)

## Case studies

There are 130+ enterprises and organizations, 150+ colleges participating in FATE project. FATE case studies refer to [Cases](https://www.fedai.org/cases/):

1. [Utilization of FATE in Risk Management of Credit in Small and Micro Enterprises](https://www.fedai.org/cases/utilization-of-fate-in-risk-management-of-credit-in-small-and-micro-enterprises/)
2. [Computer vision Platform powered by Federated Learning](https://www.fedai.org/cases/computer-vision-platform-powered-by-federated-learning/)
3. [A case of traffic violations insurance-using federated learning](https://www.fedai.org/cases/a-case-of-traffic-violations-insurance-using-federated-learning/)
4. [Utilization of FATE in Anti Money Laundering Through Multiple Banks](https://www.fedai.org/cases/utilization-of-fate-in-anti-money-laundering-through-multiple-banks/)

Other Federated Machine Learning cases:

1. [Federated Learning for Mobile Keyboard Prediction](https://research.google/pubs/pub47586/)
2. [Federated Learning powered by NVIDIA Clara](https://devblogs.nvidia.com/federated-learning-clara/): hospitals and medical institutions collaboratively share and combine their local knowledge
3. [Owkin Launches the Collaborative COVID-19 Open AI Consortium (COAI)](https://www.unite.ai/covid-19-open-ai-consortium/)

## Motivation

Kubeflow provides a toolset for end-to-end machine learning workflow on Kubernetes. Introducing the capability of federated learning to Kubeflow helps FML users and researchers leverage existing Kubeflow toolkits in their workflows and help them more efficiently build FML solutions.

A FATE-Operator is a start of supporting FML in Kubeflow. This proposal is aimed to defining what FATE operator should look like, and how to apply to Kubeflow.

## Goals

A Kubeflow user should be able to run training using FATE as easily as they can using PyTorch, Tensorflow. This proposal is centered around a Kubernetes Operator for FATE. With the FATE-Operator, a user can:

1. Provision and manage a FATE cluster;
2. Submit an FML job to FATE.

This proposal defines the following:

1. A FATE operator with three CRDs:

- FateJob: create an FML job;
- FateCluster: create a FATE cluster to serve FML jobs;
- Kubefate: the resource management component of FATE cluster.

2. Example of full lifecycle to create KubeFATE component, deploy FATE cluster and submit an FML job to created FATE and get the result. Note that, KubeFATE and FATE cluster only needs to be deployed once, and can handle multiple jobs.

## Non-Goals

For the scope of this proposal, we won’t be addressing the method of serving the model.

## Design

### Container images and deploying FATE cluster on Kubernetes

We have built a set of Docker images for FATE cluster, and put into: https://hub.docker.com/orgs/federatedai/repositories . All the images have been already used and verified by community users.

There is a provisioning and management component of FATE cluster, called [KubeFATE](https://github.com/FederatedAI/KubeFATE/tree/master/k8s-deploy). KubeFATE manages FATE clusters of one party in a federation.

All images work well and are proven in users’ environments.

### Custom Resource Definition

#### Kubefate

```
apiVersion: app.kubefate.net/v1beta1
kind: Kubefate
metadata:
  name: kubefate-sample
  namespace: kube-fate
spec:
  imageVersion: v1.0.3
  host: kubefate.net
  # kubefate config
  config:
    - name: FATECLOUD_MONGO_URL
      value: "mongo:27017"
    - name: FATECLOUD_MONGO_USERNAME
      valueFrom:
        secretKeyRef:
          name: kubefate-secret
          key: mongoUsername
    - name: FATECLOUD_MONGO_PASSWORD
      valueFrom:
        secretKeyRef:
          name: kubefate-secret
          key: mongoPassword
    - name: FATECLOUD_MONGO_DATABASE
      value: "KubeFate"
    - name: FATECLOUD_USER_USERNAME
      valueFrom:
        secretKeyRef:
          name: kubefate-secret
          key: kubefateUsername
    - name: FATECLOUD_USER_PASSWORD
      valueFrom:
        secretKeyRef:
          name: kubefate-secret
          key: kubefatePassword
```

KubeFATE is a core component to manage and coordinate FATE clusters in one FML party. The above CRD defines the KubeFATE component.

- host defines other components how to access the service service gateway of KubeFATE exposed.

#### FATECluster

```
apiVersion: app.kubefate.net/v1beta1
kind: FateCluster
metadata:
  name: fatecluster-sample
  namespace: fate-9999
spec:
  version: v1.4.0
  partyId: "9999"
  proxyPort: "30009"
  partyList:
  - partyId: "10000"
    partyIp: "192.168.1.10"
    partyPort: "30010"
  egg:
    replica: 1

  # KubeFATE service deployed in Org.
  kubefate:
    name: kubefate-sample
    namespace:  kube-fate
```

The FateCluster defines a deployment of FATE on Kubernetes.

- version defines the FATE version deployed in Kubernetes;
- partyId defines the FML party’s ID;
- proxyPort defines the exposed port for exchanging models and parameters between different parties in an FML training. It will be exposed as a node port;
- partyList defines the parties in a federation which take part in collaboratively learning;
- egg is the worker nodes of FATE.

#### FateJob

```
apiVersion: app.kubefate.net/v1beta1
kind: FateJob
metadata:
  name: fatejob-sample
  namespace: fate-9999
spec:
  # FATE cluster deployed in the Org.
  fateClusterRef: fatecluster-sample
  jobConf:
    pipeline: |-
      {
        "components": {
          "secure_add_example_0": {
            "module": "SecureAddExample"
          }
        }
      }
    modulesConfig: |-
      {
        "initiator": {
          "role": "guest",
          "party_id": 9999
        },
        "job_parameters": {
          "work_mode": 1
        },
        "role": {
          "guest": [
            9999
          ],
          "host": [
            9999
          ]
        },
        "role_parameters": {
          "guest": {
            "secure_add_example_0": {
              "seed": [
                123
              ]
            }
          },
          "host": {
            "secure_add_example_0": {
              "seed": [
                321
              ]
            }
          }
        },
        "algorithm_parameters": {
          "secure_add_example_0": {
            "partition": 10,
            "data_num": 1000
          }
        }
      }
```

FateJob defines the job sent to FATE cluster:

- fateClusterRef defines the cluster of FATE deployed on Kubernetes. Its value is resource name of FATE cluster created by CRD “FateCluster”;
- jobConf defines the details of an FML job. It includes:
  - pipeline: the workflow pipeline of FATE. In FATE, there are many prebuilt algorithm components (ref: https://github.com/FederatedAI/FATE/tree/master/federatedml and https://github.com/FederatedAI/FATE/tree/master/federatedrec), which can be used to train models. The pipeline defines how data are passed through and processed in the whole training flow;
  - moduleConf: the detail configuration of each algorithm component, e.g. the optimizers, the batch size etc.

### Controller design

We created a new custom controller for FateCluster, FateJob and Kubefate resources.

The relationship between them and processes to make everything work are shown as following diagrams.

Process 1. Creating Kubefate if it does not exist. The custom controller (1) listens for Kubefate resource, (2) creates RBAC resources (Role, Service Account, RoleBinding) to allow remote execution, (3) creates the related resource of Kubefate. (4) The controller waits the resource of Kubefate to be ready and returns the status.

![Process 1](fate-operator-1.png)

Process 2. Creating FATE cluster. In one party, only one Kubefate instance needs to be provisioned. Multiple FATE clusters can be provisioned by KubeFATe for different purposes. (1) The custom FATE controller listens for FateCluster custom resource, (2) and calls Kubefate cluster of the one federated party, sends the metadata and configurations (3) to create a FATE cluster. (4) The controller waits the resource of FATE to be ready and returns the status.

![Process 2](fate-operator-2.png)

Process 3. Submitting an FML job to FATE cluster. (1) The custom FATE controller listens for FateJob CRD, and sends the job to FATE cluster, which includes the pipeline and modules configuration. (3) The FATE controller waits for the job results from FATE cluster.

![Process 3](fate-operator-3.png)

The overall architecture of the federated learning can be presented as following diagram, the FATE cluster will handle the communication and return to FATE controller once the job is done.

![Overall](fate-operator-overall.png)

## Reference

1. Qiang Yang, Yang Liu, Tianjian Chen, and Yongxin Tong. Federated machine learning: Concept and applications. CoRR, abs/1902.04885, 2019. URL http://arxiv.org/abs/1902.04885
2. Peter Kairouz et al. Advances and open problems in federated learning. arXiv preprint arXiv:1912.04977
