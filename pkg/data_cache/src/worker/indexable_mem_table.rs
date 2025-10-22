use arrow::array::RecordBatch;
use arrow_schema::SchemaRef;
use async_trait::async_trait;
use datafusion::catalog::{Session, TableProvider};
use datafusion::common::{Constraints, DataFusionError, ScalarValue, exec_err, plan_err};
use datafusion::datasource::TableType;
use datafusion::datasource::memory::MemorySourceConfig;
use datafusion::execution::SessionState;
use datafusion::logical_expr::{BinaryExpr, Expr, TableProviderFilterPushDown};
use datafusion::physical_plan::ExecutionPlan;
use futures::{StreamExt, TryStreamExt};
use std::any::Any;
use std::sync::Arc;
use tracing::{error, info};

#[derive(Debug)]
pub struct IndexableMemTable {
    schema: SchemaRef,
    pub(crate) batches: Vec<RecordBatch>,
    pub(crate) start_indices: Vec<u64>, // Starting row positions for each batch
    pub(crate) end_indices: Vec<u64>,   // Ending row positions for each batch
}

impl IndexableMemTable {
    pub fn try_new(
        schema: SchemaRef,
        partitions: Vec<Vec<RecordBatch>>,
        start_indices: Vec<u64>,
        end_indices: Vec<u64>,
    ) -> datafusion::common::Result<Self> {
        for batches in partitions.iter().flatten() {
            let batches_schema = batches.schema();
            if !schema.contains(&batches_schema) {
                error!(
                    "mem table schema does not contain batches schema. \
                        Target_schema: {schema:?}. Batches Schema: {batches_schema:?}"
                );
                return plan_err!("Mismatch between schema and batches");
            }
        }

        Ok(Self {
            schema,
            batches: partitions.into_iter().flatten().collect(),
            start_indices,
            end_indices,
        })
    }

    pub async fn load(
        t: Arc<dyn TableProvider>,
        _output_partitions: Option<usize>,
        _state: &SessionState,
        start_index: u64,
    ) -> datafusion::common::Result<Self> {
        let schema = t.schema();
        let exec = t.scan(_state, None, &[], None).await?;

        let mut data: Vec<RecordBatch> = vec![];
        let mut start_indices: Vec<u64> = vec![];
        let mut end_indices: Vec<u64> = vec![];

        let mut current_start = start_index;
        let stream = exec.execute(0, _state.task_ctx())?;
        let _ = stream
            .map_ok(|batch| {
                let num_rows = batch.num_rows() as u64;
                let current_end = current_start + num_rows - 1;

                start_indices.push(current_start);
                end_indices.push(current_end);
                data.push(batch);

                current_start = current_end + 1;
            })
            .collect::<Vec<_>>()
            .await;

        info!("Number of batches: {}", data.len());

        IndexableMemTable::try_new(Arc::clone(&schema), vec![data], start_indices, end_indices)
    }
}

async fn fetch_partitions(
    batches: Vec<RecordBatch>,
    start_indices: &[u64],
    end_indices: &[u64],
    start: u64,
    end: u64,
) -> Vec<RecordBatch> {
    if batches.is_empty() {
        return vec![];
    }

    let n = batches.len();

    // Find FIRST batch where batch_end >= start
    let first = end_indices.partition_point(|&batch_end| batch_end < start);

    // If all batches end before the query start, no overlap
    if first >= n {
        return vec![];
    }

    // Find LAST batch where batch_start <= end
    let last = start_indices
        .partition_point(|&batch_start| batch_start <= end)
        .saturating_sub(1);

    // Verify we have valid range
    if first > last {
        return vec![];
    }

    batches[first..=last].to_vec()
}

#[async_trait]
impl TableProvider for IndexableMemTable {
    fn as_any(&self) -> &dyn Any {
        self
    }

    fn schema(&self) -> SchemaRef {
        Arc::clone(&self.schema)
    }

    fn constraints(&self) -> Option<&Constraints> {
        None
    }

    fn table_type(&self) -> TableType {
        TableType::Base
    }

