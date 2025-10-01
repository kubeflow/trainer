//! Sample client for validating access to cached datasets in the Arrow-based caching system.
//!
//! This client demonstrates how to connect to the head node and stream cached data
//! from worker nodes using Apache Arrow Flight protocol. It serves as a reference
//! implementation for accessing and consuming distributed cached datasets.

use arrow_flight::FlightDescriptor;
use arrow_flight::decode::FlightRecordBatchStream;
use arrow_flight::flight_service_client::FlightServiceClient;
use clap::Parser;
use futures::TryStreamExt;
use futures::stream::StreamExt;
use tonic::transport::Channel;
use tracing::{error, info};
use tracing_subscriber;

/// Command line arguments for the data cache client
#[derive(Parser, Debug)]
#[command(name = "data-cache-client")]
#[command(about = "Sample client for accessing cached datasets via Arrow Flight protocol")]
struct Args {
    /// Head node endpoint URL
    #[arg(
        long,
        env = "HEAD_NODE_ENDPOINT",
        default_value = "http://localhost:50051"
    )]
    endpoint: String,

    /// Local rank for this client instance
    #[arg(long, env = "LOCAL_RANK", default_value = "1")]
    local_rank: String,

    /// Total number of ranks in the distributed setup
    #[arg(long, env = "WORLD_SIZE", default_value = "3")]
    world_size: String,
}
/// Sample client main function that demonstrates accessing cached datasets.
///
/// This function:
/// 1. Connects to the configurable head node endpoint
/// 2. Requests flight information for a specific rank/partition
/// 3. Streams data from worker nodes using Arrow Flight protocol
/// 4. Validates that cached data can be successfully accessed and consumed
#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    tracing_subscriber::fmt().init();

    let args = Args::parse();

    let channel = Channel::from_shared(args.endpoint.clone())?
        .connect()
        .await?;
    let mut client = FlightServiceClient::new(channel);

    let descriptor = FlightDescriptor {
        r#type: 1,
        cmd: Default::default(),
        path: vec![args.local_rank.clone(), args.world_size.clone()],
    };

    let response = client.get_flight_info(descriptor).await?;
    let flight_info = response.into_inner();

    for endpoint in flight_info.endpoint {
        for location in endpoint.location {
            let mut client = connect_to_host(&location.uri).await?;
            let request = tonic::Request::new(
                endpoint
                    .ticket
                    .clone()
                    .ok_or("No ticket found in endpoint")?,
            );
            let response = client.do_get(request).await?.into_inner();

            let mut record_batch_stream =
                FlightRecordBatchStream::new_from_flight_data(response.map_err(|e| e.into()));

            while let Some(batch) = record_batch_stream.next().await {
                match batch {
                    Ok(record_batch) => {
                        info!("Read batch with {} rows", record_batch.num_rows());
                        // println!("{:?}", record_batch);
                    }
                    Err(e) => {
                        error!("error: {}", e)
                    }
                }
            }
        }
    }

    Ok(())
}

/// Connects to a worker node endpoint for streaming cached dataset data.
///
/// This helper function establishes a connection to a worker node and returns
/// a Flight service client for data streaming operations.
///
/// # Arguments
/// * `endpoint` - The worker node endpoint URI
///
/// # Returns
/// * `Result<FlightServiceClient<Channel>, Box<dyn std::error::Error>>` - Connected client or error
async fn connect_to_host(
    endpoint: &str,
) -> Result<FlightServiceClient<Channel>, Box<dyn std::error::Error>> {
    let address = format!("{}", endpoint);
    info!("{}", address);
    Ok(FlightServiceClient::connect(address).await?)
}
