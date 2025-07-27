use crate::head::head::Distributor;
use crate::head::provider::DataFileTableProvider;
use arrow_flight::flight_service_server::FlightServiceServer;
use arrow_flight::{
    Action, Criteria, Empty, FlightData, FlightDescriptor, FlightEndpoint, FlightInfo,
    HandshakeRequest, HandshakeResponse, Location, PollInfo, PutResult, SchemaResult, Ticket,
    flight_service_server::FlightService,
};
use arrow_schema::{DataType, Field, Schema, SchemaRef};
use bincode;
use bytes::Bytes;
use datafusion::prelude::SessionContext;
use futures::Stream;
use std::collections::HashMap;
use std::pin::Pin;
use std::sync::Arc;
use tonic::{Request, Response, Status, Streaming};

/// Head node service implementing Apache Arrow Flight protocol for distributed query coordination.
///
/// This service provides the head node functionality in the distributed Arrow
/// caching system. It coordinates query execution across multiple worker nodes
/// and provides flight information for distributed data access.
///
/// # Architecture
///
/// The head service operates as the coordinator in a head-worker distributed system:
/// - Receives flight information requests from clients
/// - Partitions data ranges across available worker nodes
/// - Distributes file assignments to worker nodes
/// - Provides flight endpoints for distributed query execution
///
/// # Flight Protocol Usage
///
/// - **`get_flight_info`**: Provides flight information with worker endpoints
/// - Other Flight methods are not currently implemented
///
/// # Data Flow
///
/// ```text
/// ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
/// │   Client        │───▶│   HeadService   │───▶│   Distributor   │
/// │(get_flight_info)│    │  (FlightService)│    │ (partitioning)  │
/// └─────────────────┘    └─────────────────┘    └─────────────────┘
///                                 │                       │
///                                 ▼                       ▼
///                        ┌─────────────────┐    ┌─────────────────┐
///                        │ Flight Endpoints│    │ Worker Nodes    │
///                        │   (workers)     │    │   (data files)  │
///                        └─────────────────┘    └─────────────────┘
/// ```
///
/// # Performance Considerations
///
/// - Partitions data to balance load across workers
/// - Uses efficient serialization for flight metadata
/// - Maintains worker topology for optimal data distribution
/// - Supports dynamic worker scaling
///
/// # See Also
///
/// - [`Distributor`]: Handles data distribution and worker coordination
/// - [`get_partition_range`]: Calculates data partitioning ranges
/// - [`IndexPair`]: Represents row ranges in flight tickets
pub(crate) struct HeadService {
    distributor: Distributor,
}

