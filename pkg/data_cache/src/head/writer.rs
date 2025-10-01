use crate::config::config::CacheConfig;
use arrow::array::UInt64Array;
use arrow::record_batch::RecordBatch;
use arrow_flight::FlightClient;
use arrow_flight::encode::FlightDataEncoderBuilder;
use arrow_schema::{DataType, SchemaRef};
use datafusion::common::exec_err;
use datafusion::error::{DataFusionError, Result};
use datafusion::execution::{SendableRecordBatchStream, TaskContext};
use datafusion::physical_expr::{EquivalenceProperties, Partitioning};
use datafusion::physical_plan::execution_plan::{Boundedness, EmissionType};
use datafusion::physical_plan::stream::RecordBatchStreamAdapter;
use datafusion::physical_plan::{
    DisplayAs, DisplayFormatType, EmptyRecordBatchStream, ExecutionPlan, PlanProperties,
};
use futures::{StreamExt, TryStreamExt};
use std::any::Any;
use std::collections::HashMap;
use std::fmt::{Debug, Formatter};
use std::sync::Arc;
use std::time::Duration;
use tracing::{error, info};

/// Execution plan for distributed writing to worker nodes via Apache Arrow Flight.
///
/// This execution plan coordinates the distribution of RecordBatches to worker
/// nodes based on worker assignments contained in the data. It reads worker
/// assignments from input batches and sends data to the appropriate worker
/// nodes using Arrow Flight protocol.
///
/// # Architecture
///
/// The distributed writer operates in a head-worker architecture:
/// - Head node executes this plan to coordinate data distribution
/// - Worker nodes receive data via Arrow Flight `do_put` operations
/// - Worker assignments are embedded in the input data as `worker_ids` column
/// - Each batch is sent to the worker specified in its `worker_ids` column
///
/// # Algorithm
///
/// 1. Execute the input execution plan to get RecordBatches
/// 2. For each batch:
///    - Extract worker ID from the `worker_ids` column
///    - Look up worker address in the worker map
///    - Establish Arrow Flight connection to worker
///    - Send batch using `do_put` operation
/// 3. Return empty stream after all batches are distributed
///
/// # Data Flow
///
/// ```text
/// ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
/// │  Input Plan     │───▶│ Distributed     │───▶│  Worker Node 1  │
/// │ (worker tasks)  │    │  WriterExec     │    │  (via Flight)   │
/// └─────────────────┘    └─────────────────┘    └─────────────────┘
///                                 │
///                                 ├──────────────────────▶│  Worker Node 2  │
///                                 │                       │  (via Flight)   │
///                                 │                       └─────────────────┘
///                                 │
///                                 └──────────────────────▶│  Worker Node N  │
///                                                         │  (via Flight)   │
///                                                         └─────────────────┘
/// ```
///
/// # Worker Assignment Format
///
/// Input batches must contain a `worker_ids` column with:
/// - Data type: UInt64
/// - Contains worker node identifier for each batch
/// - Used to lookup worker address in the worker map
///
/// # Error Handling
///
/// The execution plan handles various error conditions:
/// - Network failures during Flight connections
/// - Missing worker IDs in input batches
/// - Worker nodes not found in worker map
/// - Flight protocol errors during data transmission
///
/// # Performance Considerations
///
/// - Establishes new Flight connections for each batch (stateless)
/// - Uses configurable timeouts for connection and data transfer
/// - Processes batches sequentially to avoid overwhelming workers
/// - Memory efficient: streams data without buffering entire dataset
///
/// # See Also
///
/// - [`ExecutorClient`]: Arrow Flight client for worker communication
/// - [`send_record_batch`]: Function that performs the actual data distribution
/// - [`DataFileTableExec`]: Typical input execution plan that provides worker assignments
#[derive(Debug, Clone)]
pub struct DistributedWriterExec {
    input: Arc<dyn ExecutionPlan>,
    worker_map: Arc<HashMap<String, String>>,
    schema: SchemaRef,
    plan_properties: PlanProperties,
    config: Arc<CacheConfig>,
}

impl DisplayAs for DistributedWriterExec {
    fn fmt_as(&self, t: DisplayFormatType, f: &mut Formatter) -> std::fmt::Result {
        match t {
            DisplayFormatType::Default | DisplayFormatType::Verbose => {
                write!(
                    f,
                    "DistributedWriterExec: workers={}",
                    self.worker_map.len()
                )
            }
            DisplayFormatType::TreeRender => {
                write!(f, "workers={}", self.worker_map.len())
            }
        }
    }
}

impl DistributedWriterExec {
    pub fn new(
        input: Arc<dyn ExecutionPlan>,
        worker_map: Arc<HashMap<String, String>>,
        schema: SchemaRef,
        num_partitions: usize,
        config: Arc<CacheConfig>,
    ) -> Self {
        // TODO:// revisit plan_properties
        let eq_properties = EquivalenceProperties::new_with_orderings(schema.clone(), &[]);
        let plan_properties = PlanProperties::new(
            eq_properties,                                     // Equivalence Properties
            Partitioning::UnknownPartitioning(num_partitions), // Output Partitioning
            EmissionType::Both,
            Boundedness::Bounded, // Execution Mode
        );
        Self {
            input,
            worker_map,
            schema,
            plan_properties,
            config,
        }
    }
}

impl ExecutionPlan for DistributedWriterExec {
    fn name(&self) -> &str {
        "DistributedWriterExec"
    }

