# coding: utf-8

"""
    Kubeflow Trainer OpenAPI Spec

    No description provided (generated by Openapi Generator https://github.com/openapitools/openapi-generator)

    The version of the OpenAPI document: unversioned
    Generated by OpenAPI Generator (https://openapi-generator.tech)

    Do not edit the class manually.
"""  # noqa: E501


from __future__ import annotations
import pprint
import re  # noqa: F401
import json

from pydantic import BaseModel, ConfigDict, Field, StrictStr, field_validator
from typing import Any, ClassVar, Dict, List, Optional
from kubeflow.trainer.models.io_k8s_api_batch_v1_pod_failure_policy_on_exit_codes_requirement import IoK8sApiBatchV1PodFailurePolicyOnExitCodesRequirement
from kubeflow.trainer.models.io_k8s_api_batch_v1_pod_failure_policy_on_pod_conditions_pattern import IoK8sApiBatchV1PodFailurePolicyOnPodConditionsPattern
from typing import Optional, Set
from typing_extensions import Self

class IoK8sApiBatchV1PodFailurePolicyRule(BaseModel):
    """
    PodFailurePolicyRule describes how a pod failure is handled when the requirements are met. One of onExitCodes and onPodConditions, but not both, can be used in each rule.
    """ # noqa: E501
    action: StrictStr = Field(description="Specifies the action taken on a pod failure when the requirements are satisfied. Possible values are:  - FailJob: indicates that the pod's job is marked as Failed and all   running pods are terminated. - FailIndex: indicates that the pod's index is marked as Failed and will   not be restarted.   This value is beta-level. It can be used when the   `JobBackoffLimitPerIndex` feature gate is enabled (enabled by default). - Ignore: indicates that the counter towards the .backoffLimit is not   incremented and a replacement pod is created. - Count: indicates that the pod is handled in the default way - the   counter towards the .backoffLimit is incremented. Additional values are considered to be added in the future. Clients should react to an unknown action by skipping the rule.  Possible enum values:  - `\"Count\"` This is an action which might be taken on a pod failure - the pod failure is handled in the default way - the counter towards .backoffLimit, represented by the job's .status.failed field, is incremented.  - `\"FailIndex\"` This is an action which might be taken on a pod failure - mark the Job's index as failed to avoid restarts within this index. This action can only be used when backoffLimitPerIndex is set. This value is beta-level.  - `\"FailJob\"` This is an action which might be taken on a pod failure - mark the pod's job as Failed and terminate all running pods.  - `\"Ignore\"` This is an action which might be taken on a pod failure - the counter towards .backoffLimit, represented by the job's .status.failed field, is not incremented and a replacement pod is created.")
    on_exit_codes: Optional[IoK8sApiBatchV1PodFailurePolicyOnExitCodesRequirement] = Field(default=None, description="Represents the requirement on the container exit codes.", alias="onExitCodes")
    on_pod_conditions: Optional[List[IoK8sApiBatchV1PodFailurePolicyOnPodConditionsPattern]] = Field(default=None, description="Represents the requirement on the pod conditions. The requirement is represented as a list of pod condition patterns. The requirement is satisfied if at least one pattern matches an actual pod condition. At most 20 elements are allowed.", alias="onPodConditions")
    __properties: ClassVar[List[str]] = ["action", "onExitCodes", "onPodConditions"]

    @field_validator('action')
    def action_validate_enum(cls, value):
        """Validates the enum"""
        if value not in set(['Count', 'FailIndex', 'FailJob', 'Ignore']):
            raise ValueError("must be one of enum values ('Count', 'FailIndex', 'FailJob', 'Ignore')")
        return value

    model_config = ConfigDict(
        populate_by_name=True,
        validate_assignment=True,
        protected_namespaces=(),
    )


    def to_str(self) -> str:
        """Returns the string representation of the model using alias"""
        return pprint.pformat(self.model_dump(by_alias=True))

    def to_json(self) -> str:
        """Returns the JSON representation of the model using alias"""
        # TODO: pydantic v2: use .model_dump_json(by_alias=True, exclude_unset=True) instead
        return json.dumps(self.to_dict())

    @classmethod
    def from_json(cls, json_str: str) -> Optional[Self]:
        """Create an instance of IoK8sApiBatchV1PodFailurePolicyRule from a JSON string"""
        return cls.from_dict(json.loads(json_str))

    def to_dict(self) -> Dict[str, Any]:
        """Return the dictionary representation of the model using alias.

        This has the following differences from calling pydantic's
        `self.model_dump(by_alias=True)`:

        * `None` is only added to the output dict for nullable fields that
          were set at model initialization. Other fields with value `None`
          are ignored.
        """
        excluded_fields: Set[str] = set([
        ])

        _dict = self.model_dump(
            by_alias=True,
            exclude=excluded_fields,
            exclude_none=True,
        )
        # override the default output from pydantic by calling `to_dict()` of on_exit_codes
        if self.on_exit_codes:
            _dict['onExitCodes'] = self.on_exit_codes.to_dict()
        # override the default output from pydantic by calling `to_dict()` of each item in on_pod_conditions (list)
        _items = []
        if self.on_pod_conditions:
            for _item_on_pod_conditions in self.on_pod_conditions:
                if _item_on_pod_conditions:
                    _items.append(_item_on_pod_conditions.to_dict())
            _dict['onPodConditions'] = _items
        return _dict

    @classmethod
    def from_dict(cls, obj: Optional[Dict[str, Any]]) -> Optional[Self]:
        """Create an instance of IoK8sApiBatchV1PodFailurePolicyRule from a dict"""
        if obj is None:
            return None

        if not isinstance(obj, dict):
            return cls.model_validate(obj)

        _obj = cls.model_validate({
            "action": obj.get("action") if obj.get("action") is not None else 'Count',
            "onExitCodes": IoK8sApiBatchV1PodFailurePolicyOnExitCodesRequirement.from_dict(obj["onExitCodes"]) if obj.get("onExitCodes") is not None else None,
            "onPodConditions": [IoK8sApiBatchV1PodFailurePolicyOnPodConditionsPattern.from_dict(_item) for _item in obj["onPodConditions"]] if obj.get("onPodConditions") is not None else None
        })
        return _obj


