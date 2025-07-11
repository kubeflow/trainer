# KEP-2432: GPU Testing for LLM Blueprints**

Project Page:[ GPU Testing for LLM Blueprints](https://www.kubeflow.org/events/gsoc-2025/#project-7-gpu-testing-for-llm-blueprints)

Issue:[ https://github.com/kubeflow/trainer/issues/2432](https://github.com/kubeflow/trainer/issues/2432)

Mentors:[ @andreyvelich](https://github.com/andreyvelich),[ @varodrig](https://github.com/varodrig)

Project Size: 350 hrs


## **Summary**

This project aims to use self-hosted runners to run GPU-intensive tasks like LLM blueprint or (planned) AI Playground. The necessary infra is provided by Oracle, plan is to use Oracle Kubernetes Engine (OKE) with NVIDIA GPUs for this task. Any code or sample that requires GPU-intensive resources will be transferred to OKE infra instead of generic GitHub infra for faster and more efficient execution.

For now, the idea is to have a specific policy that whenever any Jupyter Notebook code will be added to a `trainer/examples/pytorch/**` folder (e.g., in trainer/example/pytorch/image-generation/sample.ipynb), that action is transferred to OKE infra by the GitHub self runner. For security reasons, this process will require manual approval from one of the maintainers to trigger the self-runner build. I will set up the GitHub workflow to monitor changes in the respective folder. Once approved, the CI action will execute the code using the GitHub self-runner on the OKE infrastructure. Additionally, we will set up a dashboard for monitoring and metrics to understand usage patterns and identify bottlenecks.

The scope of this project is set up on OKE, but theoretically, this is platform-agnostic; it can be deployed on any Kubernetes cluster with sufficient GPU resources.

## **Motivation**

I have been active in open source since my college days, and I like to experiment and contribute to open source in my free time. I have been past GSoCer as well. In my work at Oracle, internally, I was tasked to develop a PoC to leverage k8s to run CI/CD pipeline. We were looking to efficiently configure/allocate resources in OKE to run CI/CD pipeline, and that’s where I got to know about Volcano's batch scheduling. I started reading about Volcano and in turn, KubeFlow. I started reading and contributing to MLOps and KubeFlow.

Then I started joining community calls of KubeFlow, that's where I told Chase Christensen that I work at Oracle. He motivated me to learn and contribute to Oracle distro of KubeFlow. I also had meet with Andrey, Francisco and Victor about the plan for Oracle to donate GPU infra to KubeFlow.

After my little [experimentation](https://github.com/kubeflow/trainer/issues/2432#issuecomment-2766243340) and research on [trainer/issues/2432](https://github.com/kubeflow/trainer/issues/2432), I am confident to contribute to this project as a GSoC contributor.

### **Goals**

- Set up a sample LLM Blueprint

- Configure GPU nodes on OKE

- Establish ACR on the OKE Cluster for deploying the LLM

- Create a GitHub Action for manual triggers and runners on the OKE cluster

- Implement metrics and analytics for the GPU Cluster

- Develop an AI Playground on OKE


### **Non-Goals**

1. The GPU cluster for production deployment should be provided by Oracle. For testing purposes, I have a sufficiently powerful personal machine (Ryzen 7 8600G, 32GB RAM, Nvidia RTX 4060) to conduct tests.

2. Once the infrastructure for the self-runner is set up, running the AI Playground will require minimal setup. The primary focus of this project is to establish the infrastructure for running the LLM blueprint on OKE. The AI Playground is a secondary priority for this GSoC project, but I will continue working on it if it is not completed within the GSoC period.


## **Proposal**

### **TechStack**

GitHub Actions (and [ARC](https://docs.github.com/en/actions/hosting-your-own-runners/managing-self-hosted-runners-with-actions-runner-controller/about-actions-runner-controller)), Kubernetes, [Oracle Cloud](https://www.oracle.com/in/cloud/cloud-native/kubernetes-engine/), PyTorch, Python, Linux

### User Stories

 - Run GPU intensive tasks on self hosted gpu infra (here OKE) instead of default CPU based infra

- [Planned] Run AI playground during KubeCon or other events leveraging full potential of KubeFlow Ecosystem
  
    The idea is to automate and setup sample models where user can just Open Kubeflow Jupyter Notebook -> select Kubeflow LLM blueprint -> fine-tune model with Kubeflow Trainer -> serve it with Kubeflow KServe. This will help us to show full potential of KubeFlow ecosystem.



## Design Details

### **Setup LLM Blueprint**

To set up the same LLM blueprint that can be triggered based on admin approval. We have already one sample on in trainer repo,[ here](https://github.com/kubeflow/trainer/blob/master/examples/deepspeed/text-summarization/T5-Fine-Tuning.ipynb) I have tested a sample project for running on my local system. I will be adding 2-3 base more samples with different requirements for our testing.

These additional samples will be designed to cover a range of scenarios and configurations, thereby enhancing the versatility and applicability of the LLM blueprint. This approach will not only facilitate thorough testing but also provide valuable insights into optimizing the deployment and execution of LLMs on the OKE infrastructure.

**Github Action**

Create a GitHub action for checking changes in files in `trainer/example/self-runner` and wait to trigger the self-runner after approval from the maintainers. Once the maintainer approves the scan, the code is executed in the self-runner (that is OKE infra). Assuming it takes some time and resources, we will implement queuing so that resources don't get flooded with requests. We will maintain a queue for requests, and report the result back to CI accordingly.

Here is the branch -[ test-self-runner](https://github.com/jaiakash/trainer/tree/test-runner)

Screenshot: 


### **Setup and access control of OKE Cluster with GPU**

In this milestone, the aim is to setup an OKE Cluster with GPU node. System: Ubuntu 22.04 LTS The GPU image has the GPU drivers pre-installed.

**Cluster Architecture:** The OKE cluster will be configured with:

• GPU Node Pool utilizing NVIDIA GPUs with pre-installed drivers

• Standard Node Pool for regular workloads (Metrics and Queue)

• Managed Control Plane by OCI

![](https://lh7-rt.googleusercontent.com/docsz/AD_4nXeOXc3VOVXwuZewMJUcA4562f6KApMbhJ0x9UPfTBEgz4bs6XeLwM8TX1-ojWPKHDUa1hVDnQu9ykL6mMmUSpAn2ePSPYA9ZfUFYLL6IYgaeAGaZipoePC7n55AyKOF2lUfhkkHow?key=nSj5OwtjFXw0peMUG5DbZheN)

Accessing Cluster

1. OKE can be accessed with `kubeconfig` file

2. Or via Bastian

Access control for the OKE cluster

- Cluster Administrators: Full cluster management rights

- Maintainers: Limited administrative access

- CI/CD Systems: Restricted [service account](https://kubernetes.io/docs/concepts/security/service-accounts/) access

![](https://lh7-rt.googleusercontent.com/docsz/AD_4nXfV3rML2L4ckFTSQ_Pujaa9X9ywZ0qfm1wysrjarWx-7bA5dPBZMBGIL3NzpMiAuEOumBQa-f3X1yRf-qOUppKT7L6WWAStmJ1POKWGKuyrzPVqiWZ1MQ5lR_sqVjVZZiTz_-xO8w?key=nSj5OwtjFXw0peMUG5DbZheN)

Images for NVIDIA shapes

- [GPU driver 570 & CUDA 12.8](https://objectstorage.ca-montreal-1.oraclecloud.com/p/ts6fjAuj7hY4io5x_jfX3fyC70HRCG8-9gOFqAjuF0KE0s-6tgDZkbRRZIbMZmoN/n/hpc_limited_availability/b/images/o/Canonical-Ubuntu-22.04-2024.10.04-0-OCA-OFED-24.10-1.1.4.0-GPU-570-CUDA-12.8-2025.03.26-0)

- [GPU driver 560 & CUDA 12.6](https://objectstorage.ca-montreal-1.oraclecloud.com/p/ts6fjAuj7hY4io5x_jfX3fyC70HRCG8-9gOFqAjuF0KE0s-6tgDZkbRRZIbMZmoN/n/hpc_limited_availability/b/images/o/Canonical-Ubuntu-22.04-2024.10.04-0-OCA-OFED-24.10-1.1.4.0-GPU-560-CUDA-12.6-2025.03.26-0)

- [GPU driver 550 & CUDA 12.4](https://objectstorage.ca-montreal-1.oraclecloud.com/p/ts6fjAuj7hY4io5x_jfX3fyC70HRCG8-9gOFqAjuF0KE0s-6tgDZkbRRZIbMZmoN/n/hpc_limited_availability/b/images/o/Canonical-Ubuntu-22.04-2024.10.04-0-OCA-OFED-24.10-1.1.4.0-GPU-550-CUDA-12.4-2025.03.26-0)

**Reference**

- <https://github.com/oracle-quickstart/oci-hpc-oke>

* <https://blogs.oracle.com/java/post/create-k8s-clusters-and-deply-to-oci-from-vscode>


### **Setup GitHub Actions Runner Controller (ARC)**

[Actions Runner Controller (ARC)](https://docs.github.com/en/actions/hosting-your-own-runners/managing-self-hosted-runners-with-actions-runner-controller/about-actions-runner-controller) is a Kubernetes operator that orchestrates and scales self-hosted runners for GitHub Actions. This is advanced phase of our project where we use k8s operator that is useful to scale and orchestrate pods based on the action CI.

![](https://lh7-rt.googleusercontent.com/docsz/AD_4nXfCFFFBFy4Eq_2zeFb7dhnA9QO_eFcas4vUIXhI2SZXF2RDvViMzM1gQX0vAEn1MEAs9xvEQ4zZUk8J_vtBYIqnyXkIkcC_bsQTpF01ix6ZNa2d4umWLpngHaqyWOOjNF-NSL0S?key=nSj5OwtjFXw0peMUG5DbZheN)


### **OKE Monitoring **

For admins, we also need to maintain monitoring to see the metrics and resource utilisation of the OKE infra. Oracle already provides an open-source sample for[ OKE Monitoring](https://github.com/oracle-quickstart/oci-kubernetes-monitoring), so we can leverage that. Out of various options, installation via[ Helm](https://github.com/oracle-quickstart/oci-kubernetes-monitoring#helm) is sufficient for our basic needs.

Estimated monthly cost: **$0/month**

Metrics needed

|                |                  |
| -------------- | ---------------- |
| Avg CPU Usage  | Avg Queue timing |
| Avg GPU Usage  | Avg Build timing |
| Peak GPU Usage |                  |


![](https://lh7-rt.googleusercontent.com/docsz/AD_4nXd7BOhUArK4CqKr17wrU2HgQlHWFlEq323slgMT9A6KQ75ALaucLgS6CpBqejeLKOvRNEp8UwOOZ0P7dEccGvnvwOe6_8N_5aLXUo06_YuwUB9mJ8F43LTu1XJUs1vv1Wp1ZjZv8A?key=nSj5OwtjFXw0peMUG5DbZheN)


### **AI Playground**

This is the final phase of the project, with LLM CI deployment, as in various KubeCons various users wanted to deploy a model quickly to test and run KubeFlow. The idea is to setup sample models where user can Open Kubeflow Jupyter Notebook -> select Kubeflow LLM blueprint -> fine-tune model with Kubeflow Trainer -> serve it with Kubeflow KServe. We will set up similar GitHub action for it during events. Also we need to implement safeguards to prevent misuse, given our limited GPU infrastructure. One option is to implement GitHub OAuth and provide access on an as-needed basis during KubeCon.


## **Test Plan**

During the GSoC period, until OKE infra is donated to KubeFlow. I will be testing local machine with 32GB RAM, Nvidia RTX 4060 GPU, Ryzen 7 8700G. I will have a demo during mid term evaluation in community call, once that is finalized by mentors. 

I will be using OKE to deploy after production. To make sure there is no unnecessary usage of infra while testing, i will be putting certain guardrails on prod OKE.

## **Reference**

Oracle Docs[ https://oracle.github.io/fmw-kubernetes/wccontent-domains/oracle-cloud/prepare-oke-environment/ https://github.com/oracle/weblogic-kubernetes-operator/blob/main/kubernetes/hands-on-lab/tutorials/setup.oke.ocishell.md](https://oracle.github.io/fmw-kubernetes/wccontent-domains/oracle-cloud/prepare-oke-environment/)

KubeFlow Docs[ https://www.kubeflow.org/docs/components/trainer/getting-started/ https://www.kubeflow.org/docs/started/architecture/](https://www.kubeflow.org/docs/components/trainer/getting-started/)

GitHub ACR Docs[ https://docs.github.com/en/actions/hosting-your-own-runners/managing-self-hosted-runners-with-actions-runner-controller/about-actions-runner-controller](https://docs.github.com/en/actions/hosting-your-own-runners/managing-self-hosted-runners-with-actions-runner-controller/about-actions-runner-controller)

GitHub Self Runner Docs[ https://docs.github.com/en/actions/hosting-your-own-runners/managing-self-hosted-runners/adding-self-hosted-runners](https://docs.github.com/en/actions/hosting-your-own-runners/managing-self-hosted-runners/adding-self-hosted-runners)

Claude 3.5 Sonnet - For formatting text and improving this proposal

Diagram -[ https://app.eraser.io/](https://app.eraser.io/)

Thanks for helping and guidance[ @andreyvelich](https://github.com/andreyvelich),[ @varodrig @thesuperzapper](https://github.com/varodrig),[ @chasecadet](https://github.com/chasecadet)
