import logging
import os
from typing import Optional

import yaml
import time
from kubernetes import client, config, utils as k8s_utils
from kubernetes.client.rest import ApiException
from kubernetes.utils import FailToCreateError

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
        logging.info(f"Cache initializer called with storage URI: {self.config.storage_uri}")

        # Check required fields
        if not self.config.cache_image:
            logging.error("CACHE_IMAGE environment variable is required but not provided")
            raise ValueError("CACHE_IMAGE environment variable is required")
        
        if not self.config.iam_role:
            logging.error("IAM_ROLE environment variable is required but not provided")
            raise ValueError("IAM_ROLE environment variable is required")

        train_job_name = self.config.train_job_name or "cache-test"
        cache_image = self.config.cache_image
        cluster_size = self.config.cluster_size or "3"
        iam_role = self.config.iam_role
        head_cpu = self.config.head_cpu or "1"
        head_mem = self.config.head_mem or "1Gi"
        worker_cpu = self.config.worker_cpu or "2"
        worker_mem = self.config.worker_mem or "2Gi"
        
        substitutions = {
            'NAME': train_job_name,
            'IAM_ROLE': iam_role,
            'SIZE': cluster_size,
            'IMAGE': cache_image,
            'HEAD_CPU': head_cpu,
            'HEAD_MEM': head_mem,
            'WORKER_CPU': worker_cpu,
            'WORKER_MEM': worker_mem,
        }
        
        if self.config.metadata_loc:
            substitutions['METADATA_LOC'] = self.config.metadata_loc
        if self.config.table_name:
            substitutions['TABLE_NAME'] = self.config.table_name
        if self.config.schema_name:
            substitutions['SCHEMA_NAME'] = self.config.schema_name

        deploy_lws_with_substitution(
            train_job_name,
            'cache-initializer-template.yaml',
            namespace='cache-test',
            substitutions=substitutions,
            timeout=600
        )

        logging.info("Cache dataset initialization completed")


def deploy_lws_with_substitution(train_job_name, yaml_path, config_file: Optional[str] = None, namespace='default', substitutions=None,
                                 timeout=300):
    """
    Deploys a parameterized LeaderWorkerSet YAML with ServiceAccount, environment substitution,
    and waits for deployment readiness.

    Args:
        train_job_name(str): train_job_name
        yaml_path (str): Path to YAML file containing configuration
        namespace (str): Target Kubernetes namespace
        substitutions (dict): Additional variables for substitution
        timeout (int): Maximum wait time in seconds

    Returns:
        bool: True if deployment succeeded and became ready
    """
    with open(yaml_path, 'r') as f:
        content = f.read()

    sub_vars = {**os.environ, **(substitutions or {})}

    for key, value in sub_vars.items():
        content = content.replace(f'${key}', value)
        content = content.replace(f'${{{key}}}', value)

    resources = list(yaml.safe_load_all(content))

    if config_file or not is_running_in_k8s():
        config.load_kube_config(config_file=config_file)
    else:
        config.load_incluster_config()

    api_client = client.ApiClient()
    core_v1 = client.CoreV1Api(api_client)
    custom_api = client.CustomObjectsApi(api_client)
    training_job = custom_api.get_namespaced_custom_object(
        group="trainer.kubeflow.org",
        version="v1alpha1",
        plural="trainjobs",
        namespace=namespace,
        name=train_job_name
    )
    print(f"trainJob: {training_job}")

    # Create owner reference from TrainingJob
    owner_ref = {
        "apiVersion": training_job["apiVersion"],
        "kind": training_job["kind"],
        "name": training_job["metadata"]["name"],
        "uid": training_job["metadata"]["uid"],
        "controller": True,
        "blockOwnerDeletion": True
    }

    created_lws = []
    created_sa = []

    try:
        for resource in resources:
            if "ownerReferences" in resource["metadata"]:
                resource["metadata"]["ownerReferences"].append(owner_ref)
            else:
                resource["metadata"]["ownerReferences"] = [owner_ref]

            if resource['kind'] == 'LeaderWorkerSet':
                created_lws.append(resource)
            else:
                try:
                    print(f"creating resource {resource}")
                    k8s_utils.create_from_dict(api_client, resource, namespace=namespace)
                except FailToCreateError as ex:
                    for e in ex.api_exceptions:
                        if e.status == 409:
                            print(
                                f"Resource {resource['kind']}/{resource['metadata']['name']} already exists, skipping creation")
                        else:
                            raise

        lws_to_watch = []
        for lws_resource in created_lws:
            group = 'leaderworkerset.x-k8s.io'
            version = 'v1'
            plural = 'leaderworkersets'
            name = lws_resource['metadata']['name']

            custom_api.create_namespaced_custom_object(
                group,
                version,
                namespace,
                plural,
                lws_resource,
            )
            lws_to_watch.append((group, version, plural, name))

        start_time = time.time()
        for group, version, plural, name in lws_to_watch:
            while time.time() - start_time < timeout:
                try:
                    lws = custom_api.get_namespaced_custom_object(
                        group=group,
                        version=version,
                        plural=plural,
                        name=name,
                        namespace=namespace
                    )

                    if any(c['type'] == 'Available' and c['status'] == 'True'
                           for c in lws.get('status', {}).get('conditions', [])):
                        break

                    time.sleep(5)
                except ApiException:
                    time.sleep(2)
            else:
                raise TimeoutError(f"LWS {name} didn't become ready in {timeout}s")

        return True

    except ApiException as e:
        print(f"Deployment failed: {e}")
        for sa_name, ns in created_sa:
            try:
                core_v1.delete_namespaced_service_account(
                    name=sa_name,
                    namespace=ns
                )
            except Exception as cleanup_error:
                print(f"Error cleaning up SA {sa_name}: {cleanup_error}")
        return False


def is_running_in_k8s() -> bool:
    return os.path.isdir("/var/run/secrets/kubernetes.io/")
