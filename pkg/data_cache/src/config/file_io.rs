// Copyright The Kubeflow Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//! Helpers for constructing Iceberg [`FileIO`] from dataset metadata locations.

use iceberg::io::FileIO;

/// Supported URI schemes for Iceberg metadata and data file locations.
const ALLOWED_SCHEMES: &[&str] = &["file", "s3", "s3a"];

/// Builds an Iceberg [`FileIO`] for the given metadata location.
///
/// The location must be an absolute URI with an allowed scheme (`file://`, `s3://`, or `s3a://`).
/// Scheme inference is delegated to [`FileIO::from_path`], which selects the appropriate
/// storage backend (local filesystem, S3, etc.).
pub fn build_file_io(metadata_loc: &str) -> Result<FileIO, String> {
    validate_metadata_loc(metadata_loc)?;
    FileIO::from_path(metadata_loc)
        .map_err(|e| format!("Failed to create FileIO: {}", e))?
        .build()
        .map_err(|e| format!("Failed to build FileIO: {}", e))
}

/// Validates that `metadata_loc` is a non-empty absolute URI with an allowed scheme.
pub fn validate_metadata_loc(metadata_loc: &str) -> Result<(), String> {
    let trimmed = metadata_loc.trim();
    if trimmed.is_empty() {
        return Err("METADATA_LOC must not be empty".to_string());
    }

    let scheme = trimmed
        .split("://")
        .next()
        .filter(|_| trimmed.contains("://"))
        .ok_or_else(|| {
            format!(
                "METADATA_LOC must be an absolute URI with a scheme (e.g. file://, s3://, s3a://), got: {}",
                metadata_loc
            )
        })?;

    let scheme_lower = scheme.to_ascii_lowercase();
    if !ALLOWED_SCHEMES.contains(&scheme_lower.as_str()) {
        return Err(format!(
            "METADATA_LOC scheme '{}' is not supported; allowed schemes: {}",
            scheme,
            ALLOWED_SCHEMES.join(", ")
        ));
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn accepts_file_scheme() {
        assert!(validate_metadata_loc("file:///tmp/warehouse/metadata/v1.metadata.json").is_ok());
    }

    #[test]
    fn accepts_s3_schemes() {
        assert!(validate_metadata_loc("s3://bucket/metadata/v1.metadata.json").is_ok());
        assert!(validate_metadata_loc("s3a://bucket/metadata/v1.metadata.json").is_ok());
    }

    #[test]
    fn rejects_empty_and_relative_paths() {
        assert!(validate_metadata_loc("").is_err());
        assert!(validate_metadata_loc("/tmp/metadata.json").is_err());
        assert!(validate_metadata_loc("relative/path").is_err());
    }

    #[test]
    fn rejects_unsupported_scheme() {
        assert!(validate_metadata_loc("http://example.com/meta.json").is_err());
    }
}
