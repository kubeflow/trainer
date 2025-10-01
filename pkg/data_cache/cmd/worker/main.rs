use tracing::info;
use tracing_subscriber;

use kubeflow_data_cache::worker;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    tracing_subscriber::fmt().init();

    let args: Vec<String> = std::env::args().collect();
    info!("Arguments passed to worker: {:?}", &args[1..]);
    let host = args.get(1).ok_or("Missing host argument")?;
    let port = args.get(2).ok_or("Missing port argument")?;
    worker::worker_service::run(host, port).await
}
