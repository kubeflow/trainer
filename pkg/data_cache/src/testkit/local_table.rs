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

//! Builds a small Iceberg table backed by local Parquet files.
//!
//! The data cache resolves its storage backend from the scheme of `METADATA_LOC`
//! alone (see [`iceberg::io::FileIO::from_path`]), and the `storage-fs` feature is
//! part of the default feature set of the `iceberg` crate. A table written to the
//! local filesystem therefore flows through exactly the same code path as a table
//! on S3, which makes it usable for validating the cache without an object store.
//!
//! The table is always generated, never checked into the repository: Iceberg
//! metadata embeds **absolute** paths to its manifests and data files, so a
//! pre-built fixture would only resolve on the machine that produced it.

use std::collections::HashMap;
use std::path::Path;
use std::sync::Arc;
use std::time::{SystemTime, UNIX_EPOCH};

use arrow::array::{Int64Array, RecordBatch, StringArray};
use arrow_schema::SchemaRef as ArrowSchemaRef;
use iceberg::arrow::schema_to_arrow_schema;
use iceberg::io::{FileIO, FileIOBuilder};
use iceberg::spec::{
    DataFile, DataFileFormat, MAIN_BRANCH, ManifestListWriter, ManifestWriterBuilder, NestedField,
    Operation, PrimitiveType, Schema, SchemaRef as IcebergSchemaRef, Snapshot, SnapshotReference,
    SnapshotRetention, Summary, TableMetadata, TableMetadataBuilder, Type,
};
use iceberg::table::Table;
use iceberg::writer::base_writer::data_file_writer::DataFileWriterBuilder;
use iceberg::writer::file_writer::ParquetWriterBuilder;
use iceberg::writer::file_writer::location_generator::{
    DefaultFileNameGenerator, DefaultLocationGenerator,
};
use iceberg::writer::{IcebergWriter, IcebergWriterBuilder};
use iceberg::{TableCreation, TableIdent};
use parquet::file::properties::WriterProperties;

/// Identifier of the single snapshot the generated table contains.
const SNAPSHOT_ID: i64 = 1;

/// Namespace of the generated table, used as `SCHEMA_NAME`.
pub const LOCAL_SCHEMA_NAME: &str = "local_db";

/// Name of the generated table, used as `TABLE_NAME`.
pub const LOCAL_TABLE_NAME: &str = "local_table";

/// Name of the second column of the generated table.
pub const DEFAULT_VALUE_COLUMN: &str = "value";

/// An Iceberg table written to the local filesystem.
///
/// Created by [`LocalIcebergTable::create`], which returns the values a head or
/// worker node needs in order to load it: the metadata location, the namespace,
/// and the table name.
#[derive(Debug, Clone)]
pub struct LocalIcebergTable {
    metadata_location: String,
    data_file_paths: Vec<String>,
    row_count: u64,
}

impl LocalIcebergTable {
    /// Creates a table with an `id` (long) and a `value` (string) column, split
    /// across one Parquet data file per entry in `rows_per_file`.
    ///
    /// `warehouse_dir` must be an absolute path; Iceberg rejects relative
    /// locations when it infers the storage backend.
    pub async fn create(
        warehouse_dir: &Path,
        rows_per_file: &[usize],
    ) -> Result<Self, Box<dyn std::error::Error>> {
        Self::create_with_value_column(warehouse_dir, DEFAULT_VALUE_COLUMN, rows_per_file).await
    }