    async fn scan(
        &self,
        _state: &dyn Session,
        projection: Option<&Vec<usize>>,
        filters: &[Expr],
        _limit: Option<usize>,
    ) -> datafusion::common::Result<Arc<dyn ExecutionPlan>> {
        let (start, end) = if filters.len() == 1 {
            let start = collect_literals(&filters[0]).ok_or_else(|| {
                DataFusionError::Execution(
                    "Failed to extract start value from first filter".to_string(),
                )
            })?;
            (start, start)
        } else if filters.len() == 2 {
            let start = collect_literals(&filters[0]).ok_or_else(|| {
                DataFusionError::Execution(
                    "Failed to extract start value from first filter".to_string(),
                )
            })?;
            let end = collect_literals(&filters[1]).ok_or_else(|| {
                DataFusionError::Execution(
                    "Failed to extract end value from second filter".to_string(),
                )
            })?;
            (start, end)
        } else {
            return exec_err!("Incorrect filters");
        };
        let partitions = fetch_partitions(
            self.batches.clone(),
            &self.start_indices,
            &self.end_indices,
            start,
            end,
        )
        .await;

        let exec =
            MemorySourceConfig::try_new_exec(&[partitions], self.schema(), projection.cloned())?;

        Ok(exec)
    }

    fn supports_filters_pushdown(
        &self,
        filters: &[&Expr],
    ) -> datafusion::common::Result<Vec<TableProviderFilterPushDown>> {
        Ok(vec![TableProviderFilterPushDown::Inexact; filters.len()])
    }
}

