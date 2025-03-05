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

from pydantic import BaseModel, ConfigDict, Field, StrictBool, StrictStr
from typing import Any, ClassVar, Dict, List, Optional
from kubeflow.trainer.models.io_k8s_api_core_v1_local_object_reference import IoK8sApiCoreV1LocalObjectReference
from typing import Optional, Set
from typing_extensions import Self

class IoK8sApiCoreV1CSIVolumeSource(BaseModel):
    """
    Represents a source location of a volume to mount, managed by an external CSI driver
    """ # noqa: E501
    driver: StrictStr = Field(description="driver is the name of the CSI driver that handles this volume. Consult with your admin for the correct name as registered in the cluster.")
    fs_type: Optional[StrictStr] = Field(default=None, description="fsType to mount. Ex. \"ext4\", \"xfs\", \"ntfs\". If not provided, the empty value is passed to the associated CSI driver which will determine the default filesystem to apply.", alias="fsType")
    node_publish_secret_ref: Optional[IoK8sApiCoreV1LocalObjectReference] = Field(default=None, alias="nodePublishSecretRef")
    read_only: Optional[StrictBool] = Field(default=None, description="readOnly specifies a read-only configuration for the volume. Defaults to false (read/write).", alias="readOnly")
    volume_attributes: Optional[Dict[str, StrictStr]] = Field(default=None, description="volumeAttributes stores driver-specific properties that are passed to the CSI driver. Consult your driver's documentation for supported values.", alias="volumeAttributes")
    __properties: ClassVar[List[str]] = ["driver", "fsType", "nodePublishSecretRef", "readOnly", "volumeAttributes"]

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
        """Create an instance of IoK8sApiCoreV1CSIVolumeSource from a JSON string"""
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
        # override the default output from pydantic by calling `to_dict()` of node_publish_secret_ref
        if self.node_publish_secret_ref:
            _dict['nodePublishSecretRef'] = self.node_publish_secret_ref.to_dict()
        return _dict

    @classmethod
    def from_dict(cls, obj: Optional[Dict[str, Any]]) -> Optional[Self]:
        """Create an instance of IoK8sApiCoreV1CSIVolumeSource from a dict"""
        if obj is None:
            return None

        if not isinstance(obj, dict):
            return cls.model_validate(obj)

        _obj = cls.model_validate({
            "driver": obj.get("driver"),
            "fsType": obj.get("fsType"),
            "nodePublishSecretRef": IoK8sApiCoreV1LocalObjectReference.from_dict(obj["nodePublishSecretRef"]) if obj.get("nodePublishSecretRef") is not None else None,
            "readOnly": obj.get("readOnly"),
            "volumeAttributes": obj.get("volumeAttributes")
        })
        return _obj