    /// Same as [`LocalIcebergTable::create`], but names the second column
    /// `value_column`.
    ///
    /// Naming it `cache_index` produces a table that collides with the index
    /// column the worker appends, which is otherwise awkward to construct.
    pub async fn create_with_value_column(
        warehouse_dir: &Path,
        value_column: &str,
        rows_per_file: &[usize],
    ) -> Result<Self, Box<dyn std::error::Error>> {
        if !warehouse_dir.is_absolute() {
            return Err(format!(
                "warehouse directory must be absolute, got {}",
                warehouse_dir.display()
            )
            .into());
        }

        let file_io = FileIOBuilder::new("file").build()?;
        let table_location = format!(
            "{}/{}/{}",
            path_to_file_uri(warehouse_dir),
            LOCAL_SCHEMA_NAME,
            LOCAL_TABLE_NAME
        );

        let schema = Schema::builder()
            .with_schema_id(0)
            .with_fields(vec![
                NestedField::required(1, "id", Type::Primitive(PrimitiveType::Long)).into(),
                NestedField::required(2, value_column, Type::Primitive(PrimitiveType::String))
                    .into(),
            ])
            .build()?;

        // The table is built without a catalog: `MemoryCatalog` cannot commit
        // snapshots, and the cache loads tables as `StaticTable` anyway, so only
        // the metadata file on disk matters.
        let metadata = TableMetadataBuilder::from_table_creation(
            TableCreation::builder()
                .name(LOCAL_TABLE_NAME.to_string())
                .location(table_location.clone())
                .schema(schema.clone())
                .build(),
        )?
        .build()?
        .metadata;

        let table = Table::builder()
            .identifier(TableIdent::from_strs([
                LOCAL_SCHEMA_NAME,
                LOCAL_TABLE_NAME,
            ])?)
            .metadata(metadata.clone())
            .file_io(file_io.clone())
            .build()?;

        // The Parquet writer is driven by the Iceberg schema, while the record
        // batches handed to it are built against the matching Arrow schema.
        let arrow_schema: ArrowSchemaRef = Arc::new(schema_to_arrow_schema(&schema)?);
        let iceberg_schema: IcebergSchemaRef = Arc::new(schema);
        let location_generator = DefaultLocationGenerator::new(table.metadata().clone())?;

        let mut data_files = Vec::new();
        let mut next_id: i64 = 0;

        for (file_index, rows) in rows_per_file.iter().enumerate() {
            let parquet_writer_builder = ParquetWriterBuilder::new(
                WriterProperties::builder().build(),
                iceberg_schema.clone(),
                table.file_io().clone(),
                location_generator.clone(),
                DefaultFileNameGenerator::new(
                    format!("part-{}", file_index),
                    None,
                    DataFileFormat::Parquet,
                ),
            );
            let mut writer = DataFileWriterBuilder::new(parquet_writer_builder, None, 0)
                .build()
                .await?;

            let ids: Vec<i64> = (0..*rows).map(|row| next_id + row as i64).collect();
            let values: Vec<String> = ids.iter().map(|id| format!("row-{}", id)).collect();
            writer
                .write(RecordBatch::try_new(
                    arrow_schema.clone(),
                    vec![
                        Arc::new(Int64Array::from(ids)),
                        Arc::new(StringArray::from(values)),
                    ],
                )?)
                .await?;

            data_files.extend(writer.close().await?);
            next_id += *rows as i64;
        }

        let metadata_location =
            commit_snapshot(&file_io, &metadata, &table_location, &data_files).await?;

        Ok(Self {
            metadata_location,
            data_file_paths: data_files
                .iter()
                .map(|data_file| data_file.file_path().to_string())
                .collect(),
            row_count: rows_per_file.iter().sum::<usize>() as u64,
        })
    }

    /// Location of the table metadata, for use as `METADATA_LOC`.
    pub fn metadata_location(&self) -> &str {
        &self.metadata_location
    }

    /// Namespace of the table, for use as `SCHEMA_NAME`.
    pub fn schema_name(&self) -> String {
        LOCAL_SCHEMA_NAME.to_string()
    }

    /// Name of the table, for use as `TABLE_NAME`.
    pub fn table_name(&self) -> String {
        LOCAL_TABLE_NAME.to_string()
    }

    /// Paths of the Parquet data files, in the order they were written.
    pub fn data_file_paths(&self) -> &[String] {
        &self.data_file_paths
    }

    /// Total number of rows across every data file.
    pub fn row_count(&self) -> u64 {
        self.row_count
    }
}

