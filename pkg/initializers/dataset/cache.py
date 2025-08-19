import logging
import time
from typing import Optional

from kubernetes import client, config
from kubernetes.client.rest import ApiException

import pkg.initializers.types.types as types
import pkg.initializers.utils.utils as utils

logging.basicConfig(
    format="%(asctime)s %(levelname)-8s [%(filename)s:%(lineno)d] %(message)s",
    datefmt="%Y-%m-%dT%H:%M:%SZ",
    level=logging.INFO,
)


class CacheInitializer(utils.DatasetProvider):

    def load_config(self):
        config_dict = utils.get_config_from_env(types.CacheDatasetInitializer)
        self.config = types.CacheDatasetInitializer(**config_dict)

    def download_dataset(self):
        logging.info(
            f"Cache initializer called with storage URI: {self.config.storage_uri}"
        )

        train_job_name = self.config.train_job_name
        cache_image = self.config.cache_image
        cluster_size = int(self.config.cluster_size)
        iam_role = self.config.iam_role
        head_cpu = self.config.head_cpu
        head_mem = self.config.head_mem
        worker_cpu = self.config.worker_cpu
        worker_mem = self.config.worker_mem

        # Create Kubernetes resources using client SDK
        create_cache_resources(
            train_job_name=train_job_name,
            iam_role=iam_role,
            cluster_size=cluster_size,
            cache_image=cache_image,
            head_cpu=head_cpu,
            head_mem=head_mem,
            worker_cpu=worker_cpu,
            worker_mem=worker_mem,
            namespace=self.config.namespace,
            metadata_loc=self.config.metadata_loc,
            table_name=self.config.table_name,
            schema_name=self.config.schema_name,
        )

        logging.info("Cache dataset initialization completed")


