use crate::config::config::CACHE_INDEX_COLUMN;
use arrow::array::UInt64Array;
use arrow::record_batch::RecordBatch;
use arrow_schema::{DataType, Field, Schema, SchemaBuilder, SchemaRef};
use async_trait::async_trait;
use datafusion::catalog::{Session, TableProvider};
use datafusion::datasource::TableType;
use datafusion::error::Result;
use datafusion::execution::{SendableRecordBatchStream, TaskContext};
use datafusion::logical_expr::Expr;
use datafusion::physical_expr::EquivalenceProperties;
use datafusion::physical_plan::execution_plan::{Boundedness, EmissionType};
use datafusion::physical_plan::stream::RecordBatchStreamAdapter;
use datafusion::physical_plan::{
    DisplayAs, DisplayFormatType, ExecutionPlan, PlanProperties, RecordBatchStream,
};
use futures::{Stream, StreamExt, TryStreamExt};
use iceberg::TableIdent;
use iceberg::arrow::schema_to_arrow_schema;
use iceberg::io::FileIO;
use iceberg::scan::{FileScanTask, FileScanTaskStream};
use iceberg::table::{StaticTable, Table};
use iceberg_datafusion::{from_datafusion_error, to_datafusion_error};
use std::any::Any;
use std::fmt::{Debug, Formatter};
use std::future;
use std::pin::Pin;
use std::sync::Arc;
use std::task::{Context, Poll};
use tracing::info;

/// Worker node data source for distributed Arrow caching system.
///
/// This table provider implements the worker node functionality in a distributed
/// Arrow-based caching system. It loads specific data files assigned by the head
/// node and adds cache indexing to enable efficient data retrieval.
///
/// # Architecture
///
/// The worker data source operates as part of a head-worker architecture:
/// - Head node assigns specific file URLs to this worker
/// - Worker loads only the assigned data files from Iceberg tables
/// - Adds a `cache_index` column for global row ordering
/// - Supports streaming data processing with bounded memory usage
///
/// # Data Flow
///
/// ```text
/// ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
/// │   Head Node     │───▶│ WorkerDataSource│───▶│   IndexColumn   │
/// │ (assigns files) │    │                 │    │      Exec       │
/// └─────────────────┘    └─────────────────┘    └─────────────────┘
///                                 │                       │
///                                 ▼                       ▼
///                        ┌─────────────────┐    ┌─────────────────┐
///                        │   WorkerExec    │    │  Row Numbering  │
///                        │ (loads files)   │    │   (cache_index) │
///                        └─────────────────┘    └─────────────────┘
/// ```
///
/// # Schema Enhancement
///
/// The worker adds a `cache_index` column to the original table schema:
/// - Original columns remain unchanged
/// - `cache_index` provides global row ordering across all workers
/// - Starting index is provided by the head node for consistent numbering
///
/// # Performance Considerations
///
/// - Only loads files assigned to this worker (reduces I/O)
/// - Streams data to minimize memory footprint
/// - Maintains global row ordering for distributed queries
/// - Uses Iceberg's native file filtering capabilities
///
/// # See Also
///
/// - [`WorkerExec`]: Execution plan for loading assigned data files
/// - [`IndexColumnExec`]: Execution plan for adding cache index column
/// - [`RowNumberStream`]: Stream processor for row numbering
pub struct WorkerDataSource {
    file_urls: Vec<String>,
    start_index: u64,
    inner: Table,
    output_schema: SchemaRef,
    table_schema: SchemaRef,
}

