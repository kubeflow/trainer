# coding: utf-8

"""
    Kubeflow Training OpenAPI Spec

    No description provided (generated by Openapi Generator https://github.com/openapitools/openapi-generator)  # noqa: E501

    The version of the OpenAPI document: 1.0.0
    Generated by: https://openapi-generator.tech
"""


import pprint
import re  # noqa: F401

import six

from kubeflow.training.configuration import Configuration


class TrainerV2alpha1TorchMLPolicySource(object):
    """NOTE: This class is auto generated by OpenAPI Generator.
    Ref: https://openapi-generator.tech

    Do not edit the class manually.
    """

    """
    Attributes:
      openapi_types (dict): The key is attribute name
                            and the value is attribute type.
      attribute_map (dict): The key is attribute name
                            and the value is json key in definition.
    """
    openapi_types = {
        'elastic_policy': 'TrainerV2alpha1TorchElasticPolicy',
        'num_proc_per_node': 'str'
    }

    attribute_map = {
        'elastic_policy': 'elasticPolicy',
        'num_proc_per_node': 'numProcPerNode'
    }

    def __init__(self, elastic_policy=None, num_proc_per_node=None, local_vars_configuration=None):  # noqa: E501
        """TrainerV2alpha1TorchMLPolicySource - a model defined in OpenAPI"""  # noqa: E501
        if local_vars_configuration is None:
            local_vars_configuration = Configuration()
        self.local_vars_configuration = local_vars_configuration

        self._elastic_policy = None
        self._num_proc_per_node = None
        self.discriminator = None

        if elastic_policy is not None:
            self.elastic_policy = elastic_policy
        if num_proc_per_node is not None:
            self.num_proc_per_node = num_proc_per_node

    @property
    def elastic_policy(self):
        """Gets the elastic_policy of this TrainerV2alpha1TorchMLPolicySource.  # noqa: E501


        :return: The elastic_policy of this TrainerV2alpha1TorchMLPolicySource.  # noqa: E501
        :rtype: TrainerV2alpha1TorchElasticPolicy
        """
        return self._elastic_policy

    @elastic_policy.setter
    def elastic_policy(self, elastic_policy):
        """Sets the elastic_policy of this TrainerV2alpha1TorchMLPolicySource.


        :param elastic_policy: The elastic_policy of this TrainerV2alpha1TorchMLPolicySource.  # noqa: E501
        :type: TrainerV2alpha1TorchElasticPolicy
        """

        self._elastic_policy = elastic_policy

    @property
    def num_proc_per_node(self):
        """Gets the num_proc_per_node of this TrainerV2alpha1TorchMLPolicySource.  # noqa: E501

        Number of processes per node. This value is inserted into the `--nproc-per-node` argument of the `torchrun` CLI. Supported values: `auto`, `cpu`, `gpu`, or int value. Defaults to `auto`.  # noqa: E501

        :return: The num_proc_per_node of this TrainerV2alpha1TorchMLPolicySource.  # noqa: E501
        :rtype: str
        """
        return self._num_proc_per_node

    @num_proc_per_node.setter
    def num_proc_per_node(self, num_proc_per_node):
        """Sets the num_proc_per_node of this TrainerV2alpha1TorchMLPolicySource.

        Number of processes per node. This value is inserted into the `--nproc-per-node` argument of the `torchrun` CLI. Supported values: `auto`, `cpu`, `gpu`, or int value. Defaults to `auto`.  # noqa: E501

        :param num_proc_per_node: The num_proc_per_node of this TrainerV2alpha1TorchMLPolicySource.  # noqa: E501
        :type: str
        """

        self._num_proc_per_node = num_proc_per_node

    def to_dict(self):
        """Returns the model properties as a dict"""
        result = {}

        for attr, _ in six.iteritems(self.openapi_types):
            value = getattr(self, attr)
            if isinstance(value, list):
                result[attr] = list(map(
                    lambda x: x.to_dict() if hasattr(x, "to_dict") else x,
                    value
                ))
            elif hasattr(value, "to_dict"):
                result[attr] = value.to_dict()
            elif isinstance(value, dict):
                result[attr] = dict(map(
                    lambda item: (item[0], item[1].to_dict())
                    if hasattr(item[1], "to_dict") else item,
                    value.items()
                ))
            else:
                result[attr] = value

        return result

    def to_str(self):
        """Returns the string representation of the model"""
        return pprint.pformat(self.to_dict())

    def __repr__(self):
        """For `print` and `pprint`"""
        return self.to_str()

    def __eq__(self, other):
        """Returns true if both objects are equal"""
        if not isinstance(other, TrainerV2alpha1TorchMLPolicySource):
            return False

        return self.to_dict() == other.to_dict()

    def __ne__(self, other):
        """Returns true if both objects are not equal"""
        if not isinstance(other, TrainerV2alpha1TorchMLPolicySource):
            return True

        return self.to_dict() != other.to_dict()