/// Writes a manifest, a manifest list and a metadata file describing a single
/// append of `data_files`, and returns the location of the metadata file.
///
/// This mirrors what `Transaction::fast_append` would do, without requiring a
/// catalog to commit through.
async fn commit_snapshot(
    file_io: &FileIO,
    metadata: &TableMetadata,
    table_location: &str,
    data_files: &[DataFile],
) -> Result<String, Box<dyn std::error::Error>> {
    let sequence_number = metadata.next_sequence_number();

    let manifest_path = format!("{}/metadata/{}-m0.avro", table_location, SNAPSHOT_ID);
    let mut manifest_writer = ManifestWriterBuilder::new(
        file_io.new_output(&manifest_path)?,
        Some(SNAPSHOT_ID),
        None,
        metadata.current_schema().clone(),
        metadata.default_partition_spec().as_ref().clone(),
    )
    .build_v2_data();
    for data_file in data_files {
        manifest_writer.add_file(data_file.clone(), sequence_number)?;
    }
    let manifest_file = manifest_writer.write_manifest_file().await?;

    let manifest_list_path = format!("{}/metadata/snap-{}.avro", table_location, SNAPSHOT_ID);
    let mut manifest_list_writer = ManifestListWriter::v2(
        file_io.new_output(&manifest_list_path)?,
        SNAPSHOT_ID,
        None,
        sequence_number,
    );
    manifest_list_writer.add_manifests(vec![manifest_file].into_iter())?;
    manifest_list_writer.close().await?;

    let snapshot = Snapshot::builder()
        .with_snapshot_id(SNAPSHOT_ID)
        .with_parent_snapshot_id(None)
        .with_sequence_number(sequence_number)
        .with_manifest_list(manifest_list_path)
        .with_schema_id(metadata.current_schema_id())
        .with_summary(Summary {
            operation: Operation::Append,
            additional_properties: HashMap::new(),
        })
        .with_timestamp_ms(SystemTime::now().duration_since(UNIX_EPOCH)?.as_millis() as i64)
        .build();

    let metadata = metadata
        .clone()
        .into_builder(None)
        .add_snapshot(snapshot)?
        .set_ref(
            MAIN_BRANCH,
            SnapshotReference::new(SNAPSHOT_ID, SnapshotRetention::branch(None, None, None)),
        )?
        .build()?
        .metadata;

    let metadata_location = format!("{}/metadata/v1.metadata.json", table_location);
    file_io
        .new_output(&metadata_location)?
        .write(serde_json::to_vec(&metadata)?.into())
        .await?;

    Ok(metadata_location)
}

/// Converts an absolute path into a `file://` URI.
///
/// Iceberg infers the storage backend by parsing the location as a URL, so a
/// Windows path such as `D:\warehouse` would otherwise be read as scheme `d`.
fn path_to_file_uri(path: &Path) -> String {
    let path = path.to_string_lossy();
    // `Path::canonicalize` returns extended-length paths on Windows.
    let path = path.strip_prefix(r"\\?\").unwrap_or(&path);
    let normalized = path.replace('\\', "/");

    if normalized.starts_with('/') {
        format!("file://{}", normalized)
    } else {
        format!("file:///{}", normalized)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn unix_path_becomes_file_uri() {
        assert_eq!(
            path_to_file_uri(Path::new("/tmp/warehouse")),
            "file:///tmp/warehouse"
        );
    }

    #[test]
    fn windows_path_becomes_file_uri() {
        assert_eq!(
            path_to_file_uri(Path::new(r"D:\tmp\warehouse")),
            "file:///D:/tmp/warehouse"
        );
    }

    #[test]
    fn windows_extended_length_prefix_is_stripped() {
        assert_eq!(
            path_to_file_uri(Path::new(r"\\?\D:\tmp\warehouse")),
            "file:///D:/tmp/warehouse"
        );
    }

    #[tokio::test]
    async fn rejects_relative_warehouse_dir() {
        let error = LocalIcebergTable::create(Path::new("relative/dir"), &[1])
            .await
            .expect_err("relative warehouse directory should be rejected");
        assert!(error.to_string().contains("must be absolute"));
    }
}
