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

use crate::worker::indexable_mem_table::IndexableMemTable;
use crate::worker::worker_datasource::WorkerDataSource;
use datafusion::common::DataFusionError;
use datafusion::prelude::SessionContext;
use std::sync::Arc;
use tracing::{error, info};

pub struct DataLoader {
    data_source: Arc<WorkerDataSource>,
}

impl DataLoader {
    pub(crate) async fn new(
        metadata_loc: String,
        table_name: String,
        schema_name: String,
        file_urls: Vec<String>,
        start_index: u64,
    ) -> Result<Self, Box<dyn std::error::Error>> {
        let data_source = WorkerDataSource::new(
            metadata_loc,
            table_name,
            schema_name,
            file_urls,
            start_index,
        )
        .await?;
        Ok(Self {
            data_source: Arc::new(data_source),
        })
    }
    pub(crate) async fn load_data(
        &self,
        ctx: &SessionContext,
        table: &str,
        start_index: u64,
    ) -> datafusion::common::Result<()> {
        let memtable =
            IndexableMemTable::load(self.data_source.clone(), None, &ctx.state(), start_index)
                .await
                .map_err(|error: DataFusionError| {
                    error!("Error loading table: {:?}", error);
                    error
                })?;
        ctx.register_table(table, Arc::new(memtable))
            .map_err(|e| DataFusionError::Execution(format!("Failed to register table: {}", e)))?;
        info!("loaded and registered table");
        Ok(())
    }
}
