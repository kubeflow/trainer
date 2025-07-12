from dataclasses import dataclass
from typing import Optional


# Configuration for the HuggingFace dataset initializer.
# TODO (andreyvelich): Discuss how to keep these configurations is sync with Kubeflow SDK types.
@dataclass
class HuggingFaceDatasetInitializer:
    storage_uri: str
    access_token: Optional[str] = None


# Configuration for the S3 dataset initializer.
@dataclass
class S3DatasetInitializer:
    storage_uri: str
    endpoint_url: Optional[str] = None
    access_key_id: Optional[str] = None
    access_key_secret: Optional[str] = None
    region: Optional[str] = None


# Configuration for the HuggingFace model initializer.
@dataclass
class HuggingFaceModelInitializer:
    storage_uri: str
    access_token: Optional[str] = None


# Configuration for the S3 model initializer.
@dataclass
class S3ModelInitializer:
    storage_uri: str
    endpoint_url: Optional[str] = None
    access_key_id: Optional[str] = None
    access_key_secret: Optional[str] = None
    region: Optional[str] = None