impl WorkerDataSource {
    pub(crate) async fn new(
        metadata_loc: String,
        table_name: String,
        schema_name: String,
        file_urls: Vec<String>,
        start_index: u64,
    ) -> Result<Self, Box<dyn std::error::Error>> {
        let file_io = FileIO::from_path(&metadata_loc)
            .map_err(|e| format!("Failed to create FileIO: {}", e))?
            .build()
            .map_err(|e| format!("Failed to build FileIO: {}", e))?;
        let table_indent = TableIdent::from_strs([schema_name, table_name])
            .map_err(|e| format!("Failed to create table ident: {}", e))?;
        let static_table =
            StaticTable::from_metadata_file(&metadata_loc, table_indent, file_io.clone())
                .await
                .map_err(|e| format!("Failed to load static table: {}", e))?;
        let table = static_table.into_table();
        let schema = Arc::new(
            schema_to_arrow_schema(table.metadata().current_schema())
                .map_err(|e| format!("Failed to convert schema: {}", e))?,
        );

        let fields = schema.fields().clone();
        let mut builder = SchemaBuilder::from(&fields);
        builder.push(Field::new(CACHE_INDEX_COLUMN, DataType::UInt64, false)); // TODO:// validate name collision
        let output_schema = Arc::new(Schema::new(builder.finish().fields));
        Ok(Self {
            file_urls,
            start_index,
            inner: table,
            output_schema,
            table_schema: schema,
        })
    }
}

impl Debug for WorkerDataSource {
    fn fmt(&self, _f: &mut Formatter<'_>) -> std::fmt::Result {
        todo!()
    }
}

#[async_trait]
impl TableProvider for WorkerDataSource {
    fn as_any(&self) -> &dyn Any {
        self
    }

    fn schema(&self) -> SchemaRef {
        self.output_schema.clone()
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
    ) -> datafusion::error::Result<Arc<dyn ExecutionPlan>> {
        info!("creating exec to fetch data from iceberg table");
        let iceberg_exec = WorkerExec::new(
            self.file_urls.clone(),
            self.inner.clone(),
            self.table_schema.clone(),
        );
        Ok(Arc::new(IndexColumnExec::new(
            Arc::new(iceberg_exec),
            self.output_schema.clone(),
            self.start_index,
        )))

        //TODO: Support scanning with subset of datafiles
        //TODO: Support projection with selected columns - Exec init computes projected schema and add its to props. Converts to column names and passes it to exec
    }
}

/// Execution plan for loading assigned data files on worker nodes.
///
/// This execution plan reads specific data files assigned by the head node from
/// an Iceberg table. It filters the table's file scan tasks to only process files
/// that have been assigned to this worker node.
///
/// # Algorithm
///
/// 1. Build an Iceberg table reader from metadata
/// 2. Plan all available files from the table
/// 3. Filter file scan tasks to only include assigned file URLs
/// 4. Stream data from the filtered files
///
/// # File Filtering
///
/// The execution plan only processes files that match the assigned file URLs:
/// - Reduces I/O by avoiding unnecessary file reads
/// - Maintains data locality and distribution as planned by head node
/// - Uses Iceberg's built-in file filtering capabilities
///
/// # Performance Considerations
///
/// - Single partition output (worker processes assigned files sequentially)
/// - Streaming execution to minimize memory usage
/// - Respects data file concurrency limits for controlled resource usage
/// - Filters at the file level before reading data
///
/// # See Also
///
/// - [`WorkerDataSource`]: The table provider that creates this execution plan
/// - [`IndexColumnExec`]: Wraps this plan to add cache indexing
/// - [`read_stream`]: Async function that performs the actual file reading
pub struct WorkerExec {
    file_urls: Vec<String>,
    inner: Table,
    schema: SchemaRef,
    plan_properties: PlanProperties,
}

impl WorkerExec {
    fn new(file_urls: Vec<String>, inner: Table, schema: SchemaRef) -> Self {
        let eq_properties = EquivalenceProperties::new_with_orderings(schema.clone(), &[]);
        let plan_properties = PlanProperties::new(
            eq_properties, // Equivalence Properties
            datafusion::physical_expr::Partitioning::UnknownPartitioning(1), // Output Partitioning
            EmissionType::Both,
            Boundedness::Bounded, // Execution Mode
        );
        Self {
            file_urls,
            inner,
            schema,
            plan_properties,
        }
    }
}

