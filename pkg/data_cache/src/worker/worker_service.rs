use crate::config::config::{CACHE_INDEX_COLUMN, DatasetConfig, IndexPair};
use crate::worker::worker::DataLoader;
use arrow::array::{ListArray, StringViewArray, UInt64Array};
use arrow_flight::decode::FlightRecordBatchStream;
use arrow_flight::encode::FlightDataEncoderBuilder;
use arrow_flight::error::FlightError;
use arrow_flight::flight_service_server::FlightServiceServer;
use arrow_flight::{
    Action, Criteria, Empty, FlightData, FlightDescriptor, FlightInfo, HandshakeRequest,
    HandshakeResponse, PollInfo, PutResult, SchemaResult, Ticket,
    flight_service_server::FlightService,
};
use arrow_schema::DataType;
use bytes::Bytes;
use datafusion::prelude::{SessionConfig, SessionContext};
use futures::{Stream, StreamExt, TryStreamExt};
use std::pin::Pin;
use std::sync::Arc;
use tonic::{Request, Response, Status, Streaming};
use tracing::info;

/// Worker node service implementing Apache Arrow Flight protocol for distributed caching.
///
/// This service provides the worker node functionality in the distributed Arrow
/// caching system. It receives data file assignments from the head node and
/// serves query results for specific row ranges.
///
/// # Architecture
///
/// The worker service operates as part of a head-worker distributed system:
/// - Receives file assignments via Flight `do_put` operations
/// - Loads assigned data files into memory tables
/// - Serves query results via Flight `do_get` operations
/// - Maintains cached data for efficient retrieval
///
/// # Flight Protocol Usage
///
/// - **`do_put`**: Receives file assignments and row indexing information
/// - **`do_get`**: Serves query results for specific row ranges
/// - **`get_schema`**: Returns schema information for cached data
/// - Other Flight methods are not currently implemented
///
/// # Data Flow
///
/// ```text
/// ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
/// │   Head Node     │───▶│ WorkerService   │───▶│   DataLoader    │
/// │   (do_put)      │    │  (FlightService)│    │  (load files)   │
/// └─────────────────┘    └─────────────────┘    └─────────────────┘
///                                 │                       │
///                                 ▼                       ▼
///                        ┌─────────────────┐    ┌─────────────────┐
/// ┌─────────────────┐    │   Memory Table  │    │   Query Engine  │
/// │   Client        │◀───┤   (cached data) │◀───┤   (DataFusion)  │
/// │   (do_get)      │    └─────────────────┘    └─────────────────┘
/// └─────────────────┘
/// ```
///
/// # Performance Considerations
///
/// - Data is cached in memory for fast query response
/// - Supports streaming queries for large result sets
/// - Uses DataFusion's query engine for efficient processing
/// - Maintains global row indexing for distributed coordination
///
/// # See Also
///
/// - [`DataLoader`]: Loads assigned data files into memory tables
/// - [`IndexPair`]: Represents row range queries in tickets
pub(crate) struct WorkerService {
    metadata_loc: String,
    table_name: String,
    schema_name: String,
    ctx: Arc<SessionContext>,
}

#[tonic::async_trait]
impl FlightService for WorkerService {
    type HandshakeStream =
        Pin<Box<dyn Stream<Item = Result<HandshakeResponse, Status>> + Send + 'static>>;
    type ListFlightsStream =
        Pin<Box<dyn Stream<Item = Result<FlightInfo, Status>> + Send + 'static>>;
    type DoGetStream = Pin<Box<dyn Stream<Item = Result<FlightData, Status>> + Send + 'static>>;
    type DoPutStream = Pin<Box<dyn Stream<Item = Result<PutResult, Status>> + Send + 'static>>;
    type DoExchangeStream =
        Pin<Box<dyn Stream<Item = Result<FlightData, Status>> + Send + 'static>>;
    type DoActionStream =
        Pin<Box<dyn Stream<Item = Result<arrow_flight::Result, Status>> + Send + 'static>>;
    type ListActionsStream =
        Pin<Box<dyn Stream<Item = Result<arrow_flight::ActionType, Status>> + Send + 'static>>;

    async fn get_schema(
        &self,
        _request: Request<FlightDescriptor>,
    ) -> Result<Response<SchemaResult>, Status> {
        unimplemented!()
    }