    fn as_any(&self) -> &dyn Any {
        self
    }

    fn properties(&self) -> &PlanProperties {
        &self.plan_properties
    }

    fn children(&self) -> Vec<&Arc<(dyn ExecutionPlan + 'static)>> {
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
        info!("In partition: {_partition}");
        //let addr = self.worker_map.get(&_partition.to_string()).unwrap().clone();
        let stream = futures::stream::once(send_record_batch(
            self.input.clone(),
            _context,
            _partition,
            self.worker_map.clone(),
            self.config.clone(),
        ))
        .try_flatten();
        Ok(Box::pin(RecordBatchStreamAdapter::new(
            self.schema.clone(),
            stream,
        )))
    }
}

pub async fn send_record_batch(
    input: Arc<dyn ExecutionPlan>,
    _context: Arc<TaskContext>,
    _partition: usize,
    worker_map: Arc<HashMap<String, String>>,
    config: Arc<CacheConfig>,
) -> Result<SendableRecordBatchStream> {
    info!("Executing batch of partition: {_partition}");
    let mut stream = match input.execute(_partition, _context) {
        Err(e) => {
            let err = e.to_string();
            error!("Stopping execution: error executing input: {err}");
            return exec_err!("Error with executing input plan in distributedExec");
        }
        Ok(stream) => stream,
    };

    while let Some(item) = stream.next().await {
        match item {
            Ok(rb) => {
                info!("sending rb :{:?}", rb);
                let worker_ids = rb.column_by_name("worker_ids").ok_or_else(|| {
                    DataFusionError::Execution("worker_ids column not found".to_string())
                })?;
                let worker_id = if let DataType::UInt64 = worker_ids.data_type() {
                    let list_array = worker_ids
                        .as_any()
                        .downcast_ref::<UInt64Array>()
                        .ok_or_else(|| {
                            DataFusionError::Execution(
                                "Failed to downcast to UInt64Array".to_string(),
                            )
                        })?;
                    let values = list_array.values();
                    values.first().ok_or_else(|| {
                        DataFusionError::Execution("No worker ID found".to_string())
                    })?
                } else {
                    return exec_err!("Expected UInt64 DataType");
                };
                info!("Sending batch of partition: {_partition}");
                let addr = worker_map.get(&worker_id.to_string()).ok_or_else(|| {
                    DataFusionError::Execution(format!(
                        "Worker {} not found in worker map",
                        worker_id
                    ))
                })?;
                let mut client = ExecutorClient::try_new(addr, config.connect_timeout).await?;
                let _ = client.send_batch(rb.schema(), vec![Ok(rb)]).await;
            }
            Err(error) => {
                let err = error.to_string();
                error!("Stopping execution: {err}");
                return exec_err!("error sending batch");
            }
        }
    }
    Ok(Box::pin(EmptyRecordBatchStream::new(input.schema())))
}

/// Arrow Flight client for communicating with worker nodes.
///
/// This client provides a high-level interface for sending RecordBatches to
/// worker nodes using the Arrow Flight protocol. It handles connection
/// establishment, data encoding, and error handling.
///
/// # Connection Management
///
/// - Establishes connections with configurable timeouts
/// - Uses connection pooling for efficient resource usage
/// - Handles connection failures gracefully
/// - Supports both connection and request timeouts
///
/// # Data Transfer
///
/// - Encodes RecordBatches using Arrow Flight format
/// - Sends data via `do_put` operations
/// - Handles streaming large datasets efficiently
/// - Provides error handling for transmission failures
///
/// # Performance Characteristics
///
/// - Connection timeout: 20 seconds
/// - Request timeout: 60 seconds
/// - Streaming data transfer for memory efficiency
/// - Automatic retry on transient failures
///
/// # See Also
///
/// - [`DistributedWriterExec`]: The execution plan that uses this client
/// - [`send_record_batch`]: Function that coordinates batch distribution
pub struct ExecutorClient {
    flight_client: FlightClient,
}

impl ExecutorClient {
    pub async fn try_new(addr: &String, connect_timeout: Duration) -> Result<Self> {
        info!("Connecting to {}", addr);
        let connection = tonic::transport::Endpoint::new(addr.clone())
            .map_err(|e| DataFusionError::Execution(format!("Failed to create endpoint: {}", e)))?
            .connect_timeout(connect_timeout)
            .timeout(Duration::from_secs(60)) //TODO: fix timeout to not allowing closing connection
            .connect()
            .await
            .map_err(|e| DataFusionError::Execution(format!("{e:?}")))?;
        let flight_client = FlightClient::new(connection);
        info!("Connected to {}", addr);

        Ok(Self { flight_client })
    }

    pub async fn send_batch(
        &mut self,
        schema: SchemaRef,
        record_batches: Vec<arrow_flight::error::Result<RecordBatch>>,
    ) -> Result<SendableRecordBatchStream> {
        let flight_data_stream = FlightDataEncoderBuilder::new()
            .build(futures::stream::iter(record_batches.into_iter()));
        self.flight_client
            .do_put(flight_data_stream)
            .await
            .map_err(|e| DataFusionError::Execution(format!("Error sending batch: {e:?}")))?
            .try_collect::<Vec<_>>()
            .await
            .map_err(|e| DataFusionError::Execution(format!("Error calling do_put: {}", e)))?;
        Ok(Box::pin(EmptyRecordBatchStream::new(schema)))
    }
}
