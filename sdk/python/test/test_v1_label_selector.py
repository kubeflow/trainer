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
from kubeflow.training.models.v1_label_selector import V1LabelSelector  # noqa: E501
from kubeflow.training.rest import ApiException

class TestV1LabelSelector(unittest.TestCase):
    """V1LabelSelector unit test stubs"""

    def setUp(self):
        pass

    def tearDown(self):
        pass

    def make_instance(self, include_optional):
        """Test V1LabelSelector
            include_option is a boolean, when False only required
            params are included, when True both required and
            optional params are included """
        # model = kubeflow.training.models.v1_label_selector.V1LabelSelector()  # noqa: E501
        if include_optional :
            return V1LabelSelector(
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
                    }
            )
        else :
            return V1LabelSelector(
        )

    def testV1LabelSelector(self):
        """Test V1LabelSelector"""
        inst_req_only = self.make_instance(include_optional=False)
        inst_req_and_optional = self.make_instance(include_optional=True)


if __name__ == '__main__':
    unittest.main()
