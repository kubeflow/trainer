from dataclasses import dataclass
from typing import Optional


# Configuration for the HuggingFace dataset initializer.
# TODO (andreyvelich): Discuss how to keep these configurations is sync with Kubeflow SDK types.
@dataclass
class HuggingFaceDatasetInitializer:
    storage_uri: str
    access_token: Optional[str] = None


# Configuration for the HuggingFace model initializer.
@dataclass
class HuggingFaceModelInitializer:
    storage_uri: str
    access_token: Optional[str] = None


# Configuration for the cache dataset initializer.
@dataclass
class CacheDatasetInitializer:
    storage_uri: str
    train_job_name: Optional[str] = None
    cache_image: Optional[str] = None
    cluster_size: Optional[str] = None
    metadata_loc: Optional[str] = None
    table_name: Optional[str] = None
    schema_name: Optional[str] = None
    iam_role: Optional[str] = None
    head_cpu: Optional[str] = None
    head_mem: Optional[str] = None
    worker_cpu: Optional[str] = None
    worker_mem: Optional[str] = None
