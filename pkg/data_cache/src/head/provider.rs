use arrow::array::{ArrayRef, GenericListBuilder, RecordBatch, StringViewBuilder, UInt64Array};
use arrow_schema::SchemaRef;
use async_trait::async_trait;
use datafusion::catalog::Session;
use datafusion::datasource::{TableProvider, TableType};
use datafusion::error::{DataFusionError, Result};
use datafusion::execution::{SendableRecordBatchStream, TaskContext};
use datafusion::logical_expr::Expr;
use datafusion::physical_expr::{EquivalenceProperties, Partitioning};
use datafusion::physical_plan::execution_plan::{Boundedness, EmissionType};
use datafusion::physical_plan::stream::RecordBatchStreamAdapter;
use datafusion::physical_plan::{
    DisplayAs, DisplayFormatType, EmptyRecordBatchStream, ExecutionPlan, PlanProperties,
};
use futures::StreamExt;
use futures::stream::iter;
use iceberg::TableIdent;
use iceberg::expr::Predicate;
use iceberg::io::FileIO;
use iceberg::scan::{FileScanTask, FileScanTaskStream};
use iceberg::table::{StaticTable, Table};
use std::any::Any;
use std::fmt::Formatter;
use std::sync::Arc;
use tracing::{debug, info};

/// Table provider for distributed Arrow caching system that coordinates data
/// distribution across multiple worker nodes.
///
/// This table provider implements the head node functionality in a distributed
/// Arrow-based caching system. It loads Iceberg table metadata and partitions
/// data files across worker nodes for parallel processing.
///
/// # Architecture
///
/// The provider uses a head-worker architecture where:
/// - Head node (this provider) coordinates data distribution
/// - Worker nodes execute queries on partitioned data subsets
/// - Data is partitioned based on file scan tasks and record counts
///
/// # Data Flow
///
/// ```text
/// ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
/// │  Iceberg Table  │───▶│ DataFileTable   │───▶│ DataFileTable   │
/// │   Metadata      │    │   Provider      │    │      Exec       │
/// └─────────────────┘    └─────────────────┘    └─────────────────┘
///                                 │                       │
///                                 ▼                       ▼
///                        ┌─────────────────┐    ┌─────────────────┐
///                        │  Partition      │    │  Worker Task    │
///                        │   Planning      │    │   Distribution  │
///                        └─────────────────┘    └─────────────────┘
/// ```
///
/// # Performance Considerations
///
/// - Files are sorted by record count and distributed using a greedy algorithm
/// - Task groups are balanced to minimize data skew across workers
/// - Memory usage scales with the number of files and partitions
///
/// # See Also
///
/// - [`DataFileTableExec`]: The execution plan produced by this provider
/// - [`TaskGroup`]: Groups of file scan tasks assigned to worker nodes
#[derive(Debug, Clone)]
pub struct DataFileTableProvider {
    inner: Table,
    schema: SchemaRef,
    num_workers: usize,
}
impl DataFileTableProvider {
    pub(crate) async fn new(
        metadata_loc: &String,
        table_name: &String,
        schema_name: &String,
        arrow_schema: SchemaRef,
        num_workers: usize,
    ) -> Result<Self, Box<dyn std::error::Error>> {
        let file_io = FileIO::from_path(metadata_loc)
            .map_err(|e| format!("Failed to create FileIO: {}", e))?
            .build()
            .map_err(|e| format!("Failed to build FileIO: {}", e))?;
        let table_indent = TableIdent::from_strs([schema_name, table_name])
            .map_err(|e| format!("Failed to create table ident: {}", e))?;
        let static_table = StaticTable::from_metadata_file(metadata_loc, table_indent, file_io)
            .await
            .map_err(|e| format!("Failed to load static table: {}", e))?;
        let table = static_table.into_table();
        Ok(Self {
            inner: table,
            schema: arrow_schema,
            num_workers,
        })
    }
}
#[async_trait]
impl TableProvider for DataFileTableProvider {
    fn as_any(&self) -> &dyn Any {
        self
    }

    fn schema(&self) -> SchemaRef {
        self.schema.clone()
    }

    fn table_type(&self) -> TableType {
        TableType::Base
    }

