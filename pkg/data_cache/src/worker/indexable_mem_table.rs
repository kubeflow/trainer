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
    pub(crate) indices: Vec<u64>, // Starting row positions for each batch to enable range queries
}

impl IndexableMemTable {
    pub fn try_new(
        schema: SchemaRef,
        partitions: Vec<Vec<RecordBatch>>,
        indices: Vec<u64>,
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
            indices,
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
        let mut indices: Vec<u64> = vec![];

        let mut start_index = start_index;
        let stream = exec.execute(0, _state.task_ctx())?;
        let _ = stream
            .map_ok(|batch| {
                indices.push(start_index);
                start_index += batch.num_rows() as u64;
                data.push(batch);
            })
            .collect::<Vec<_>>()
            .await;

        info!("Number of batches: {}", data.len());

        // let mut exec = MemoryExec::try_new(&[data], Arc::clone(&schema), None)?;
        // if let Some(cons) = constraints {
        //     exec = exec.with_constraints(cons.clone());
        // }
        //
        // if let Some(num_partitions) = output_partitions {
        //     // TODO: handle repartioning
        // }
        IndexableMemTable::try_new(Arc::clone(&schema), vec![data], indices)
    }
}

async fn fetch_partitions(
    batches: Vec<RecordBatch>,
    indices: &[u64],
    start: u64,
    end: u64,
) -> Vec<RecordBatch> {
    let start_index = indices.partition_point(|&x| x < start);
    let end_index = indices.partition_point(|&x| x <= end);

    if start_index < end_index {
        batches[start_index..end_index].to_owned()
    } else if start_index == end_index && end_index > 0 {
        vec![batches[end_index - 1].to_owned()]
    } else {
        vec![]
    }
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
            info!("{:?}", filters[0]);
            let start = collect_literals(&filters[0]).ok_or_else(|| {
                DataFusionError::Execution(
                    "Failed to extract start value from first filter".to_string(),
                )
            })?;
            (start, start)
        } else if filters.len() == 2 {
            info!("{:?}", filters[0]);
            info!("{:?}", filters[1]);
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
        let partitions = fetch_partitions(self.batches.clone(), &self.indices, start, end).await;

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
        let result = fetch_partitions(batches, &[0, 4], 0, 2).await;

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
        let result = fetch_partitions(batches, &[0, 4], 2, 2).await;

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
        let result = fetch_partitions(batches, &[0, 4], 0, 4).await;
        // println!("{}", &result.get(0).unwrap().num_rows());
        // println!("{}", &result.get(1).unwrap().num_rows());
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
        let mem_table = IndexableMemTable::try_new(schema, vec![rb], [0, 4].to_vec())?;

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
}
