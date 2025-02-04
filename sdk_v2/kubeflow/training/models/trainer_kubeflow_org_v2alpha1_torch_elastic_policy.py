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


class TrainerKubeflowOrgV2alpha1TorchElasticPolicy(object):
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
        'max_nodes': 'int',
        'max_restarts': 'int',
        'metrics': 'list[K8sIoApiAutoscalingV2MetricSpec]',
        'min_nodes': 'int'
    }

    attribute_map = {
        'max_nodes': 'maxNodes',
        'max_restarts': 'maxRestarts',
        'metrics': 'metrics',
        'min_nodes': 'minNodes'
    }

    def __init__(self, max_nodes=None, max_restarts=None, metrics=None, min_nodes=None, local_vars_configuration=None):  # noqa: E501
        """TrainerKubeflowOrgV2alpha1TorchElasticPolicy - a model defined in OpenAPI"""  # noqa: E501
        if local_vars_configuration is None:
            local_vars_configuration = Configuration()
        self.local_vars_configuration = local_vars_configuration

        self._max_nodes = None
        self._max_restarts = None
        self._metrics = None
        self._min_nodes = None
        self.discriminator = None

        if max_nodes is not None:
            self.max_nodes = max_nodes
        if max_restarts is not None:
            self.max_restarts = max_restarts
        if metrics is not None:
            self.metrics = metrics
        if min_nodes is not None:
            self.min_nodes = min_nodes

    @property
    def max_nodes(self):
        """Gets the max_nodes of this TrainerKubeflowOrgV2alpha1TorchElasticPolicy.  # noqa: E501

        Upper limit for the number of nodes to which training job can scale up.  # noqa: E501

        :return: The max_nodes of this TrainerKubeflowOrgV2alpha1TorchElasticPolicy.  # noqa: E501
        :rtype: int
        """
        return self._max_nodes

    @max_nodes.setter
    def max_nodes(self, max_nodes):
        """Sets the max_nodes of this TrainerKubeflowOrgV2alpha1TorchElasticPolicy.

        Upper limit for the number of nodes to which training job can scale up.  # noqa: E501

        :param max_nodes: The max_nodes of this TrainerKubeflowOrgV2alpha1TorchElasticPolicy.  # noqa: E501
        :type: int
        """

        self._max_nodes = max_nodes

    @property
    def max_restarts(self):
        """Gets the max_restarts of this TrainerKubeflowOrgV2alpha1TorchElasticPolicy.  # noqa: E501

        How many times the training job can be restarted. This value is inserted into the `--max-restarts` argument of the `torchrun` CLI and the `.spec.failurePolicy.maxRestarts` parameter of the training Job.  # noqa: E501

        :return: The max_restarts of this TrainerKubeflowOrgV2alpha1TorchElasticPolicy.  # noqa: E501
        :rtype: int
        """
        return self._max_restarts

    @max_restarts.setter
    def max_restarts(self, max_restarts):
        """Sets the max_restarts of this TrainerKubeflowOrgV2alpha1TorchElasticPolicy.

        How many times the training job can be restarted. This value is inserted into the `--max-restarts` argument of the `torchrun` CLI and the `.spec.failurePolicy.maxRestarts` parameter of the training Job.  # noqa: E501

        :param max_restarts: The max_restarts of this TrainerKubeflowOrgV2alpha1TorchElasticPolicy.  # noqa: E501
        :type: int
        """

        self._max_restarts = max_restarts

    @property
    def metrics(self):
        """Gets the metrics of this TrainerKubeflowOrgV2alpha1TorchElasticPolicy.  # noqa: E501

        Specification which are used to calculate the desired number of nodes. See the individual metric source types for more information about how each type of metric must respond. The HPA will be created to perform auto-scaling.  # noqa: E501

        :return: The metrics of this TrainerKubeflowOrgV2alpha1TorchElasticPolicy.  # noqa: E501
        :rtype: list[K8sIoApiAutoscalingV2MetricSpec]
        """
        return self._metrics

    @metrics.setter
    def metrics(self, metrics):
        """Sets the metrics of this TrainerKubeflowOrgV2alpha1TorchElasticPolicy.

        Specification which are used to calculate the desired number of nodes. See the individual metric source types for more information about how each type of metric must respond. The HPA will be created to perform auto-scaling.  # noqa: E501

        :param metrics: The metrics of this TrainerKubeflowOrgV2alpha1TorchElasticPolicy.  # noqa: E501
        :type: list[K8sIoApiAutoscalingV2MetricSpec]
        """

        self._metrics = metrics

    @property
    def min_nodes(self):
        """Gets the min_nodes of this TrainerKubeflowOrgV2alpha1TorchElasticPolicy.  # noqa: E501

        Lower limit for the number of nodes to which training job can scale down.  # noqa: E501

        :return: The min_nodes of this TrainerKubeflowOrgV2alpha1TorchElasticPolicy.  # noqa: E501
        :rtype: int
        """
        return self._min_nodes

    @min_nodes.setter
    def min_nodes(self, min_nodes):
        """Sets the min_nodes of this TrainerKubeflowOrgV2alpha1TorchElasticPolicy.

        Lower limit for the number of nodes to which training job can scale down.  # noqa: E501

        :param min_nodes: The min_nodes of this TrainerKubeflowOrgV2alpha1TorchElasticPolicy.  # noqa: E501
        :type: int
        """

        self._min_nodes = min_nodes

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
        if not isinstance(other, TrainerKubeflowOrgV2alpha1TorchElasticPolicy):
            return False

        return self.to_dict() == other.to_dict()

    def __ne__(self, other):
        """Returns true if both objects are not equal"""
        if not isinstance(other, TrainerKubeflowOrgV2alpha1TorchElasticPolicy):
            return True

        return self.to_dict() != other.to_dict()
