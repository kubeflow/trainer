use std::env;
use std::time::Duration;
use tokio::time::sleep;
use tracing::info;
use tracing_subscriber;
use trust_dns_resolver::TokioAsyncResolver;
use trust_dns_resolver::config::*;

use kubeflow_data_cache::head;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    tracing_subscriber::fmt().init();

    let args: Vec<String> = std::env::args().collect();
    info!("Arguments passed to head: {:?}", &args[1..]);

    let mut rpc_hosts = Vec::new();

    if env::var("RUNTIME_ENV").is_ok() {
        rpc_hosts.push(format!("{}:{}", "localhost", "50052"));
        rpc_hosts.push(format!("{}:{}", "localhost", "50053"));
    } else {
        let lws_leader_address = env::var("LWS_LEADER_ADDRESS")?;
        let lws_size: i32 = env::var("LWS_GROUP_SIZE")?.parse()?;
        let rpc_port = 50051;

        let service_tokens: Vec<&str> = lws_leader_address.split('.').collect();

        let _resolver =
            TokioAsyncResolver::tokio(ResolverConfig::default(), ResolverOpts::default());

        for i in 1..lws_size {
            let host = format!(
                "{}-{}.{}",
                service_tokens[0],
                i,
                service_tokens[1..].join(".")
            );

            sleep(Duration::from_secs(10)).await;

            rpc_hosts.push(format!("{}:{}", host, rpc_port));
        }
    }

    info!("RPC Hosts: {:?}", rpc_hosts);
    let host = args.get(1).ok_or("Missing host argument")?;
    let port = args.get(2).ok_or("Missing port argument")?;
    head::head_service::run(host, port, rpc_hosts).await
}
