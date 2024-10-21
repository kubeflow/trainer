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
from kubeflow.training.models.kubeflow_org_v2alpha1_input_model import KubeflowOrgV2alpha1InputModel  # noqa: E501
from kubeflow.training.rest import ApiException

class TestKubeflowOrgV2alpha1InputModel(unittest.TestCase):
    """KubeflowOrgV2alpha1InputModel unit test stubs"""

    def setUp(self):
        pass

    def tearDown(self):
        pass

    def make_instance(self, include_optional):
        """Test KubeflowOrgV2alpha1InputModel
            include_option is a boolean, when False only required
            params are included, when True both required and
            optional params are included """
        # model = kubeflow.training.models.kubeflow_org_v2alpha1_input_model.KubeflowOrgV2alpha1InputModel()  # noqa: E501
        if include_optional :
            return KubeflowOrgV2alpha1InputModel(
                env = [
                    None
                    ], 
                secret_ref = None, 
                storage_uri = '0'
            )
        else :
            return KubeflowOrgV2alpha1InputModel(
        )

    def testKubeflowOrgV2alpha1InputModel(self):
        """Test KubeflowOrgV2alpha1InputModel"""
        inst_req_only = self.make_instance(include_optional=False)
        inst_req_and_optional = self.make_instance(include_optional=True)


if __name__ == '__main__':
    unittest.main()