    async fn scan(
        &self,
        _state: &dyn Session,
        _projection: Option<&Vec<usize>>,
        _filters: &[Expr],
        _limit: Option<usize>,
    ) -> Result<Arc<dyn ExecutionPlan>> {
        let tablescan = self.inner.scan().build().map_err(|e| {
            DataFusionError::Execution(format!("Failed to build table scan: {}", e))
        })?;
        let partitions = partition_tasks(
            tablescan
                .plan_files()
                .await
                .map_err(|e| DataFusionError::Execution(format!("Failed to plan files: {}", e)))?,
            self.num_workers,
        )
        .await
        .map_err(|e| DataFusionError::Execution(format!("Failed to partition tasks: {}", e)))?;
        info!("Partition count: {}", partitions.len());
        Ok(Arc::new(DataFileTableExec::new(
            self.schema.clone(),
            partitions,
        )))
    }
}

/// Execution plan for distributed data file processing in Arrow caching system.
///
/// This execution plan coordinates the distribution of file scan tasks across
/// multiple worker nodes and produces RecordBatches containing worker assignment
/// information and file paths for distributed processing.
///
/// # Algorithm
///
/// 1. Load file scan tasks from Iceberg table metadata
/// 2. Partition tasks across worker nodes using greedy load balancing
/// 3. Generate RecordBatches with worker assignments and file paths
/// 4. Each partition contains tasks for a specific worker node
///
/// # Output Schema
///
/// The execution plan produces RecordBatches with the following columns:
/// - `worker_ids`: Array of worker node identifiers
/// - `row_start_indexes`: Starting row indices for each task group
/// - `row_end_indexes`: Ending row indices for each task group
/// - `file_paths`: Lists of file paths assigned to each worker
///
/// # Partitioning Strategy
///
/// Tasks are distributed across workers using a greedy algorithm:
/// - Sort tasks by record count (descending)
/// - Assign each task to the worker with the least total records
/// - This minimizes data skew and balances workload
///
/// # Performance Considerations
///
/// - Each partition corresponds to one worker node
/// - Task assignment is computed once during planning
/// - Memory usage is proportional to number of files and workers
/// - No network communication during execution (coordination only)
///
/// # See Also
///
/// - [`DataFileTableProvider`]: The table provider that creates this execution plan
/// - [`TaskGroup`]: Groups of file scan tasks assigned to worker nodes
/// - [`RecordBatchBuilder`]: Builds output RecordBatches with task assignments
#[derive(Debug, Clone)]
pub struct DataFileTableExec {
    _projection: Option<Vec<String>>,
    _predicates: Option<Predicate>,
    schema: SchemaRef,
    partitions: Arc<Vec<TaskGroup>>,
    plan_properties: PlanProperties,
}

impl DataFileTableExec {
    fn new(schema: SchemaRef, partitions: Arc<Vec<TaskGroup>>) -> Self {
        // TODO:// revisit plan_properties
        let eq_properties = EquivalenceProperties::new_with_orderings(schema.clone(), &[]);
        let plan_properties = PlanProperties::new(
            eq_properties,
            Partitioning::UnknownPartitioning(partitions.len()), // TODO:// address partitioning during scan
            EmissionType::Both,
            Boundedness::Bounded,
        );
        Self {
            _projection: None,
            _predicates: None,
            schema: schema.clone(),
            partitions,
            plan_properties,
        }
    }
}

impl DisplayAs for DataFileTableExec {
    fn fmt_as(&self, t: DisplayFormatType, f: &mut Formatter) -> std::fmt::Result {
        match t {
            DisplayFormatType::Default | DisplayFormatType::Verbose => {
                write!(f, "DataFileTableExec: partitions={}", self.partitions.len())
            }
            DisplayFormatType::TreeRender => {
                write!(f, "partitions={}", self.partitions.len())
            }
        }
    }
}

#[async_trait]
impl ExecutionPlan for DataFileTableExec {
    fn name(&self) -> &str {
        "DataFileTableExec"
    }

    fn as_any(&self) -> &dyn Any {
        self
    }

    fn properties(&self) -> &PlanProperties {
        &self.plan_properties
    }

