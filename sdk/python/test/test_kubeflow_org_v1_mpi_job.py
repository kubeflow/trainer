# coding: utf-8

"""
    Kubeflow Training SDK

    Python SDK for Kubeflow Training  # noqa: E501

    The version of the OpenAPI document: v1.7.0
    Generated by: https://openapi-generator.tech
"""


from __future__ import absolute_import

import unittest
import datetime

from kubeflow.training.models import *
from kubeflow.training.models.kubeflow_org_v1_mpi_job import KubeflowOrgV1MPIJob  # noqa: E501
from kubeflow.training.rest import ApiException

class TestKubeflowOrgV1MPIJob(unittest.TestCase):
    """KubeflowOrgV1MPIJob unit test stubs"""

    def setUp(self):
        pass

    def tearDown(self):
        pass

    def make_instance(self, include_optional):
        """Test KubeflowOrgV1MPIJob
            include_option is a boolean, when False only required
            params are included, when True both required and
            optional params are included """
        # model = kubeflow.training.models.kubeflow_org_v1_mpi_job.KubeflowOrgV1MPIJob()  # noqa: E501
        if include_optional :
            return KubeflowOrgV1MPIJob(
                api_version = '0', 
                kind = '0', 
                metadata = V1ObjectMeta(
                    annotations = {
                        'key' : '0'
                        }, 
                    creation_timestamp = datetime.datetime.strptime('2013-10-20 19:20:30.00', '%Y-%m-%d %H:%M:%S.%f'), 
                    deletion_grace_period_seconds = 56, 
                    deletion_timestamp = datetime.datetime.strptime('2013-10-20 19:20:30.00', '%Y-%m-%d %H:%M:%S.%f'), 
                    finalizers = [
                        '0'
                        ], 
                    generate_name = '0', 
                    generation = 56, 
                    labels = {
                        'key' : '0'
                        }, 
                    managed_fields = [
                        V1ManagedFieldsEntry(
                            api_version = '0', 
                            fields_type = '0', 
                            fields_v1 = V1FieldsV1(), 
                            manager = '0', 
                            operation = '0', 
                            subresource = '0', 
                            time = datetime.datetime.strptime('2013-10-20 19:20:30.00', '%Y-%m-%d %H:%M:%S.%f'), )
                        ], 
                    name = '0', 
                    namespace = '0', 
                    owner_references = [
                        V1OwnerReference(
                            api_version = '0', 
                            block_owner_deletion = True, 
                            controller = True, 
                            kind = '0', 
                            name = '0', 
                            uid = '0', )
                        ], 
                    resource_version = '0', 
                    self_link = '0', 
                    uid = '0', ), 
                spec = kubeflow_org_v1_mpi_job_spec.KubeflowOrgV1MPIJobSpec(
                    clean_pod_policy = '0', 
                    main_container = '0', 
                    mpi_replica_specs = {
                        'key' : kubeflow_org_v1_replica_spec.KubeflowOrgV1ReplicaSpec(
                            replicas = 56, 
                            restart_policy = '0', 
                            template = None, )
                        }, 
                    run_policy = kubeflow_org_v1_run_policy.KubeflowOrgV1RunPolicy(
                        active_deadline_seconds = 56, 
                        backoff_limit = 56, 
                        clean_pod_policy = '0', 
                        managed_by = '0', 
                        scheduling_policy = kubeflow_org_v1_scheduling_policy.KubeflowOrgV1SchedulingPolicy(
                            min_available = 56, 
                            min_resources = {
                                'key' : None
                                }, 
                            priority_class = '0', 
                            queue = '0', 
                            schedule_timeout_seconds = 56, ), 
                        suspend = True, 
                        ttl_seconds_after_finished = 56, ), 
                    slots_per_worker = 56, ), 
                status = kubeflow_org_v1_job_status.KubeflowOrgV1JobStatus(
                    completion_time = datetime.datetime.strptime('2013-10-20 19:20:30.00', '%Y-%m-%d %H:%M:%S.%f'), 
                    conditions = [
                        kubeflow_org_v1_job_condition.KubeflowOrgV1JobCondition(
                            last_transition_time = datetime.datetime.strptime('2013-10-20 19:20:30.00', '%Y-%m-%d %H:%M:%S.%f'), 
                            last_update_time = datetime.datetime.strptime('2013-10-20 19:20:30.00', '%Y-%m-%d %H:%M:%S.%f'), 
                            message = '0', 
                            reason = '0', 
                            status = '0', 
                            type = '0', )
                        ], 
                    last_reconcile_time = datetime.datetime.strptime('2013-10-20 19:20:30.00', '%Y-%m-%d %H:%M:%S.%f'), 
                    replica_statuses = {
                        'key' : kubeflow_org_v1_replica_status.KubeflowOrgV1ReplicaStatus(
                            active = 56, 
                            failed = 56, 
                            label_selector = V1LabelSelector(
                                match_expressions = [
                                    V1LabelSelectorRequirement(
                                        key = '0', 
                                        operator = '0', 
                                        values = [
                                            '0'
                                            ], )
                                    ], 
                                match_labels = {
                                    'key' : '0'
                                    }, ), 
                            selector = '0', 
                            succeeded = 56, )
                        }, 
                    start_time = datetime.datetime.strptime('2013-10-20 19:20:30.00', '%Y-%m-%d %H:%M:%S.%f'), )
            )
        else :
            return KubeflowOrgV1MPIJob(
        )

    def testKubeflowOrgV1MPIJob(self):
        """Test KubeflowOrgV1MPIJob"""
        inst_req_only = self.make_instance(include_optional=False)
        inst_req_and_optional = self.make_instance(include_optional=True)


if __name__ == '__main__':
    unittest.main()
