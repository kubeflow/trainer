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

//! Integration tests for local on-disk Iceberg tables (issue #3174).

use futures::StreamExt;
use iceberg::TableIdent;
use iceberg::table::StaticTable;
use kubeflow_data_cache::config::file_io::build_file_io;
use std::path::{Path, PathBuf};
use std::process::Command;

fn repo_root() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .parent()
        .and_then(|p| p.parent())
        .expect("repo root")
        .to_path_buf()
}

fn fixture_metadata_dir() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("testdata/local_iceberg/warehouse/local/demo/metadata")
}

fn generate_fixture() {
    let script = repo_root().join("hack/data_cache/generate_local_iceberg_fixture.py");
    let status = Command::new("python3")
        .arg(&script)
        .status()
        .expect("failed to run python3");
    assert!(
        status.success(),
        "generate_local_iceberg_fixture.py failed; install pyiceberg pyarrow sqlalchemy"
    );
}

fn latest_metadata_path() -> Option<PathBuf> {
    let dir = fixture_metadata_dir();
    if !dir.is_dir() {
        return None;
    }
    let mut files: Vec<PathBuf> = std::fs::read_dir(&dir)
        .ok()?
        .filter_map(|e| e.ok())
        .map(|e| e.path())
        .filter(|p| {
            p.file_name()
                .and_then(|n| n.to_str())
                .is_some_and(|n| n.ends_with(".metadata.json"))
        })
        .collect();
    files.sort();
    files.pop()
}

fn metadata_loc_uri(path: &Path) -> String {
    format!(
        "file://{}",
        path.canonicalize()
            .expect("canonicalize metadata")
            .display()
    )
}

fn ensure_fixture_metadata() -> String {
    if latest_metadata_path().is_none() {
        generate_fixture();
    }
    let path = latest_metadata_path().expect("metadata file after generation");
    metadata_loc_uri(&path)
}

#[tokio::test]
async fn loads_local_iceberg_metadata_file() {
    let metadata_loc = ensure_fixture_metadata();
    let file_io = build_file_io(&metadata_loc).expect("build FileIO");
    let table_ident = TableIdent::from_strs(["local", "demo"]).expect("table ident");
    let static_table = StaticTable::from_metadata_file(&metadata_loc, table_ident, file_io)
        .await
        .expect("load static table");
    let table = static_table.into_table();
    let scan = table.scan().build().expect("scan");
    let mut file_count = 0u32;
    let mut stream = scan.plan_files().await.expect("plan files");
    while let Some(task) = stream.next().await {
        let _task = task.expect("file scan task");
        file_count += 1;
    }
    assert!(
        file_count > 0,
        "expected at least one data file in local fixture"
    );
}