    fn children(&self) -> Vec<&Arc<dyn ExecutionPlan + 'static>> {
        vec![]
    }

    fn with_new_children(
        self: Arc<Self>,
        _children: Vec<Arc<dyn ExecutionPlan>>,
    ) -> Result<Arc<dyn ExecutionPlan>> {
        Ok(self)
    }

    fn execute(
        &self,
        _partition: usize,
        _context: Arc<TaskContext>,
    ) -> Result<SendableRecordBatchStream> {
        let mut builder = RecordBatchBuilder::new();
        // for (index, item) in self.partitions.iter().enumerate() { //support partitions into multiple batches
        //     builder.add_task(index, item);
        // }
        if _partition < self.partitions.len() {
            builder.add_task(
                _partition,
                self.partitions.get(_partition).ok_or_else(|| {
                    DataFusionError::Execution(format!("Partition {} not found", _partition))
                })?,
            )?;
            let record_batch_stream = builder.build();
            create_multi_batch_stream(self.schema.clone(), vec![record_batch_stream])
        } else {
            Ok(Box::pin(EmptyRecordBatchStream::new(self.schema.clone())))
        }
    }
}

fn create_multi_batch_stream(
    schema: SchemaRef,
    batches: Vec<Result<RecordBatch>>,
) -> Result<SendableRecordBatchStream> {
    let stream = iter(batches);
    Ok(Box::pin(RecordBatchStreamAdapter::new(schema, stream)))
}
#[derive(Debug, Default)]
struct RecordBatchBuilder {
    worker_ids: Vec<u64>,
    file_paths: Vec<Vec<String>>,
    row_start_indexes: Vec<u64>,
    row_end_indexes: Vec<u64>,
}

impl RecordBatchBuilder {
    fn new() -> Self {
        Self::default()
    }
    fn add_task(&mut self, index: usize, group: &TaskGroup) -> Result<()> {
        if group.tasks.is_empty() {
            return Ok(());
        }
        let mut _row_count = 0;
        let mut file_paths = Vec::new();
        for task in group.tasks.iter() {
            file_paths.push(task.data_file_path.clone());
            _row_count += task.record_count.ok_or_else(|| {
                DataFusionError::Execution("Task record count is None".to_string())
            })? //check if filescantask doesnt have record_count. IcebergFileScan always includes complete datafile
        }
        self.file_paths.push(file_paths);
        self.row_start_indexes.push(group.start_index as u64);
        self.row_end_indexes.push(group.end_index as u64);
        self.worker_ids.push(index as u64);
        Ok(())
    }

    fn build(self) -> Result<RecordBatch> {
        let worker_ids: ArrayRef = Arc::new(UInt64Array::from(self.worker_ids));
        let row_start_indexes: ArrayRef = Arc::new(UInt64Array::from(self.row_start_indexes));
        let row_end_indexes: ArrayRef = Arc::new(UInt64Array::from(self.row_end_indexes));

        let worker_list_builder = StringViewBuilder::new();
        let mut file_paths_builder =
            GenericListBuilder::<i32, StringViewBuilder>::new(worker_list_builder);

        debug!("file_paths: {:?}", self.file_paths);

        for paths in self.file_paths {
            for path in paths {
                file_paths_builder.values().append_value(path);
            }
            file_paths_builder.append(true)
        }

        let file_paths = Arc::new(file_paths_builder.finish());
        let rb = RecordBatch::try_from_iter(vec![
            ("worker_ids", worker_ids),
            ("row_start_indexes", row_start_indexes),
            ("row_end_indexes", row_end_indexes),
            ("file_paths", file_paths),
        ])?;
        Ok(rb)
    }
}

/// Groups of file scan tasks assigned to a specific worker node in the distributed system.
///
/// Each TaskGroup represents a logical partition of work that will be executed by
/// a single worker node. Tasks are grouped to balance workload and minimize data
/// skew across the distributed system.
///
/// # Fields
///
/// - `tasks`: File scan tasks assigned to this worker
/// - `start_index`: Global starting row index for this task group
/// - `end_index`: Global ending row index for this task group
///
/// # Row Index Calculation
///
/// Row indices provide a global ordering across all task groups:
/// - Group 0: rows 0 to (count-1)
/// - Group 1: rows count to (count + next_count - 1)
/// - And so on...
///
/// This enables consistent row numbering across distributed workers.
#[derive(Debug, Clone)]
struct TaskGroup {
    tasks: Vec<FileScanTask>,
    start_index: usize,
    end_index: usize,
}