    /// Retrieves data for a specific row range from the cached table.
    ///
    /// This method serves query results for a specific row range specified in the
    /// ticket. The ticket contains a serialized [`IndexPair`] with start and end
    /// row indices for the requested data range.
    ///
    /// # Parameters
    ///
    /// - `request`: Flight ticket containing serialized [`IndexPair`] with row range
    ///
    /// # Returns
    ///
    /// Returns a Flight data stream containing the requested rows from the cached
    /// table, excluding the `cache_index` column from the result set.
    ///
    /// # Algorithm
    ///
    /// 1. Deserialize the [`IndexPair`] from the ticket
    /// 2. Execute SQL query to select rows in the specified range
    /// 3. Exclude the `cache_index` column from results
    /// 4. Encode results as Flight data stream
    ///
    /// # Errors
    ///
    /// - Returns `Status::internal` if ticket deserialization fails
    /// - Returns `Status::internal` if SQL query execution fails
    /// - Returns `Status::internal` if stream creation fails
    ///
    /// # Example Query
    ///
    /// ```sql
    /// SELECT * EXCEPT(cache_index) FROM memtable
    /// WHERE cache_index >= {start} AND cache_index <= {end}
    /// ```
    async fn do_get(
        &self,
        request: Request<Ticket>,
    ) -> Result<Response<<Self as FlightService>::DoGetStream>, Status> {
        info!("querying worker");
        let ticket = request.into_inner();
        let pair = bincode::deserialize::<IndexPair>(&ticket.ticket)
            .map_err(|e| Status::internal(format!("Deserialization error: {}", e)))?;
        info!("{:?}", pair);
        let df = self
            .ctx
            .sql(
                format!(
                    "select * except({}) from memtable where {} >= {} and {} <= {}",
                    CACHE_INDEX_COLUMN,
                    CACHE_INDEX_COLUMN,
                    pair.start,
                    CACHE_INDEX_COLUMN,
                    pair.end
                )
                .as_str(),
            )
            .await
            .map_err(|e| Status::internal(format!("Error executing query: {}", e)))?;
        let stream = df
            .execute_stream()
            .await
            .map_err(|e| Status::internal(format!("Error creating stream: {}", e)))?;

        let encoder = FlightDataEncoderBuilder::new()
            .build(stream.map_err(|e| FlightError::ExternalError(Box::new(e))))
            .map_err(Status::from);
        Ok(Response::new(Box::pin(encoder)))
    }

    async fn handshake(
        &self,
        _request: Request<Streaming<HandshakeRequest>>,
    ) -> Result<Response<Self::HandshakeStream>, Status> {
        todo!()
    }

    async fn list_flights(
        &self,
        _request: Request<Criteria>,
    ) -> Result<Response<Self::ListFlightsStream>, Status> {
        todo!()
    }

    async fn get_flight_info(
        &self,
        _request: Request<FlightDescriptor>,
    ) -> Result<Response<FlightInfo>, Status> {
        todo!()
    }

    async fn poll_flight_info(
        &self,
        _request: Request<FlightDescriptor>,
    ) -> Result<Response<PollInfo>, Status> {
        todo!()
    }