impl Debug for WorkerExec {
    fn fmt(&self, _f: &mut Formatter<'_>) -> std::fmt::Result {
        todo!()
    }
}

impl DisplayAs for WorkerExec {
    fn fmt_as(&self, t: DisplayFormatType, f: &mut Formatter) -> std::fmt::Result {
        match t {
            DisplayFormatType::Default | DisplayFormatType::Verbose => {
                write!(f, "WorkerExec: files={}", self.file_urls.len())
            }
            DisplayFormatType::TreeRender => {
                write!(f, "files={}", self.file_urls.len())
            }
        }
    }
}

impl ExecutionPlan for WorkerExec {
    fn name(&self) -> &str {
        "WorkerExec"
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
        let stream = futures::stream::once(read_stream(self.inner.clone(), self.file_urls.clone()))
            .try_flatten();
        Ok(Box::pin(RecordBatchStreamAdapter::new(
            self.schema.clone(),
            stream,
        )))
    }
}

/// Execution plan that adds a cache index column to incoming data streams.
///
/// This execution plan wraps another execution plan and adds a monotonically
/// increasing `cache_index` column to each RecordBatch. The index provides
/// global row ordering across all workers in the distributed system.
///
/// # Algorithm
///
/// 1. Execute the wrapped input execution plan
/// 2. For each incoming RecordBatch:
///    - Add a `cache_index` column with sequential row numbers
///    - Start numbering from the provided `start_index`
///    - Increment the counter for subsequent batches
///
/// # Index Calculation
///
/// The cache index is calculated as:
/// - First row in first batch: `start_index`
/// - Last row in first batch: `start_index + batch_size - 1`
/// - First row in second batch: `start_index + batch_size`
/// - And so on...
///
/// # Schema Modification
///
/// The output schema includes all original columns plus:
/// - `cache_index`: UInt64 column with globally unique row identifiers
/// - Column is appended to the end of the schema
/// - Non-nullable (every row gets an index)
///
/// # Performance Considerations
///
/// - Minimal overhead: only adds one column per batch
/// - Streaming execution: processes batches as they arrive
/// - Memory efficient: doesn't buffer entire dataset
/// - Preserves original data ordering and partitioning
///
/// # See Also
///
/// - [`WorkerDataSource`]: Creates this execution plan
/// - [`RowNumberStream`]: The stream that performs the index addition
/// - [`WorkerExec`]: The typical input execution plan
pub struct IndexColumnExec {
    input: Arc<dyn ExecutionPlan>,
    schema: SchemaRef,
    plan_properties: PlanProperties,
    start_index: u64,
}

impl IndexColumnExec {
    fn new(input: Arc<dyn ExecutionPlan>, schema: SchemaRef, start_index: u64) -> Self {
        let eq_properties = EquivalenceProperties::new_with_orderings(schema.clone(), &[]);
        let plan_properties = PlanProperties::new(
            eq_properties, // Equivalence Properties
            datafusion::physical_expr::Partitioning::UnknownPartitioning(1), // Output Partitioning
            EmissionType::Both,
            Boundedness::Bounded, // Execution Mode
        );
        Self {
            input,
            schema,
            plan_properties,
            start_index,
        }
    }
}

impl Debug for IndexColumnExec {
    fn fmt(&self, _f: &mut Formatter<'_>) -> std::fmt::Result {
        todo!()
    }
}

impl DisplayAs for IndexColumnExec {
    fn fmt_as(&self, t: DisplayFormatType, f: &mut Formatter) -> std::fmt::Result {
        match t {
            DisplayFormatType::Default | DisplayFormatType::Verbose => {
                write!(f, "IndexColumnExec: start_index={}", self.start_index)
            }
            DisplayFormatType::TreeRender => {
                write!(f, "start_index={}", self.start_index)
            }
        }
    }
}

