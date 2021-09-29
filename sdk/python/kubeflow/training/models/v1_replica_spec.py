# coding: utf-8

"""
    tensorflow

    Python SDK for tensorflow  # noqa: E501

    The version of the OpenAPI document: v1.3.0
    Generated by: https://openapi-generator.tech
"""


import pprint
import re  # noqa: F401

import six

from kubeflow.training.configuration import Configuration


class V1ReplicaSpec(object):
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
        'replicas': 'int',
        'restart_policy': 'str',
        'template': 'V1PodTemplateSpec'
    }

    attribute_map = {
        'replicas': 'replicas',
        'restart_policy': 'restartPolicy',
        'template': 'template'
    }

    def __init__(self, replicas=None, restart_policy=None, template=None, local_vars_configuration=None):  # noqa: E501
        """V1ReplicaSpec - a model defined in OpenAPI"""  # noqa: E501
        if local_vars_configuration is None:
            local_vars_configuration = Configuration()
        self.local_vars_configuration = local_vars_configuration

        self._replicas = None
        self._restart_policy = None
        self._template = None
        self.discriminator = None

        if replicas is not None:
            self.replicas = replicas
        if restart_policy is not None:
            self.restart_policy = restart_policy
        if template is not None:
            self.template = template

    @property
    def replicas(self):
        """Gets the replicas of this V1ReplicaSpec.  # noqa: E501

        Replicas is the desired number of replicas of the given template. If unspecified, defaults to 1.  # noqa: E501

        :return: The replicas of this V1ReplicaSpec.  # noqa: E501
        :rtype: int
        """
        return self._replicas

    @replicas.setter
    def replicas(self, replicas):
        """Sets the replicas of this V1ReplicaSpec.

        Replicas is the desired number of replicas of the given template. If unspecified, defaults to 1.  # noqa: E501

        :param replicas: The replicas of this V1ReplicaSpec.  # noqa: E501
        :type: int
        """

        self._replicas = replicas

    @property
    def restart_policy(self):
        """Gets the restart_policy of this V1ReplicaSpec.  # noqa: E501

        Restart policy for all replicas within the job. One of Always, OnFailure, Never and ExitCode. Default to Never.  # noqa: E501

        :return: The restart_policy of this V1ReplicaSpec.  # noqa: E501
        :rtype: str
        """
        return self._restart_policy

    @restart_policy.setter
    def restart_policy(self, restart_policy):
        """Sets the restart_policy of this V1ReplicaSpec.

        Restart policy for all replicas within the job. One of Always, OnFailure, Never and ExitCode. Default to Never.  # noqa: E501

        :param restart_policy: The restart_policy of this V1ReplicaSpec.  # noqa: E501
        :type: str
        """

        self._restart_policy = restart_policy

    @property
    def template(self):
        """Gets the template of this V1ReplicaSpec.  # noqa: E501


        :return: The template of this V1ReplicaSpec.  # noqa: E501
        :rtype: V1PodTemplateSpec
        """
        return self._template

    @template.setter
    def template(self, template):
        """Sets the template of this V1ReplicaSpec.


        :param template: The template of this V1ReplicaSpec.  # noqa: E501
        :type: V1PodTemplateSpec
        """

        self._template = template

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
        if not isinstance(other, V1ReplicaSpec):
            return False

        return self.to_dict() == other.to_dict()

    def __ne__(self, other):
        """Returns true if both objects are not equal"""
        if not isinstance(other, V1ReplicaSpec):
            return True

        return self.to_dict() != other.to_dict()