#[tonic::async_trait]
impl FlightService for HeadService {
    type HandshakeStream =
        Pin<Box<dyn Stream<Item = Result<HandshakeResponse, Status>> + Send + 'static>>;
    async fn handshake(
        &self,
        _request: Request<Streaming<HandshakeRequest>>,
    ) -> Result<Response<Self::HandshakeStream>, Status> {
        todo!()
    }
    type ListFlightsStream =
        Pin<Box<dyn Stream<Item = Result<FlightInfo, Status>> + Send + 'static>>;
    async fn list_flights(
        &self,
        _request: Request<Criteria>,
    ) -> Result<Response<Self::ListFlightsStream>, Status> {
        todo!()
    }
    /// Provides flight information for distributed query execution.
    ///
    /// This method handles client requests for flight information by partitioning
    /// data ranges across available worker nodes and returning flight endpoints
    /// that clients can use to query specific data ranges.
    ///
    /// # Parameters
    ///
    /// - `request`: Flight descriptor containing partition information in the path
    ///   - `path[0]`: Local rank (partition ID) for the requesting client
    ///   - `path[1]`: Total number of partitions across all clients
    ///
    /// # Returns
    ///
    /// Returns [`FlightInfo`] containing:
    /// - Flight endpoints with worker URIs for the requested partition
    /// - Serialized [`IndexPair`] tickets containing row ranges
    /// - Schema information for the distributed data
    ///
    /// # Algorithm
    ///
    /// 1. Parse local rank and total partitions from the flight descriptor
    /// 2. Calculate data partition range using [`get_partition_range`]
    /// 3. Get worker nodes responsible for the partition range
    /// 4. Create flight endpoints with worker locations and row range tickets
    /// 5. Return flight information with schema and endpoints
    ///
    /// # Data Partitioning
    ///
    /// Data is partitioned evenly across the requested number of partitions:
    /// - Each partition gets approximately `total_rows / num_partitions` rows
    /// - Partition ranges are calculated to avoid gaps or overlaps
    /// - Empty partitions are handled gracefully
    ///
    /// # Error Handling
    ///
    /// - Returns `Status::invalid_argument` if path parameters are missing
    /// - Returns `Status::invalid_argument` if parameters cannot be parsed
    /// - Returns `Status::internal` if worker lookup fails
    /// - Returns `Status::internal` if serialization fails
    ///
    /// # Example Usage
    ///
    /// For a client requesting partition 0 of 4 total partitions:
    /// ```
    /// path = ["0", "4"]
    /// // Returns flight endpoints for rows 0 to (total_rows/4 - 1)
    /// ```
    ///
    /// # Performance Considerations
    ///
    /// - Uses efficient binary serialization for flight metadata
    /// - Minimizes network communication by providing direct worker endpoints
    /// - Balances load across available workers
    /// - Supports parallel query execution across partitions
    ///
    /// # See Also
    ///
    /// - [`get_partition_range`]: Calculates partition boundaries
    /// - [`Distributor::get_workers_to_connect`]: Finds responsible workers
    /// - [`IndexPair`]: Row range representation in tickets
    async fn get_flight_info(
        &self,
        request: Request<FlightDescriptor>,
    ) -> Result<Response<FlightInfo>, Status> {
        let request = request.into_inner();
        let local_rank = request
            .path
            .first()
            .ok_or_else(|| Status::invalid_argument("Missing local_rank in path"))?;
        let total = request
            .path
            .get(1)
            .ok_or_else(|| Status::invalid_argument("Missing total in path"))?;
        let total_parsed = total
            .parse()
            .map_err(|_| Status::invalid_argument("Invalid total value"))?;
        let local_rank_parsed = local_rank
            .parse()
            .map_err(|_| Status::invalid_argument("Invalid local_rank value"))?;

        let (pair, workers) = if let Some((start, end)) = get_partition_range(
            self.distributor.total_row_count as usize,
            total_parsed,
            local_rank_parsed,
        ) {
            let pair = IndexPair { start, end };
            let workers = self.distributor.get_workers_to_connect(start, end).await;
            (pair, workers)
        } else {
            return Err(Status::out_of_range("Invalid partition range"));
        };
        let mut endpoints = vec![];
        for uri in workers.map_err(|e| Status::internal(format!("Error getting workers: {}", e)))? {
            endpoints.push(FlightEndpoint {
                ticket: Some(Ticket::new(Bytes::from(
                    bincode::serialize(&pair)
                        .map_err(|e| Status::internal(format!("Serialization error: {}", e)))?,
                ))),
                location: vec![Location { uri }],
                expiration_time: None,
                app_metadata: Bytes::from(
                    bincode::serialize(&pair)
                        .map_err(|e| Status::internal(format!("Serialization error: {}", e)))?,
                ),
            });
        }

        let flight_info = FlightInfo {
            schema: Bytes::new(),
            flight_descriptor: Some(request),
            endpoint: endpoints,
            total_records: -1,
            total_bytes: -1,
            ordered: false,
            app_metadata: Default::default(),
        };

        let flight_info = flight_info
            .try_with_schema(arrow_schema().as_ref())
            .map_err(|e| Status::internal(format!("Schema error: {}", e)))?; // TODO:// pass correct schema
        Ok(Response::new(flight_info))
    }
    async fn poll_flight_info(
        &self,
        _request: Request<FlightDescriptor>,
    ) -> Result<Response<PollInfo>, Status> {
        todo!()
    }
    async fn get_schema(
        &self,
        _request: Request<FlightDescriptor>,
    ) -> Result<Response<SchemaResult>, Status> {
        unimplemented!()
    }

    type DoGetStream = Pin<Box<dyn Stream<Item = Result<FlightData, Status>> + Send + 'static>>;

    async fn do_get(
        &self,
        _request: Request<Ticket>,
    ) -> Result<Response<<Self as FlightService>::DoGetStream>, Status> {
        todo!()
    }

    type DoPutStream = Pin<Box<dyn Stream<Item = Result<PutResult, Status>> + Send + 'static>>;

    async fn do_put(
        &self,
        _request: Request<Streaming<FlightData>>,
    ) -> Result<Response<Self::DoPutStream>, Status> {
        todo!()
    }

    type DoExchangeStream =
        Pin<Box<dyn Stream<Item = Result<FlightData, Status>> + Send + 'static>>;

    async fn do_exchange(
        &self,
        _request: Request<Streaming<FlightData>>,
    ) -> Result<Response<Self::DoExchangeStream>, Status> {
        todo!()
    }

    type DoActionStream =
        Pin<Box<dyn Stream<Item = Result<arrow_flight::Result, Status>> + Send + 'static>>;

    async fn do_action(
        &self,
        _request: Request<Action>,
    ) -> Result<Response<Self::DoActionStream>, Status> {
        todo!()
    }

    type ListActionsStream =
        Pin<Box<dyn Stream<Item = Result<arrow_flight::ActionType, Status>> + Send + 'static>>;

    async fn list_actions(
        &self,
        _request: Request<Empty>,
    ) -> Result<Response<Self::ListActionsStream>, Status> {
        todo!()
    }
}

use crate::config::config::{CacheConfig, IndexPair};