impl ExecutionPlan for IndexColumnExec {
    fn name(&self) -> &str {
        "IndexColumnExec"
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
        let stream = self.input.execute(_partition, _context)?;
        Ok(Box::pin(RowNumberStream::new(
            stream,
            self.schema.clone(),
            self.start_index,
        )))
    }
}

/// Stream processor that adds cache index column to RecordBatches.
///
/// This stream wraps another RecordBatchStream and adds a monotonically
/// increasing `cache_index` column to each batch. The index provides global
/// row ordering across the distributed system.
///
/// # Row Numbering
///
/// Each row receives a unique index based on:
/// - Starting index provided during construction
/// - Sequential numbering across all batches
/// - Continuous numbering (no gaps between batches)
///
/// # Performance Characteristics
///
/// - Streaming: processes one batch at a time
/// - Low memory overhead: only stores current row counter
/// - Preserves batch boundaries and ordering
/// - Efficient array construction using range operations
///
/// # See Also
///
/// - [`IndexColumnExec`]: The execution plan that creates this stream
/// - [`RecordBatchStream`]: The trait this stream implements
pub struct RowNumberStream {
    inner: SendableRecordBatchStream,
    row_count: u64,
    schema: SchemaRef,
}

impl RowNumberStream {
    pub fn new(inner: SendableRecordBatchStream, schema: SchemaRef, start_index: u64) -> Self {
        RowNumberStream {
            inner,
            row_count: start_index,
            schema,
        }
    }
}

impl RecordBatchStream for RowNumberStream {
    fn schema(&self) -> SchemaRef {
        Arc::clone(&self.schema)
    }
}

impl Stream for RowNumberStream {
    type Item = Result<RecordBatch>;

    fn poll_next(mut self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<Option<Self::Item>> {
        match self.inner.poll_next_unpin(cx) {
            Poll::Ready(Some(Ok(batch))) => {
                let num_rows = batch.num_rows();
                let mut new_columns = batch.columns().to_vec();

                let row_numbers: UInt64Array = (self.row_count..self.row_count + num_rows as u64)
                    .collect::<Vec<u64>>()
                    .into();

                new_columns.push(Arc::new(row_numbers));
                let new_batch = RecordBatch::try_new(self.schema.clone(), new_columns)?;
                self.row_count += num_rows as u64;

                Poll::Ready(Some(Ok(new_batch)))
            }
            other => other,
        }
    }
}

async fn read_stream(
    table: Table,
    file_urls: Vec<String>,
) -> Result<Pin<Box<dyn Stream<Item = Result<RecordBatch>> + Send>>> {
    let reader = table.reader_builder().build();
    let files = table
        .scan()
        .with_data_file_concurrency_limit(1)
        .build()
        .map_err(to_datafusion_error)?
        .plan_files()
        .await
        .map_err(to_datafusion_error)?;
    // limit the number of files to read in parallel to support streaming from replicas
    let stream = reader
        .read(filter_and_create_stream(Ok(files), Arc::new(file_urls.clone())).await?)
        .await
        .map_err(to_datafusion_error)?
        .map_err(to_datafusion_error);
    Ok(Box::pin(stream))
}

async fn filter_and_create_stream(
    result: Result<FileScanTaskStream>,
    file_urls: Arc<Vec<String>>,
) -> Result<Pin<Box<dyn Stream<Item = std::result::Result<FileScanTask, iceberg::Error>> + Send>>> {
    match result {
        Ok(stream) => Ok(Box::pin(
            stream
                .try_filter(move |task| {
                    future::ready(file_urls.clone().contains(&task.data_file_path))
                })
                .map(|result| result),
        )),
        Err(e) => Ok(Box::pin(futures::stream::once(future::ready(Err(
            from_datafusion_error(e),
        ))))),
    }
}