fn collect_literals(expr: &Expr) -> Option<u64> {
    match expr {
        Expr::BinaryExpr(BinaryExpr {
            left: _,
            op: _,
            right,
        }) => {
            if let Expr::Literal(scalar) = &**right {
                if let ScalarValue::UInt64(Some(val)) = scalar {
                    return Some(*val);
                }
            }
            None
        }
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use arrow::array::UInt64Array;
    use arrow::{
        array::StringArray,
        datatypes::{DataType, Field, Schema},
        record_batch::RecordBatch,
    };
    use datafusion::assert_batches_eq;
    use datafusion::datasource::MemTable;
    use datafusion::prelude::{SessionConfig, SessionContext};
    use std::sync::Arc;

    fn create_schema() -> Arc<Schema> {
        Arc::new(Schema::new(vec![
            Field::new("id", DataType::UInt64, false),
            Field::new("name", DataType::Utf8, false),
        ]))
    }
    // Create sample RecordBatches for testing
    fn create_test_batches() -> Result<Vec<RecordBatch>, Box<dyn std::error::Error>> {
        let schema = create_schema();
        Ok(vec![
            RecordBatch::try_new(
                schema.clone(),
                vec![
                    Arc::new(UInt64Array::from(vec![0, 1, 2, 3])),
                    Arc::new(StringArray::from(vec!["A", "B", "C", "D"])),
                ],
            )?,
            RecordBatch::try_new(
                schema.clone(),
                vec![
                    Arc::new(UInt64Array::from(vec![4, 5, 6, 7])),
                    Arc::new(StringArray::from(vec!["E", "F", "G", "H"])),
                ],
            )?,
        ])
    }

    // #[test]
    // fn test_basic_and_condition_with_filter() {
    //     let expr = col("a").eq(lit(1u64)).and(col("b").eq(lit(2u64)));
    //     let df_schema = DFSchema::try_from(create_schema()).unwrap();
    //     let physical_expr = SessionContext::new().create_physical_expr(expr, &df_schema).unwrap();
    //     let a = collect_literals(&physical_expr).unwrap();
    //     let b = collect_literals(&physical_expr[1]).unwrap();
    //     assert_eq!(a, 1);
    //     assert_eq!(b, 2);
    // }

    #[tokio::test]
    async fn test_partition_selection() -> Result<(), Box<dyn std::error::Error>> {
        let batches = create_test_batches()?;
        let result = fetch_partitions(batches, &[0, 4], &[3, 7], 0, 2).await;

        assert_batches_eq!(
            [
                "+----+------+",
                "| id | name |",
                "+----+------+",
                "| 0  | A    |",
                "| 1  | B    |",
                "| 2  | C    |",
                "| 3  | D    |",
                "+----+------+"
            ],
            &result
        );
        Ok(())
    }

    #[tokio::test]
    async fn test_partition_selection_single_row() -> Result<(), Box<dyn std::error::Error>> {
        let batches = create_test_batches()?;
        let result = fetch_partitions(batches, &[0, 4], &[3, 7], 2, 2).await;

        assert_batches_eq!(
            [
                "+----+------+",
                "| id | name |",
                "+----+------+",
                "| 0  | A    |",
                "| 1  | B    |",
                "| 2  | C    |",
                "| 3  | D    |",
                "+----+------+"
            ],
            &result
        );
        Ok(())
    }

    #[tokio::test]
    async fn test_partition_selection_all() -> Result<(), Box<dyn std::error::Error>> {
        let batches = create_test_batches()?;
        let result = fetch_partitions(batches, &[0, 4], &[3, 7], 0, 7).await;

        assert_batches_eq!(
            [
                "+----+------+",
                "| id | name |",
                "+----+------+",
                "| 0  | A    |",
                "| 1  | B    |",
                "| 2  | C    |",
                "| 3  | D    |",
                "| 4  | E    |",
                "| 5  | F    |",
                "| 6  | G    |",
                "| 7  | H    |",
                "+----+------+"
            ],
            &result
        );
        Ok(())
    }

    #[tokio::test]
    async fn test_indexable_mem_table_with_scan() -> Result<(), Box<dyn std::error::Error>> {
        let schema = create_schema();
        let rb = create_test_batches()?;
        let mem_table = IndexableMemTable::try_new(schema, vec![rb], vec![0, 4], vec![3, 7])?;

        let ctx = SessionContext::new();
        ctx.register_table("test_table", Arc::new(mem_table))?;
        let df = ctx
            .sql("SELECT id, name FROM test_table where id >= 0 AND id <= 2")
            .await?;
        let result = df.collect().await?;
        assert_batches_eq!(
            [
                "+----+------+",
                "| id | name |",
                "+----+------+",
                "| 0  | A    |",
                "| 1  | B    |",
                "| 2  | C    |",
                "+----+------+"
            ],
            &result
        );
        Ok(())
    }

    #[tokio::test]
    async fn test_indexable_mem_table_with_load_and_scan() -> Result<(), Box<dyn std::error::Error>>
    {
        let schema = create_schema();
        let rb = create_test_batches()?;
        let config = SessionConfig::new().with_batch_size(1024);
        let ctx = SessionContext::new_with_config(config);
        let mem_table = MemTable::try_new(schema, vec![rb])?;
        let indexable_mem_table =
            IndexableMemTable::load(Arc::new(mem_table), None, &ctx.state(), 0).await?;

        ctx.register_table("test_table", Arc::new(indexable_mem_table))?;
        let df = ctx
            .sql("SELECT id, name FROM test_table where id >= 0 AND id <= 2")
            .await?;
        let result = df.collect().await?;
        assert_batches_eq!(
            [
                "+----+------+",
                "| id | name |",
                "+----+------+",
                "| 0  | A    |",
                "| 1  | B    |",
                "| 2  | C    |",
                "+----+------+"
            ],
            &result
        );
        Ok(())
    }

    #[tokio::test]
    async fn test_fetch_partitions_boundary_overlap() -> Result<(), Box<dyn std::error::Error>> {
        // This validates the fix for batches starting before query range but overlapping it
        // Batch 0: rows 0-3, Batch 1: rows 4-7
        // Query [2,5] should select BOTH batches (Batch 0 has 2-3, Batch 1 has 4-5)
        let batches = create_test_batches()?;
        let result = fetch_partitions(batches, &[0, 4], &[3, 7], 2, 5).await;

        assert_eq!(result.len(), 2, "Expected both batches for boundary query");
        Ok(())
    }
}
