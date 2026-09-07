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

//! Generates an Iceberg table backed by local Parquet files.
//!
//! The Rust tests build the same table in a temporary directory, so this is only
//! needed to run a cache cluster by hand:
//!
//! ```bash
//! cargo run --features local-fixtures --example create_local_table -- /tmp/warehouse 3 2
//! ```
//!
//! The environment variables the `head` and `worker` binaries expect are printed
//! as `KEY=VALUE` lines, so the output can be sourced directly:
//!
//! ```bash
//! eval "$(cargo run -q --features local-fixtures --example create_local_table -- /tmp/warehouse)"
//! ```

use std::path::PathBuf;

use kubeflow_data_cache::testkit::local_table::LocalIcebergTable;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let mut args = std::env::args().skip(1);

    let warehouse_dir = match args.next() {
        Some(dir) => PathBuf::from(dir),
        None => {
            eprintln!("Usage: create_local_table <warehouse-dir> [rows-per-file ...]");
            eprintln!("Example: create_local_table /tmp/warehouse 3 2");
            std::process::exit(1);
        }
    };

    let rows_per_file: Vec<usize> = args
        .map(|rows| rows.parse::<usize>())
        .collect::<Result<Vec<_>, _>>()?;
    let rows_per_file = if rows_per_file.is_empty() {
        vec![3, 2]
    } else {
        rows_per_file
    };

    // Iceberg records absolute paths in its metadata, so the warehouse location
    // has to be resolved before the table is written.
    std::fs::create_dir_all(&warehouse_dir)?;
    let warehouse_dir = warehouse_dir.canonicalize()?;

    let table = LocalIcebergTable::create(&warehouse_dir, &rows_per_file).await?;

    eprintln!(
        "Created Iceberg table with {} rows across {} data file(s) under {}",
        table.row_count(),
        rows_per_file.len(),
        warehouse_dir.display()
    );

    println!("export METADATA_LOC='{}'", table.metadata_location());
    println!("export SCHEMA_NAME='{}'", table.schema_name());
    println!("export TABLE_NAME='{}'", table.table_name());

    Ok(())
}
