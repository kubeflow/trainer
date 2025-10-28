from dataclasses import dataclass, field
from typing import Optional


# Configuration for the HuggingFace dataset initializer.
# TODO (andreyvelich): Discuss how to keep these configurations is sync with Kubeflow SDK types.
@dataclass
class HuggingFaceDatasetInitializer:
    storage_uri: str
    ignore_patterns: Optional[list[str]] = None
    access_token: Optional[str] = None


# Configuration for the S3 dataset initializer.
@dataclass
class S3DatasetInitializer:
    storage_uri: str
    ignore_patterns: Optional[list[str]] = None
    endpoint: Optional[str] = None
    access_key_id: Optional[str] = None
    secret_access_key: Optional[str] = None
    region: Optional[str] = None
    role_arn: Optional[str] = None


# Configuration for the HuggingFace model initializer.
@dataclass
class HuggingFaceModelInitializer:
    storage_uri: str
    ignore_patterns: Optional[list[str]] = field(
        default_factory=lambda: ["*.msgpack", "*.h5", "*.bin", ".pt", ".pth"]
    )
    access_token: Optional[str] = None


# Configuration for the S3 model initializer.
@dataclass
class S3ModelInitializer:
    storage_uri: str
    ignore_patterns: Optional[list[str]] = field(
        default_factory=lambda: ["*.msgpack", "*.h5", "*.bin", ".pt", ".pth"]
    )
    endpoint: Optional[str] = None
    access_key_id: Optional[str] = None
    secret_access_key: Optional[str] = None
    region: Optional[str] = None
    role_arn: Optional[str] = None


# Configuration for the cache dataset initializer.
@dataclass
class CacheDatasetInitializer:
    storage_uri: str
    train_job_name: str
    cache_image: str
    iam_role: str
    metadata_loc: str
    cluster_size: str = "3"
    head_cpu: str = "1"
    head_mem: str = "1Gi"
    worker_cpu: str = "2"
    worker_mem: str = "2Gi"
