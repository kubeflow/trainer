use serde::{Deserialize, Serialize};
use std::env;
use std::sync::Arc;
use std::time::Duration;

/// Cache index column name used across the data cache system
pub const CACHE_INDEX_COLUMN: &str = "cache_index";

/// Index pair representing a start and end range
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct IndexPair {
    pub start: u64,
    pub end: u64,
}

/// Configuration for dataset metadata and table information
#[derive(Debug, Clone)]
pub struct DatasetConfig {
    pub metadata_loc: String,
    pub schema_name: String,
    pub table_name: String,
}

/// Comprehensive configuration for the data cache system
/// Consolidates all environment variables used across the application
#[derive(Debug, Clone)]
pub struct CacheConfig {
    pub dataset: DatasetConfig,
    pub connect_timeout: Duration,
}

impl DatasetConfig {
    pub fn from_env() -> Result<Self, Box<dyn std::error::Error>> {
        let metadata_loc = env::var("METADATA_LOC")?;
        let schema_name = env::var("SCHEMA_NAME")?;
        let table_name = env::var("TABLE_NAME")?;
        Ok(DatasetConfig {
            metadata_loc,
            schema_name,
            table_name,
        })
    }
}

impl CacheConfig {
    pub fn from_env() -> Result<Self, Box<dyn std::error::Error>> {
        let dataset = DatasetConfig::from_env()?;

        // Connection timeout with default of 20 seconds
        let connect_timeout = env::var("CONNECT_TIMEOUT_SECS")
            .ok()
            .and_then(|s| s.parse::<u64>().ok())
            .map(Duration::from_secs)
            .unwrap_or(Duration::from_secs(20));

        Ok(CacheConfig {
            dataset,
            connect_timeout,
        })
    }

    /// Create shared configuration from environment variables
    /// Returns Arc<CacheConfig> for efficient sharing across components
    pub fn shared_from_env() -> Result<Arc<Self>, Box<dyn std::error::Error>> {
        Ok(Arc::new(Self::from_env()?))
    }

    /// Create a new configuration with custom timeout
    pub fn with_timeout(mut self, timeout: Duration) -> Self {
        self.connect_timeout = timeout;
        self
    }
}
