import logging
import time

from kubernetes import client, config
from kubernetes.client.rest import ApiException
from kubernetes.dynamic.exceptions import ConflictError

import pkg.initializers.types.types as types
import pkg.initializers.utils.utils as utils

logging.basicConfig(
    format="%(asctime)s %(levelname)-8s [%(filename)s:%(lineno)d] %(message)s",
    datefmt="%Y-%m-%dT%H:%M:%SZ",
    level=logging.INFO,
)


def get_namespace() -> str:
    """Get the current namespace from the service account token."""
    try:
        with open("/var/run/secrets/kubernetes.io/serviceaccount/namespace") as f:
            return f.readline().strip()
    except FileNotFoundError:
        logging.warning(
            "Service account namespace file not found, using 'default' namespace"
        )
        return "default"


class CacheInitializer(utils.DatasetProvider):

    def load_config(self):
        config_dict = utils.get_config_from_env(types.CacheDatasetInitializer)
        # Filter out None values to allow dataclass defaults to be used
        config_dict = {k: v for k, v in config_dict.items() if v is not None}
        self.config = types.CacheDatasetInitializer(**config_dict)

        # Parse schema_name and table_name from storage_uri
        # Format: cache://<SCHEMA_NAME>/<TABLE_NAME>
        uri_path = self.config.storage_uri[len("cache://") :]
        parts = uri_path.split("/")
        self.schema_name = parts[0]
        self.table_name = parts[1]

    def download_dataset(self):
        """Bootstrap cache cluster with dataset"""
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
        readiness_initial_delay = int(self.config.readiness_initial_delay_seconds)
        readiness_period = int(self.config.readiness_period_seconds)
        readiness_timeout = int(self.config.readiness_timeout_seconds)
        readiness_failure_threshold = int(self.config.readiness_failure_threshold)
        namespace = get_namespace()
        metadata_loc = self.config.metadata_loc
        table_name = self.table_name
        schema_name = self.schema_name

        # Load Kubernetes configuration
        config.load_incluster_config()

        api_client = client.ApiClient()
        core_v1 = client.CoreV1Api(api_client)
        custom_api = client.CustomObjectsApi(api_client)

        # Get TrainJob for owner reference
        try:
            training_job = custom_api.get_namespaced_custom_object(
                group="trainer.kubeflow.org",
                version="v1alpha1",
                plural="trainjobs",
                namespace=namespace,
                name=train_job_name,
            )
            logging.info(f"TrainJob: {training_job}")

            # Create owner reference dictionary
            logging.info(
                f"Creating owner reference from TrainJob: {training_job['metadata']['name']}"
            )

            owner_ref_dict = {
                "apiVersion": training_job["apiVersion"],
                "kind": training_job["kind"],
                "name": training_job["metadata"]["name"],
                "uid": training_job["metadata"]["uid"],
                "controller": True,
                "blockOwnerDeletion": True,
            }

            logging.info(
                f"Owner reference created with apiVersion='{training_job['apiVersion']}', "
                f"kind='{training_job['kind']}')"
            )
        except ApiException as e:
            logging.error(f"Failed to get TrainJob {train_job_name}: {e}")
            return

        try:
            # Create ServiceAccount
            service_account = client.V1ServiceAccount(
                metadata=client.V1ObjectMeta(
                    name=f"{train_job_name}-cache",
                    namespace=namespace,
                    annotations={
                        "eks.amazonaws.com/sts-regional-endpoints": "true",
                        "eks.amazonaws.com/role-arn": iam_role,
                    },
                    owner_references=[owner_ref_dict],
                )
            )

            try:
                core_v1.create_namespaced_service_account(
                    namespace=namespace, body=service_account
                )
                logging.info(f"Created ServiceAccount {service_account.metadata.name}")
            except ApiException as e:
                if e.status == 409:
                    logging.info(
                        f"ServiceAccount {service_account.metadata.name} "
                        f"already exists, skipping creation"
                    )
                else:
                    raise e

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
                    "ownerReferences": [owner_ref_dict],
                },
                "spec": {
                    "replicas": 1,
                    "leaderWorkerTemplate": {
                        "size": cluster_size,
                        "leaderTemplate": {
                            "metadata": {
                                "labels": {"app": f"{train_job_name}-cache-head"}
                            },
                            "spec": {
                                "serviceAccountName": service_account.metadata.name,
                                "containers": [
                                    {
                                        "name": "head",
                                        "image": cache_image,
                                        "command": ["head"],
                                        "args": ["0.0.0.0", "50051"],
                                        "resources": {
                                            "limits": {
                                                "cpu": head_cpu,
                                                "memory": head_mem,
                                            },
                                            "requests": {
                                                "cpu": head_cpu,
                                                "memory": head_mem,
                                            },
                                        },
                                        "env": env_vars,
                                        "ports": [
                                            {"containerPort": 50051, "name": "grpc"},
                                            {"containerPort": 8080, "name": "health"},
                                        ],
                                        "readinessProbe": {
                                            "httpGet": {
                                                "path": "/ready",
                                                "port": 8080,
                                            },
                                            "initialDelaySeconds": readiness_initial_delay,
                                            "periodSeconds": readiness_period,
                                            "timeoutSeconds": readiness_timeout,
                                            "failureThreshold": readiness_failure_threshold,
                                        },
                                    }
                                ],
                            },
                        },
                        "workerTemplate": {
                            "spec": {
                                "serviceAccountName": f"{train_job_name}-cache",
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
                                        "ports": [
                                            {"containerPort": 50051, "name": "grpc"},
                                            {"containerPort": 8080, "name": "health"},
                                        ],
                                        "readinessProbe": {
                                            "httpGet": {
                                                "path": "/ready",
                                                "port": 8080,
                                            },
                                            "initialDelaySeconds": readiness_initial_delay,
                                            "periodSeconds": readiness_period,
                                            "timeoutSeconds": readiness_timeout,
                                            "failureThreshold": readiness_failure_threshold,
                                        },
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
            logging.info(f"Created LeaderWorkerSet {lws_body['metadata']['name']}")

            # Create Service
            service = client.V1Service(
                metadata=client.V1ObjectMeta(
                    name=f"{train_job_name}-cache-service",
                    namespace=namespace,
                    owner_references=[owner_ref_dict],
                ),
                spec=client.V1ServiceSpec(
                    selector={"app": f"{train_job_name}-cache-head"},
                    ports=[
                        client.V1ServicePort(
                            protocol="TCP", port=50051, target_port=50051
                        )
                    ],
                ),
            )

            try:
                core_v1.create_namespaced_service(namespace=namespace, body=service)
                logging.info(f"Created Service {service.metadata.name}")
            except ApiException as e:
                if e is ConflictError:
                    logging.info(
                        f"Service {service.metadata.name} already exists, "
                        f"skipping creation"
                    )
                else:
                    raise e

            # Wait for LeaderWorkerSet to become ready
            # TODO:// refactor to use watch API
            while True:
                try:
                    lws = custom_api.get_namespaced_custom_object(
                        group="leaderworkerset.x-k8s.io",
                        version="v1",
                        plural="leaderworkersets",
                        name=lws_body["metadata"]["name"],
                        namespace=namespace,
                    )

                    conditions = lws.get("status", {}).get("conditions", [])
                    if any(
                        c["type"] == "Available" and c["status"] == "True"
                        for c in conditions
                    ):
                        logging.info(
                            f"LeaderWorkerSet {lws_body['metadata']['name']} is ready"
                        )
                        break

                    time.sleep(5)
                except ApiException as e:
                    raise e

        except ApiException as e:
            logging.error(f"Cache cluster creation failed: {e}")
            # Cleanup on failure
            try:
                core_v1.delete_namespaced_service_account(
                    name=f"{train_job_name}-cache", namespace=namespace
                )
            except Exception as cleanup_error:
                logging.error(f"Error cleaning up ServiceAccount: {cleanup_error}")
            return

        logging.info("Cache cluster creation completed")
