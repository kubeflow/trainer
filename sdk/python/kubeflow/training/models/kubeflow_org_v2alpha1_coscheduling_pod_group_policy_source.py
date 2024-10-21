# coding: utf-8

"""
    Kubeflow Training SDK

    Python SDK for Kubeflow Training  # noqa: E501

    The version of the OpenAPI document: v1.7.0
    Generated by: https://openapi-generator.tech
"""


import pprint
import re  # noqa: F401

import six

from kubeflow.training.configuration import Configuration


class KubeflowOrgV2alpha1CoschedulingPodGroupPolicySource(object):
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
        'schedule_timeout_seconds': 'int'
    }

    attribute_map = {
        'schedule_timeout_seconds': 'scheduleTimeoutSeconds'
    }

    def __init__(self, schedule_timeout_seconds=None, local_vars_configuration=None):  # noqa: E501
        """KubeflowOrgV2alpha1CoschedulingPodGroupPolicySource - a model defined in OpenAPI"""  # noqa: E501
        if local_vars_configuration is None:
            local_vars_configuration = Configuration()
        self.local_vars_configuration = local_vars_configuration

        self._schedule_timeout_seconds = None
        self.discriminator = None

        if schedule_timeout_seconds is not None:
            self.schedule_timeout_seconds = schedule_timeout_seconds

    @property
    def schedule_timeout_seconds(self):
        """Gets the schedule_timeout_seconds of this KubeflowOrgV2alpha1CoschedulingPodGroupPolicySource.  # noqa: E501

        Time threshold to schedule PodGroup for gang-scheduling. If the scheduling timeout is equal to 0, the default value is used. Defaults to 60 seconds.  # noqa: E501

        :return: The schedule_timeout_seconds of this KubeflowOrgV2alpha1CoschedulingPodGroupPolicySource.  # noqa: E501
        :rtype: int
        """
        return self._schedule_timeout_seconds

    @schedule_timeout_seconds.setter
    def schedule_timeout_seconds(self, schedule_timeout_seconds):
        """Sets the schedule_timeout_seconds of this KubeflowOrgV2alpha1CoschedulingPodGroupPolicySource.

        Time threshold to schedule PodGroup for gang-scheduling. If the scheduling timeout is equal to 0, the default value is used. Defaults to 60 seconds.  # noqa: E501

        :param schedule_timeout_seconds: The schedule_timeout_seconds of this KubeflowOrgV2alpha1CoschedulingPodGroupPolicySource.  # noqa: E501
        :type: int
        """

        self._schedule_timeout_seconds = schedule_timeout_seconds

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
        if not isinstance(other, KubeflowOrgV2alpha1CoschedulingPodGroupPolicySource):
            return False

        return self.to_dict() == other.to_dict()

    def __ne__(self, other):
        """Returns true if both objects are not equal"""
        if not isinstance(other, KubeflowOrgV2alpha1CoschedulingPodGroupPolicySource):
            return True

        return self.to_dict() != other.to_dict()
