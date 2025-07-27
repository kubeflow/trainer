use crate::config::config::CacheConfig;
use crate::head::provider::DataFileTableProvider;
use crate::head::writer::DistributedWriterExec;
use arrow::array::UInt64Array;
use arrow_schema::SchemaRef;
use datafusion::datasource::MemTable;
use datafusion::error::{DataFusionError, Result};
use datafusion::physical_expr::Partitioning;
use datafusion::physical_plan::execute_stream;
use datafusion::physical_plan::repartition::RepartitionExec;
use datafusion::prelude::SessionContext;
use datafusion::sql::TableReference;
use futures::StreamExt;
use std::collections::HashMap;
use std::sync::Arc;
use tracing::{error, info};

pub struct Distributor {
    ctx: Arc<SessionContext>,
    num_workers: usize,
    data_file_provider: Arc<DataFileTableProvider>,
    mem_table_name: String,
    worker_map: Arc<HashMap<String, String>>,
    arrow_schema: SchemaRef,
    pub(crate) total_row_count: u64,
    config: Arc<CacheConfig>,
}

impl Distributor {
    pub fn new(
        ctx: Arc<SessionContext>,
        num_workers: usize,
        data_file_provider: Arc<DataFileTableProvider>,
        mem_table_name: String,
        worker_map: Arc<HashMap<String, String>>,
        arrow_schema: SchemaRef,
        config: Arc<CacheConfig>,
    ) -> Self {
        Self {
            ctx,
            num_workers,
            data_file_provider,
            mem_table_name,
            worker_map,
            arrow_schema,
            total_row_count: 0,
            config,
        }
    }

    pub async fn init(&mut self) -> Result<()> {
        let _ = self.load_data_file_locs().await?;
        let df = self
            .ctx
            .sql(&format!("select * from {}", self.mem_table_name))
            .await?
            .collect()
            .await?;
        let _ = arrow::util::pretty::print_batches(&df);

        let df = self
            .ctx
            .sql(&format!(
                "SELECT MAX(row_end_indexes) AS max FROM {}",
                self.mem_table_name
            ))
            .await?;
        let results = df.collect().await?;

        if let Some(batch) = results.first() {
            let column = batch.column(0);
            let max_value = column
                .as_any()
                .downcast_ref::<UInt64Array>()
                .ok_or_else(|| {
                    DataFusionError::Execution("Failed to downcast to UInt64Array".to_string())
                })?
                .value(0);
            self.total_row_count = max_value + 1;
            info!("Total num of rows: {}", self.total_row_count);
        }

        self.distribute_data_files().await?;
        Ok(())
    }

    pub async fn get_workers_to_connect(&self, start: u64, end: u64) -> Result<Vec<String>> {
        info!("start: {}, end: {}", start, end);
        let df = self.ctx.sql(format!("SELECT worker_ids FROM {} WHERE row_start_indexes <= {} AND row_end_indexes >= {}", self.mem_table_name, end, start).as_str()).await?;
        let results = df.collect().await?;
        let _ = arrow::util::pretty::print_batches(&results);

        let mut string_results: Vec<String> = Vec::new();

        for batch in results {
            for row in 0..batch.num_rows() {
                let mut row_string = String::new();
                let array = batch.column(0);
                let value = array
                    .as_any()
                    .downcast_ref::<UInt64Array>()
                    .ok_or_else(|| {
                        DataFusionError::Execution("Failed to downcast to UInt64Array".to_string())
                    })?
                    .value(row);
                let url = self.worker_map.get(&value.to_string()).ok_or_else(|| {
                    DataFusionError::Execution(format!("Worker {} not found in worker map", value))
                })?;
                row_string.push_str(url);
                string_results.push(row_string);
            }
        }
        Ok(string_results)
    }

    async fn load_data_file_locs(&self) -> Result<()> {
        let memtable = MemTable::load(
            self.data_file_provider.clone(),
            Some(self.num_workers),
            &self.ctx.state(),
        )
        .await
        .map_err(|err: DataFusionError| {
            error!("Error loading table: {}", err);
            err
        })?;
        self.ctx
            .register_table(self.mem_table_name.clone(), Arc::new(memtable))
            .map_err(|err: DataFusionError| {
                error!("Failed to register table: {}", err);
                err
            })?;
        Ok(())
    }

    async fn distribute_data_files(&self) -> Result<()> {
        let table = self
            .ctx
            .table_provider(TableReference::parse_str(&self.mem_table_name))
            .await
            .map_err(|err: DataFusionError| {
                error!("Error retrieving table: {}", err);
                err
            })?;
        let plan = table.scan(&self.ctx.state(), None, &[], None).await?;
        let plan = RepartitionExec::try_new(plan, Partitioning::RoundRobinBatch(self.num_workers))?;
        let plan = DistributedWriterExec::new(
            Arc::new(plan),
            self.worker_map.clone(),
            self.arrow_schema.clone(),
            self.num_workers,
            self.config.clone(),
        );
        let _ = execute_stream(Arc::new(plan), self.ctx.task_ctx())?
            .collect::<Vec<_>>()
            .await;
        Ok(())
    }
}
