package constants

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
)

const (

	// DefaultJobReplicas is the default value for the ReplicatedJob replicas.
	DefaultJobReplicas = 1

	// JobSetKind is the Kind name for the JobSet.
	JobSetKind string = "JobSet"

	// JobTrainerNode is the Job name for the trainer node.
	JobTrainerNode string = "trainer-node"

	// ContainerTrainer is the container name for the trainer.
	ContainerTrainer string = "trainer"

	// ContainerTrainerPort is the default port for the trainer nodes communication.
	ContainerTrainerPort int32 = 29500

	// JobInitializer is the Job name for the initializer.
	JobInitializer string = "initializer"

	// ContainerModelInitializer is the container name for the model initializer.
	ContainerModelInitializer string = "model-initializer"

	// ContainerDatasetInitializer is the container name for the dataset initializer.
	ContainerDatasetInitializer string = "dataset-initializer"

	// PodGroupKind is the Kind name for the PodGroup.
	PodGroupKind string = "PodGroup"

	// Distributed envs for torchrun.
	// Ref: https://github.com/pytorch/pytorch/blob/3a0d0885171376ed610c8175a19ba40411fc6f3f/torch/distributed/argparse_util.py#L45
	// TorchEnvNumNodes is the env name for the number of training nodes.
	TorchEnvNumNodes string = "PET_NNODES"

	// TorchEnvNumProcPerNode is the env name for the number of procs per node (e.g. number of GPUs per Pod).
	TorchEnvNumProcPerNode string = "PET_NPROC_PER_NODE"

	// TorchEnvNodeRank is the env name for the node RANK
	TorchEnvNodeRank string = "PET_NODE_RANK"

	// TorchEnvMasterAddr is the env name for the master node address.
	TorchEnvMasterAddr string = "PET_MASTER_ADDR"

	// TorchEnvMasterPort is the env name for the master node port.
	TorchEnvMasterPort string = "PET_MASTER_PORT"
)

var (
	// JobCompletionIndexFieldPath is the field path for the Job completion index annotation.
	JobCompletionIndexFieldPath string = fmt.Sprintf("metadata.annotations['%s']", batchv1.JobCompletionIndexAnnotation)
)