/// Represents a row range for distributed query execution.
///
/// This structure is used to communicate row ranges between the head node and
/// worker nodes in the distributed caching system. It is serialized into
/// flight tickets and application metadata for efficient communication.
///
/// # Fields
///
/// - `start`: Starting row index (inclusive)
/// - `end`: Ending row index (inclusive)
///
/// # Serialization
///
/// The struct is serialized using `bincode` for efficient binary representation
/// in flight tickets and metadata. This enables fast serialization/deserialization
/// across network boundaries.
///
/// # Usage
///
/// ```rust
/// let range = IndexPair { start: 0, end: 999 };
/// // Represents rows 0 through 999 (1000 rows total)
/// ```
///
/// # See Also
///
/// - [`get_partition_range`]: Creates partition ranges that are converted to IndexPair
/// - [`get_flight_info`]: Uses IndexPair in flight tickets
/// - [`do_get`]: Deserializes IndexPair from tickets for query execution
pub async fn run(
    host: &String,
    port: &String,
    workers: Vec<String>,
) -> datafusion::common::Result<(), Box<dyn std::error::Error>> {
    let ctx = Arc::new(SessionContext::new());
    let addr = format!("{host}:{port}").parse()?;
    let num_workers = workers.len();
    let schema = arrow_schema();
    let cache_config = CacheConfig::shared_from_env()
        .map_err(|e| format!("Failed to load dataset config: {}", e))?;
    let provider = DataFileTableProvider::new(
        &cache_config.dataset.metadata_loc,
        &cache_config.dataset.table_name,
        &cache_config.dataset.schema_name,
        schema.clone(),
        num_workers,
    )
    .await
    .map_err(|e| format!("Failed to create provider: {}", e))?;
    let mut worker_map: HashMap<String, String> = HashMap::new();
    for (index, worker_uri) in workers.into_iter().enumerate() {
        worker_map.insert(index.to_string(), format!("grpc://{worker_uri}"));
    }
    let mut distributor = Distributor::new(
        ctx,
        num_workers,
        Arc::new(provider),
        "memtable".to_string(),
        Arc::new(worker_map),
        schema.clone(),
        cache_config,
    );
    let _ = distributor.init().await;
    let service = HeadService { distributor };
    tonic::transport::Server::builder()
        .add_service(FlightServiceServer::new(service))
        .serve(addr)
        .await
        .map_err(|e| format!("Error starting server: {}", e))?;
    Ok(())
}

fn arrow_schema() -> SchemaRef {
    let columns = vec![
        Field::new("worker_ids", DataType::UInt64, false),
        Field::new("row_start_indexes", DataType::UInt64, false),
        Field::new("row_end_indexes", DataType::UInt64, false),
        Field::new(
            "file_paths",
            DataType::List(Arc::new(Field::new("item", DataType::Utf8View, true))),
            false,
        ),
    ];
    Arc::new(Schema::new(columns))
}

/// Calculates the row range for a specific partition in distributed query execution.
///
/// This function divides the total data count evenly across the requested number
/// of partitions and returns the start and end row indices for the specified
/// partition. It handles edge cases like empty datasets and ensures no gaps
/// or overlaps between partitions.
///
/// # Parameters
///
/// - `total_count`: Total number of rows in the dataset
/// - `num_partitions`: Number of partitions to divide the data into
/// - `partition_id`: Zero-based ID of the partition to calculate range for
///
/// # Returns
///
/// Returns `Some((start, end))` where:
/// - `start`: First row index for this partition (inclusive)
/// - `end`: Last row index for this partition (inclusive)
///
/// Returns `None` if:
/// - `total_count` is 0 (empty dataset)
/// - `num_partitions` is 0 (invalid partition count)
/// - `partition_id` >= `num_partitions` (invalid partition ID)
///
/// # Algorithm
///
/// 1. Calculate rows per partition using ceiling division
/// 2. Compute start index as `partition_id * rows_per_partition`
/// 3. Compute end index as `min(start + rows_per_partition, total_count)`
/// 4. Return inclusive range `[start, end-1]`
///
/// # Examples
///
/// ```rust
/// // 100 rows, 4 partitions
/// assert_eq!(get_partition_range(100, 4, 0), Some((0, 24)));   // Rows 0-24
/// assert_eq!(get_partition_range(100, 4, 1), Some((25, 49)));  // Rows 25-49
/// assert_eq!(get_partition_range(100, 4, 2), Some((50, 74)));  // Rows 50-74
/// assert_eq!(get_partition_range(100, 4, 3), Some((75, 99)));  // Rows 75-99
///
/// // Edge cases
/// assert_eq!(get_partition_range(0, 4, 0), None);     // Empty dataset
/// assert_eq!(get_partition_range(100, 4, 4), None);   // Invalid partition ID
/// ```
///
/// # Performance Considerations
///
/// - Uses ceiling division to handle uneven partition sizes
/// - Ensures the last partition gets any remaining rows
/// - Constant time complexity O(1)
/// - No memory allocation required
fn get_partition_range(
    total_count: usize,
    num_partitions: usize,
    partition_id: usize,
) -> Option<(u64, u64)> {
    if total_count == 0 || num_partitions == 0 || partition_id >= num_partitions {
        return None;
    }

    let ids_per_partition = total_count.div_ceil(num_partitions);

    let start_index = partition_id * ids_per_partition;
    let end_index = (start_index + ids_per_partition).min(total_count);

    if start_index >= total_count {
        None
    } else {
        Some((start_index as u64, (end_index - 1) as u64))
    }
}
