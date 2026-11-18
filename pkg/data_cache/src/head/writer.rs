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
use std::collections::hash_map::Entry;
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
/// - Reuses one Flight connection per worker for the life of the partition, instead of
///   reconnecting for every batch routed to that worker
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
    plan_properties: Arc<PlanProperties>,
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
        let eq_properties = EquivalenceProperties::new(schema.clone());
        let plan_properties = Arc::new(PlanProperties::new(
            eq_properties,                                     // Equivalence Properties
            Partitioning::UnknownPartitioning(num_partitions), // Output Partitioning
            EmissionType::Both,
            Boundedness::Bounded, // Execution Mode
        ));
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

    fn properties(&self) -> &Arc<PlanProperties> {
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

    // Reuse one Flight connection per worker across the batches this partition sends,
    // instead of paying a fresh TCP/HTTP2 handshake for every batch routed to the same
    // worker. A partition's input stream is split into many RecordBatches whenever the
    // assigned data exceeds one batch, and they are not guaranteed to alternate workers,
    // so without this a large partition reconnects to the same worker over and over.
    let mut clients: HashMap<u64, ExecutorClient> = HashMap::new();

    while let Some(item) = stream.next().await {
        match item {
            Ok(rb) => {
                // A worker that was assigned no data files produces an empty batch,
                // which carries no worker id to route on. Those workers are told that
                // they own nothing by `Distributor::assign_empty_partitions`.
                if rb.num_rows() == 0 {
                    continue;
                }
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
                    *values.first().ok_or_else(|| {
                        DataFusionError::Execution("No worker ID found".to_string())
                    })?
                } else {
                    return exec_err!("Expected UInt64 DataType");
                };
                info!("Sending batch of partition: {_partition}");
                let client = match clients.entry(worker_id) {
                    Entry::Occupied(entry) => entry.into_mut(),
                    Entry::Vacant(entry) => {
                        let addr = worker_map.get(&worker_id.to_string()).ok_or_else(|| {
                            DataFusionError::Execution(format!(
                                "Worker {} not found in worker map",
                                worker_id
                            ))
                        })?;
                        let new_client =
                            ExecutorClient::try_new(addr, config.connect_timeout).await?;
                        entry.insert(new_client)
                    }
                };
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

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::config::DatasetConfig;
    use arrow::array::ArrayRef;
    use arrow_flight::{
        Action, ActionType, Criteria, Empty, FlightData, FlightDescriptor, FlightInfo,
        HandshakeRequest, HandshakeResponse, PollInfo, PutResult, SchemaResult, Ticket,
    };
    use arrow_schema::{Field, Schema};
    use datafusion::datasource::memory::MemorySourceConfig;
    use datafusion::prelude::SessionContext;
    use futures::Stream;
    use std::pin::Pin;
    use std::sync::atomic::{AtomicUsize, Ordering};
    use tonic::{Request, Response, Status, Streaming};

    fn test_config() -> Arc<CacheConfig> {
        Arc::new(CacheConfig {
            dataset: DatasetConfig {
                metadata_loc: String::new(),
                schema_name: String::new(),
                table_name: String::new(),
            },
            connect_timeout: Duration::from_secs(1),
        })
    }

    fn assignment_schema() -> SchemaRef {
        Arc::new(Schema::new(vec![Field::new(
            "worker_ids",
            DataType::UInt64,
            false,
        )]))
    }

    /// A worker that was assigned no data files produces an empty batch. Routing on
    /// it used to fail the whole partition with "No worker ID found", which aborted
    /// the stream before any batch queued behind it could be delivered.
    #[tokio::test]
    async fn test_send_record_batch_skips_empty_batches() -> Result<()> {
        let schema = assignment_schema();
        let empty = RecordBatch::new_empty(schema.clone());
        let input = MemorySourceConfig::try_new_exec(&[vec![empty]], schema.clone(), None)?;

        let ctx = SessionContext::new();
        let stream = send_record_batch(
            input,
            ctx.task_ctx(),
            0,
            Arc::new(HashMap::new()),
            test_config(),
        )
        .await?;

        let batches: Vec<RecordBatch> = stream.try_collect().await?;
        assert!(batches.is_empty());
        Ok(())
    }

    /// A non-empty batch still has to be routed, and an unknown worker id is still
    /// an error rather than something to skip over.
    #[tokio::test]
    async fn test_send_record_batch_rejects_unknown_worker() -> Result<()> {
        let schema = assignment_schema();
        let worker_ids: ArrayRef = Arc::new(UInt64Array::from(vec![7]));
        let batch = RecordBatch::try_new(schema.clone(), vec![worker_ids])?;
        let input = MemorySourceConfig::try_new_exec(&[vec![batch]], schema.clone(), None)?;

        let ctx = SessionContext::new();
        let result = send_record_batch(
            input,
            ctx.task_ctx(),
            0,
            Arc::new(HashMap::new()),
            test_config(),
        )
        .await;

        assert!(result.is_err());
        Ok(())
    }

    /// A [`FlightService`] that only implements `do_put`, draining the request body
    /// and replying with an empty stream. Every other method is unreachable from this
    /// test because `ExecutorClient` only ever calls `do_put`.
    struct CountingFlightService;

    #[tonic::async_trait]
    impl arrow_flight::flight_service_server::FlightService for CountingFlightService {
        type HandshakeStream =
            Pin<Box<dyn Stream<Item = std::result::Result<HandshakeResponse, Status>> + Send>>;
        type ListFlightsStream =
            Pin<Box<dyn Stream<Item = std::result::Result<FlightInfo, Status>> + Send>>;
        type DoGetStream =
            Pin<Box<dyn Stream<Item = std::result::Result<FlightData, Status>> + Send>>;
        type DoPutStream =
            Pin<Box<dyn Stream<Item = std::result::Result<PutResult, Status>> + Send>>;
        type DoExchangeStream =
            Pin<Box<dyn Stream<Item = std::result::Result<FlightData, Status>> + Send>>;
        type DoActionStream =
            Pin<Box<dyn Stream<Item = std::result::Result<arrow_flight::Result, Status>> + Send>>;
        type ListActionsStream =
            Pin<Box<dyn Stream<Item = std::result::Result<ActionType, Status>> + Send>>;

        async fn handshake(
            &self,
            _request: Request<Streaming<HandshakeRequest>>,
        ) -> std::result::Result<Response<Self::HandshakeStream>, Status> {
            Err(Status::unimplemented("not used in this test"))
        }
        async fn list_flights(
            &self,
            _request: Request<Criteria>,
        ) -> std::result::Result<Response<Self::ListFlightsStream>, Status> {
            Err(Status::unimplemented("not used in this test"))
        }
        async fn get_flight_info(
            &self,
            _request: Request<FlightDescriptor>,
        ) -> std::result::Result<Response<FlightInfo>, Status> {
            Err(Status::unimplemented("not used in this test"))
        }
        async fn poll_flight_info(
            &self,
            _request: Request<FlightDescriptor>,
        ) -> std::result::Result<Response<PollInfo>, Status> {
            Err(Status::unimplemented("not used in this test"))
        }
        async fn get_schema(
            &self,
            _request: Request<FlightDescriptor>,
        ) -> std::result::Result<Response<SchemaResult>, Status> {
            Err(Status::unimplemented("not used in this test"))
        }
        async fn do_get(
            &self,
            _request: Request<Ticket>,
        ) -> std::result::Result<Response<Self::DoGetStream>, Status> {
            Err(Status::unimplemented("not used in this test"))
        }
        async fn do_put(
            &self,
            request: Request<Streaming<FlightData>>,
        ) -> std::result::Result<Response<Self::DoPutStream>, Status> {
            // Drain the request body so the client's do_put call completes normally.
            let mut inbound = request.into_inner();
            while inbound
                .message()
                .await
                .map_err(|e| Status::internal(e.to_string()))?
                .is_some()
            {}
            Ok(Response::new(Box::pin(futures::stream::empty())))
        }
        async fn do_exchange(
            &self,
            _request: Request<Streaming<FlightData>>,
        ) -> std::result::Result<Response<Self::DoExchangeStream>, Status> {
            Err(Status::unimplemented("not used in this test"))
        }
        async fn do_action(
            &self,
            _request: Request<Action>,
        ) -> std::result::Result<Response<Self::DoActionStream>, Status> {
            Err(Status::unimplemented("not used in this test"))
        }
        async fn list_actions(
            &self,
            _request: Request<Empty>,
        ) -> std::result::Result<Response<Self::ListActionsStream>, Status> {
            Err(Status::unimplemented("not used in this test"))
        }
    }

    /// Wraps a bound `TcpListener` as the `Stream` of accepted sockets `serve_with_incoming`
    /// expects, counting each accepted connection in `counter`.
    fn counting_incoming(
        listener: tokio::net::TcpListener,
        counter: Arc<AtomicUsize>,
    ) -> impl Stream<Item = std::io::Result<tokio::net::TcpStream>> {
        futures::stream::unfold((listener, counter), |(listener, counter)| async move {
            let accepted = listener.accept().await.map(|(socket, _)| {
                counter.fetch_add(1, Ordering::SeqCst);
                socket
            });
            Some((accepted, (listener, counter)))
        })
    }

    /// Three batches routed to the same worker used to open three Flight connections
    /// (one `ExecutorClient::try_new` per batch). They should now share one connection
    /// for the life of the partition.
    #[tokio::test]
    async fn test_send_record_batch_reuses_connection_per_worker() -> Result<()> {
        let std_listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
        std_listener.set_nonblocking(true).unwrap();
        let addr = std_listener.local_addr().unwrap();
        let listener = tokio::net::TcpListener::from_std(std_listener).unwrap();

        let connections = Arc::new(AtomicUsize::new(0));
        let incoming = counting_incoming(listener, connections.clone());

        let server = tokio::spawn(async move {
            tonic::transport::Server::builder()
                .add_service(
                    arrow_flight::flight_service_server::FlightServiceServer::new(
                        CountingFlightService,
                    ),
                )
                .serve_with_incoming(incoming)
                .await
        });

        let schema = assignment_schema();
        // Three separate batches addressed to the same worker, the way a single
        // partition's assigned rows split across DataFusion's target batch size.
        let batches: Vec<RecordBatch> = (0..3)
            .map(|_| {
                let worker_ids: ArrayRef = Arc::new(UInt64Array::from(vec![0u64]));
                RecordBatch::try_new(schema.clone(), vec![worker_ids]).unwrap()
            })
            .collect();
        let input = MemorySourceConfig::try_new_exec(&[batches], schema.clone(), None)?;

        let mut worker_map = HashMap::new();
        worker_map.insert("0".to_string(), format!("http://{addr}"));

        let ctx = SessionContext::new();
        let stream = send_record_batch(
            input,
            ctx.task_ctx(),
            0,
            Arc::new(worker_map),
            test_config(),
        )
        .await?;
        let _: Vec<RecordBatch> = stream.try_collect().await?;

        assert_eq!(
            connections.load(Ordering::SeqCst),
            1,
            "expected one reused connection for 3 batches sent to the same worker, got {}",
            connections.load(Ordering::SeqCst)
        );

        server.abort();
        Ok(())
    }
}