def create_cache_resources(
    train_job_name: str,
    iam_role: str,
    cluster_size: int,
    cache_image: str,
    head_cpu: str,
    head_mem: str,
    worker_cpu: str,
    worker_mem: str,
    namespace: str,
    metadata_loc: Optional[str] = None,
    table_name: Optional[str] = None,
    schema_name: Optional[str] = None,
) -> bool:
    """
    Creates Kubernetes resources for cache initializer using the client SDK.

    Args:
        train_job_name: Name of the training job
        iam_role: IAM role ARN for the service account
        cluster_size: Number of workers in the cluster
        cache_image: Container image to use
        head_cpu: CPU limit/request for head node
        head_mem: Memory limit/request for head node
        worker_cpu: CPU limit/request for worker nodes
        worker_mem: Memory limit/request for worker nodes
        metadata_loc: Optional metadata location
        table_name: Optional table name
        schema_name: Optional schema name
        namespace: Target Kubernetes namespace

    Returns:
        bool: True if deployment succeeded
    """
    # Load Kubernetes configuration
    config.load_incluster_config()

    api_client = client.ApiClient()
    core_v1 = client.CoreV1Api(api_client)
    custom_api = client.CustomObjectsApi(api_client)

    # Get TrainingJob for owner reference
    try:
        training_job = custom_api.get_namespaced_custom_object(
            group="trainer.kubeflow.org",
            version="v1alpha1",
            plural="trainjobs",
            namespace=namespace,
            name=train_job_name,
        )
        logging.info(f"TrainJob: {training_job}")

        # Create owner reference
        owner_ref = {
            "apiVersion": training_job["apiVersion"],
            "kind": training_job["kind"],
            "name": training_job["metadata"]["name"],
            "uid": training_job["metadata"]["uid"],
            "controller": True,
            "blockOwnerDeletion": True,
        }
    except ApiException as e:
        logging.error(f"Failed to get TrainingJob {train_job_name}: {e}")
        return False

    try:
        # Create ServiceAccount
        service_account = client.V1ServiceAccount(
            metadata=client.V1ObjectMeta(
                name=f"{train_job_name}-sa",
                namespace=namespace,
                annotations={
                    "eks.amazonaws.com/sts-regional-endpoints": "true",
                    "eks.amazonaws.com/role-arn": iam_role,
                },
                owner_references=[owner_ref],
            )
        )

        try:
            core_v1.create_namespaced_service_account(
                namespace=namespace, body=service_account
            )
            logging.info(f"Created ServiceAccount {train_job_name}-sa")
        except ApiException as e:
            if e.status == 409:
                logging.info(
                    f"ServiceAccount {train_job_name}-sa already exists, skipping creation"
                )
            else:
                raise

        # Prepare environment variables
        env_vars = []
        if metadata_loc:
            env_vars.append({"name": "METADATA_LOC", "value": metadata_loc})
        if table_name:
            env_vars.append({"name": "TABLE_NAME", "value": table_name})
        if schema_name:
            env_vars.append({"name": "SCHEMA_NAME", "value": schema_name})

        # Create LeaderWorkerSet
        lws_body = {
            "apiVersion": "leaderworkerset.x-k8s.io/v1",
            "kind": "LeaderWorkerSet",
            "metadata": {
                "name": f"{train_job_name}-cache",
                "namespace": namespace,
                "ownerReferences": [owner_ref],
            },
            "spec": {
                "replicas": 1,
                "leaderWorkerTemplate": {
                    "size": cluster_size,
                    "leaderTemplate": {
                        "metadata": {"labels": {"app": f"{train_job_name}-cache-head"}},
                        "spec": {
                            "serviceAccountName": f"{train_job_name}-sa",
                            "containers": [
                                {
                                    "name": "head",
                                    "image": cache_image,
                                    "command": ["head"],
                                    "args": ["0.0.0.0", "50051"],
                                    "resources": {
                                        "limits": {"cpu": head_cpu, "memory": head_mem},
                                        "requests": {
                                            "cpu": head_cpu,
                                            "memory": head_mem,
                                        },
                                    },
                                    "env": env_vars,
                                    "ports": [{"containerPort": 50051}],
                                }
                            ],
                        },
                    },
                    "workerTemplate": {
                        "spec": {
                            "serviceAccountName": f"{train_job_name}-sa",
                            "containers": [
                                {
                                    "name": "worker",
                                    "image": cache_image,
                                    "command": ["worker"],
                                    "args": ["0.0.0.0", "50051"],
                                    "resources": {
                                        "limits": {
                                            "cpu": worker_cpu,
                                            "memory": worker_mem,
                                        },
                                        "requests": {
                                            "cpu": worker_cpu,
                                            "memory": worker_mem,
                                        },
                                    },
                                    "env": env_vars,
                                    "ports": [{"containerPort": 50051}],
                                }
                            ],
                        }
                    },
                },
            },
        }

        # Create LeaderWorkerSet
        custom_api.create_namespaced_custom_object(
            group="leaderworkerset.x-k8s.io",
            version="v1",
            namespace=namespace,
            plural="leaderworkersets",
            body=lws_body,
        )
        logging.info(f"Created LeaderWorkerSet {train_job_name}-cache")

        # Create Service
        service = client.V1Service(
            metadata=client.V1ObjectMeta(
                name=f"{train_job_name}-cache-service",
                namespace=namespace,
                owner_references=[owner_ref],
            ),
            spec=client.V1ServiceSpec(
                selector={"app": f"{train_job_name}-cache-head"},
                ports=[
                    client.V1ServicePort(protocol="TCP", port=50051, target_port=50051)
                ],
            ),
        )

        try:
            core_v1.create_namespaced_service(namespace=namespace, body=service)
            logging.info(f"Created Service {train_job_name}-cache-service")
        except ApiException as e:
            if e.status == 409:
                logging.info(
                    f"Service {train_job_name}-cache-service already exists, skipping creation"
                )
            else:
                raise

        # Wait for LeaderWorkerSet to become ready
        lws_name = f"{train_job_name}-cache"

        while True:
            try:
                lws = custom_api.get_namespaced_custom_object(
                    group="leaderworkerset.x-k8s.io",
                    version="v1",
                    plural="leaderworkersets",
                    name=lws_name,
                    namespace=namespace,
                )

                conditions = lws.get("status", {}).get("conditions", [])
                if any(
                    c["type"] == "Available" and c["status"] == "True"
                    for c in conditions
                ):
                    logging.info(f"LeaderWorkerSet {lws_name} is ready")
                    break

                time.sleep(5)
            except ApiException:
                time.sleep(2)

        return True

    except ApiException as e:
        logging.error(f"Deployment failed: {e}")
        # Cleanup on failure
        try:
            core_v1.delete_namespaced_service_account(
                name=f"{train_job_name}-sa", namespace=namespace
            )
        except Exception as cleanup_error:
            logging.error(f"Error cleaning up ServiceAccount: {cleanup_error}")
        return False