async fn partition_tasks(
    mut stream: FileScanTaskStream,
    num_groups: usize,
) -> Result<Arc<Vec<TaskGroup>>, Box<dyn std::error::Error>> {
    let mut tasks: Vec<FileScanTask> = Vec::new();
    while let Some(result) = stream.next().await {
        match result {
            Ok(task) => {
                tasks.push(task);
            }
            Err(_e) => {
                // Handle the error
            }
        }
    }

    let mut groups: Vec<TaskGroup> = vec![
        TaskGroup {
            tasks: Vec::new(),
            start_index: 0,
            end_index: 0
        };
        num_groups
    ];
    let mut group_sizes: Vec<usize> = vec![0; num_groups];

    tasks.sort_by_key(|task| std::cmp::Reverse(task.record_count));

    for task in tasks {
        let min_group_index = group_sizes
            .iter()
            .enumerate()
            .min_by_key(|&(_, size)| size)
            .map(|(index, _)| index)
            .ok_or_else(|| format!("Failed to find minimum group"))?;

        let record_count =
            task.record_count
                .ok_or_else(|| format!("Task record count is None"))? as usize;

        group_sizes[min_group_index] += record_count;
        groups[min_group_index].tasks.push(task);
    }

    let mut end = 0;
    for (i, elem) in groups.iter_mut().enumerate() {
        let start = end;
        end += group_sizes[i];
        elem.start_index = start;
        elem.end_index = if end > 0 { end - 1 } else { 0 };
    }
    Ok(Arc::new(groups))
}

#[cfg(test)]
mod tests {
    use super::*;
    use arrow::datatypes::{DataType, Field, Schema};
    use datafusion::prelude::SessionContext;
    use iceberg::spec::{DataContentType, DataFileFormat};

    fn create_test_schema() -> SchemaRef {
        Arc::new(Schema::new(vec![
            Field::new("worker_ids", DataType::UInt64, false),
            Field::new("row_start_indexes", DataType::UInt64, false),
            Field::new("row_end_indexes", DataType::UInt64, false),
            Field::new(
                "file_paths",
                DataType::List(Arc::new(Field::new("item", DataType::Utf8View, false))),
                false,
            ),
        ]))
    }

    fn create_iceberg_schema() -> Arc<iceberg::spec::Schema> {
        Arc::new(
            iceberg::spec::Schema::builder()
                .with_schema_id(1)
                .build()
                .unwrap(),
        )
    }

    fn create_test_file_scan_task_with_record_count(
        record_count: u64,
        file_path: &str,
    ) -> FileScanTask {
        FileScanTask {
            start: 0,
            length: record_count,
            record_count: Some(record_count),
            data_file_path: String::from(file_path),
            data_file_content: DataContentType::Data,
            schema: create_iceberg_schema(),
            project_field_ids: vec![],
            predicate: None,
            data_file_format: DataFileFormat::Parquet,
            deletes: vec![],
        }
    }

    #[tokio::test]
    async fn test_data_file_table_exec_invalid_partition() -> Result<(), Box<dyn std::error::Error>>
    {
        let schema = create_test_schema();
        let partitions = Arc::new(vec![]);

        let exec = DataFileTableExec::new(schema.clone(), partitions);
        let session_ctx = SessionContext::new();
        let task_ctx = session_ctx.task_ctx();

        let result = exec.execute(0, task_ctx);
        assert!(result.is_ok());
        Ok(())
    }

    #[tokio::test]
    async fn test_partition_tasks_empty_stream() -> Result<(), Box<dyn std::error::Error>> {
        use futures::stream;

        let empty_stream = stream::empty();
        let result = partition_tasks(Box::pin(empty_stream), 2).await?;

        assert_eq!(result.len(), 2);
        assert!(result[0].tasks.is_empty());
        assert!(result[1].tasks.is_empty());
        assert_eq!(result[0].start_index, 0);
        assert_eq!(result[0].end_index, 0);
        assert_eq!(result[1].start_index, 0);
        assert_eq!(result[1].end_index, 0);

        Ok(())
    }

