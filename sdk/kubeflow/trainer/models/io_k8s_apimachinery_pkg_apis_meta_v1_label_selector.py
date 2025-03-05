# coding: utf-8

"""
    Kubeflow Trainer OpenAPI Spec

    No description provided (generated by Openapi Generator https://github.com/openapitools/openapi-generator)

    The version of the OpenAPI document: 1.0.0
    Generated by OpenAPI Generator (https://openapi-generator.tech)

    Do not edit the class manually.
"""  # noqa: E501


from __future__ import annotations
import pprint
import re  # noqa: F401
import json

from pydantic import BaseModel, ConfigDict, Field, StrictStr
from typing import Any, ClassVar, Dict, List, Optional
from kubeflow.trainer.models.io_k8s_apimachinery_pkg_apis_meta_v1_label_selector_requirement import IoK8sApimachineryPkgApisMetaV1LabelSelectorRequirement
from typing import Optional, Set
from typing_extensions import Self

class IoK8sApimachineryPkgApisMetaV1LabelSelector(BaseModel):
    """
    A label selector is a label query over a set of resources. The result of matchLabels and matchExpressions are ANDed. An empty label selector matches all objects. A null label selector matches no objects.
    """ # noqa: E501
    match_expressions: Optional[List[IoK8sApimachineryPkgApisMetaV1LabelSelectorRequirement]] = Field(default=None, description="matchExpressions is a list of label selector requirements. The requirements are ANDed.", alias="matchExpressions")
    match_labels: Optional[Dict[str, StrictStr]] = Field(default=None, description="matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is \"key\", the operator is \"In\", and the values array contains only \"value\". The requirements are ANDed.", alias="matchLabels")
    __properties: ClassVar[List[str]] = ["matchExpressions", "matchLabels"]

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
        """Create an instance of IoK8sApimachineryPkgApisMetaV1LabelSelector from a JSON string"""
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
        # override the default output from pydantic by calling `to_dict()` of each item in match_expressions (list)
        _items = []
        if self.match_expressions:
            for _item_match_expressions in self.match_expressions:
                if _item_match_expressions:
                    _items.append(_item_match_expressions.to_dict())
            _dict['matchExpressions'] = _items
        return _dict

    @classmethod
    def from_dict(cls, obj: Optional[Dict[str, Any]]) -> Optional[Self]:
        """Create an instance of IoK8sApimachineryPkgApisMetaV1LabelSelector from a dict"""
        if obj is None:
            return None

        if not isinstance(obj, dict):
            return cls.model_validate(obj)

        _obj = cls.model_validate({
            "matchExpressions": [IoK8sApimachineryPkgApisMetaV1LabelSelectorRequirement.from_dict(_item) for _item in obj["matchExpressions"]] if obj.get("matchExpressions") is not None else None,
            "matchLabels": obj.get("matchLabels")
        })
        return _obj


