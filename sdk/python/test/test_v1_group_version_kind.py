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
from kubeflow.training.models.v1_group_version_kind import V1GroupVersionKind  # noqa: E501
from kubeflow.training.rest import ApiException

class TestV1GroupVersionKind(unittest.TestCase):
    """V1GroupVersionKind unit test stubs"""

    def setUp(self):
        pass

    def tearDown(self):
        pass

    def make_instance(self, include_optional):
        """Test V1GroupVersionKind
            include_option is a boolean, when False only required
            params are included, when True both required and
            optional params are included """
        # model = kubeflow.training.models.v1_group_version_kind.V1GroupVersionKind()  # noqa: E501
        if include_optional :
            return V1GroupVersionKind(
                group = '0', 
                kind = '0', 
                version = '0'
            )
        else :
            return V1GroupVersionKind(
                group = '0',
                kind = '0',
                version = '0',
        )

    def testV1GroupVersionKind(self):
        """Test V1GroupVersionKind"""
        inst_req_only = self.make_instance(include_optional=False)
        inst_req_and_optional = self.make_instance(include_optional=True)


if __name__ == '__main__':
    unittest.main()
