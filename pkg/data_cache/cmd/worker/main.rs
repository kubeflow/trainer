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