    #[tokio::test]
    async fn test_partition_tasks_with_stream_errors() -> Result<(), Box<dyn std::error::Error>> {
        use futures::stream;

        // Test the error handling path in partition_tasks
        let mixed_stream = stream::iter(vec![
            Err(iceberg::Error::new(
                iceberg::ErrorKind::DataInvalid,
                "Test error 1",
            )),
            Err(iceberg::Error::new(
                iceberg::ErrorKind::DataInvalid,
                "Test error 2",
            )),
        ]);

        let result = partition_tasks(Box::pin(mixed_stream), 2).await?;

        assert_eq!(result.len(), 2);
        assert!(result[0].tasks.is_empty());
        assert!(result[1].tasks.is_empty());

        Ok(())
    }
    #[tokio::test]
    async fn test_partition_tasks_validates_start_end_indices_empty()
    -> Result<(), Box<dyn std::error::Error>> {
        use futures::stream;

        let empty_stream = stream::empty();
        let num_groups = 3;

        let result = partition_tasks(Box::pin(empty_stream), num_groups).await?;

        // Verify we have the expected number of groups
        assert_eq!(result.len(), num_groups);

        // Verify start and end indices for empty groups
        for (group_idx, group) in result.iter().enumerate() {
            assert_eq!(
                group.start_index, 0,
                "Empty group {} should have start_index 0, got {}",
                group_idx, group.start_index
            );

            assert_eq!(
                group.end_index, 0,
                "Empty group {} should have end_index 0, got {}",
                group_idx, group.end_index
            );

            assert!(
                group.tasks.is_empty(),
                "Empty group {} should have no tasks",
                group_idx
            );
        }

        Ok(())
    }

    #[tokio::test]
    async fn test_partition_tasks_validates_start_end_indices_varying_groups()
    -> Result<(), Box<dyn std::error::Error>> {
        use futures::stream;

        // Test Case 1: Single group with multiple files
        let single_group_tasks = vec![
            create_test_file_scan_task_with_record_count(1000, "/test/file1.parquet"),
            create_test_file_scan_task_with_record_count(500, "/test/file2.parquet"),
            create_test_file_scan_task_with_record_count(750, "/test/file3.parquet"),
        ];
        let single_stream = stream::iter(single_group_tasks.into_iter().map(Ok));
        let single_result = partition_tasks(Box::pin(single_stream), 1).await?;

        assert_eq!(single_result.len(), 1);
        assert_eq!(single_result[0].tasks.len(), 3);

        // Calculate total records and verify indices
        let total_records: usize = single_result[0]
            .tasks
            .iter()
            .map(|t| t.record_count.unwrap_or(0) as usize)
            .sum();
        assert_eq!(total_records, 2250); // 1000 + 500 + 750
        assert_eq!(single_result[0].start_index, 0);
        assert_eq!(single_result[0].end_index, 2249); // 2250 - 1

        // Test Case 2: Two groups
        let two_group_tasks = vec![
            create_test_file_scan_task_with_record_count(2000, "/test/file1.parquet"),
            create_test_file_scan_task_with_record_count(1500, "/test/file2.parquet"),
            create_test_file_scan_task_with_record_count(1000, "/test/file3.parquet"),
            create_test_file_scan_task_with_record_count(500, "/test/file4.parquet"),
        ];
        let two_stream = stream::iter(two_group_tasks.into_iter().map(Ok));
        let two_result = partition_tasks(Box::pin(two_stream), 2).await?;

        assert_eq!(two_result.len(), 2);

        // Calculate records per group and verify distribution
        let mut group_records = vec![0; 2];
        for (group_idx, group) in two_result.iter().enumerate() {
            for task in &group.tasks {
                if let Some(count) = task.record_count {
                    group_records[group_idx] += count as usize;
                }
            }
        }

        let total_records: usize = group_records.iter().sum();
        assert_eq!(total_records, 5000); // 2000 + 1500 + 1000 + 500

        // Verify start and end indices are calculated correctly
        let mut expected_start = 0;
        for (group_idx, group) in two_result.iter().enumerate() {
            assert_eq!(
                group.start_index, expected_start,
                "Group {} start_index should be {}, got {}",
                group_idx, expected_start, group.start_index
            );

            if group_records[group_idx] > 0 {
                let expected_end = expected_start + group_records[group_idx] - 1;
                assert_eq!(
                    group.end_index, expected_end,
                    "Group {} end_index should be {}, got {}",
                    group_idx, expected_end, group.end_index
                );

                let range_size = group.end_index - group.start_index + 1;
                assert_eq!(
                    range_size, group_records[group_idx],
                    "Group {} range size ({}) should match record count ({})",
                    group_idx, range_size, group_records[group_idx]
                );
            }

            expected_start += group_records[group_idx];
        }

        Ok(())
    }
}