    /// Receives file assignments and loads data into the worker's memory table.
    ///
    /// This method receives file assignments from the head node and loads the
    /// specified data files into a memory table for caching. The request contains
    /// file paths and row indexing information needed for distributed coordination.
    ///
    /// # Parameters
    ///
    /// - `request`: Flight data stream containing file assignment information
    ///
    /// # Expected Input Format
    ///
    /// The input RecordBatch must contain the following columns:
    /// - `file_paths`: List<Utf8View> - Array of file paths to load
    /// - `row_start_indexes`: UInt64 - Starting row index for global ordering
    ///
    /// # Returns
    ///
    /// Returns a single [`PutResult`] indicating successful data loading.
    ///
    /// # Algorithm
    ///
    /// 1. Decode the incoming Flight data stream
    /// 2. Extract file paths from the `file_paths` column
    /// 3. Extract starting row index from `row_start_indexes` column
    /// 4. Create a [`DataLoader`] with the assigned files
    /// 5. Load data into the memory table named "memtable"
    /// 6. Verify data loading with a test query
    ///
    /// # Error Handling
    ///
    /// - Returns `Status::internal` if no RecordBatch is received
    /// - Returns `Status::internal` if required columns are missing
    /// - Returns `Status::internal` if column types don't match expected format
    /// - Returns `Status::internal` if data loading fails
    ///
    /// # Performance Considerations
    ///
    /// - Data is loaded into memory for fast subsequent queries
    /// - Uses streaming to handle large file lists efficiently
    /// - Maintains global row indexing for distributed coordination
    ///
    /// # See Also
    ///
    /// - [`DataLoader`]: Handles the actual file loading process
    /// - [`do_get`]: Serves queries against the loaded data
    async fn do_put(
        &self,
        request: Request<Streaming<FlightData>>,
    ) -> Result<Response<Self::DoPutStream>, Status> {
        let record_batch = FlightRecordBatchStream::new_from_flight_data(
            request.into_inner().map_err(|e| e.into()),
        )
        .try_next()
        .await
        .map_err(|e| Status::internal(format!("Flight data error: {}", e)))?
        .ok_or_else(|| Status::internal("No record batch received"))?;
        let file_paths_column = record_batch
            .column_by_name("file_paths")
            .ok_or_else(|| Status::internal("file_paths column not found"))?;
        let start_indexes_column = record_batch
            .column_by_name("row_start_indexes")
            .ok_or_else(|| Status::internal("row_start_indexes column not found"))?;

        let file_urls;
        if let DataType::List(field) = file_paths_column.data_type() {
            if field.data_type() == &DataType::Utf8View {
                let list_array = file_paths_column
                    .as_any()
                    .downcast_ref::<ListArray>()
                    .ok_or_else(|| Status::internal("Failed to downcast to ListArray"))?;
                let values = list_array.values();
                let string_array = values
                    .as_any()
                    .downcast_ref::<StringViewArray>()
                    .ok_or_else(|| Status::internal("Failed to downcast to StringViewArray"))?;

                file_urls = string_array
                    .iter()
                    .map(|opt_str| opt_str.map(|s| s.to_string()).unwrap_or_default())
                    .collect();
            } else {
                return Err(Status::internal(
                    "Expected List<Utf8>, found List with different item type",
                ));
            }
        } else {
            return Err(Status::internal("Expected List DataType"));
        }
        info!("file_urls received in worker: {:?}", file_urls);

        let start_index = if let DataType::UInt64 = start_indexes_column.data_type() {
            let list_array = start_indexes_column
                .as_any()
                .downcast_ref::<UInt64Array>()
                .ok_or_else(|| Status::internal("Failed to downcast to UInt64Array"))?;
            let values = list_array.values();
            *values
                .first()
                .ok_or_else(|| Status::internal("No start index found"))?
        } else {
            return Err(Status::internal("Expected UInt64 DataType"));
        };

        info!("start_index received in worker: {:?}", start_index);

        let data_loader = DataLoader::new(
            self.metadata_loc.clone(),
            self.table_name.clone(),
            self.schema_name.clone(),
            file_urls,
            start_index,
        )
        .await
        .map_err(|e| Status::internal(format!("Failed to create data loader: {}", e)))?;
        let _ = data_loader
            .load_data(&self.ctx.clone(), "memtable", start_index)
            .await
            .map_err(|e| Status::internal(format!("failed to load dataset: {}", e)));
        let df = self
            .ctx
            .sql(
                format!(
                    "select {} from memtable where {} >= {} and {} <= {}",
                    CACHE_INDEX_COLUMN,
                    CACHE_INDEX_COLUMN,
                    start_index,
                    CACHE_INDEX_COLUMN,
                    start_index
                )
                .as_str(),
            )
            .await
            .map_err(|e| Status::internal(format!("SQL error: {}", e)))?
            .collect()
            .await
            .map_err(|e| Status::internal(format!("Collection error: {}", e)))?;
        info!("printing recordbatch");
        let _ = arrow::util::pretty::print_batches(&df);

        let app_metadata = Bytes::new();
        let result = PutResult { app_metadata };
        let stream = futures::stream::iter([Ok(result)]);
        Ok(Response::new(stream.boxed()))
    }

    async fn do_exchange(
        &self,
        _request: Request<Streaming<FlightData>>,
    ) -> Result<Response<Self::DoExchangeStream>, Status> {
        todo!()
    }

    async fn do_action(
        &self,
        _request: Request<Action>,
    ) -> Result<Response<Self::DoActionStream>, Status> {
        todo!()
    }

    async fn list_actions(
        &self,
        _request: Request<Empty>,
    ) -> Result<Response<Self::ListActionsStream>, Status> {
        todo!()
    }
}

pub async fn run(
    host: &String,
    port: &String,
) -> datafusion::common::Result<(), Box<dyn std::error::Error>> {
    let config = SessionConfig::new().with_batch_size(1024);
    let ctx = Arc::new(SessionContext::new_with_config(config));
    let addr = format!("{host}:{port}").parse()?;
    let dataset_config =
        DatasetConfig::from_env().map_err(|e| format!("Failed to load dataset config: {}", e))?;

    let service = WorkerService {
        metadata_loc: dataset_config.metadata_loc,
        table_name: dataset_config.table_name,
        schema_name: dataset_config.schema_name,
        ctx: ctx.clone(),
    };
    tonic::transport::Server::builder()
        .add_service(FlightServiceServer::new(service))
        .serve(addr)
        .await
        .map_err(|e| format!("Error starting worker: {}", e))?;
    Ok(())
}
