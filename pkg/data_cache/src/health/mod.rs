use axum::{Router, extract::State, http::StatusCode, response::IntoResponse, routing::get};
use datafusion::logical_expr::{col, lit};
use datafusion::prelude::SessionContext;
use std::net::SocketAddr;
use std::sync::Arc;
use tracing::{error, info};

/// Health check state containing the session context for querying memtable
#[derive(Clone)]
pub struct HealthState {
    pub ctx: Arc<SessionContext>,
    pub table_name: String,
}

impl HealthState {
    pub fn new(ctx: Arc<SessionContext>, table_name: String) -> Self {
        Self { ctx, table_name }
    }
}

/// Readiness probe handler for head service
/// Checks if the memtable can be queried by attempting to count rows
/// Returns 200 OK if ready, 503 Service Unavailable if not ready
async fn ready_head_handler(State(state): State<HealthState>) -> impl IntoResponse {
    match check_table_ready(&state.ctx, &state.table_name, "worker_ids").await {
        Ok(_) => (StatusCode::OK, "ready"),
        Err(e) => {
            error!("Head service not ready - memtable query failed: {}", e);
            error!(
                "Available tables: {:?}",
                state.ctx.catalog("datafusion").unwrap().schema_names()
            );
            (StatusCode::SERVICE_UNAVAILABLE, "not ready")
        }
    }
}

/// Readiness probe handler for worker service
/// Checks if the memtable exists and can be queried
/// Returns 200 OK if ready, 503 Service Unavailable if not ready
async fn ready_worker_handler(State(state): State<HealthState>) -> impl IntoResponse {
    match check_table_ready(&state.ctx, &state.table_name, "cache_index").await {
        Ok(_) => (StatusCode::OK, "ready"),
        Err(e) => {
            error!("Worker service not ready - memtable query failed: {}", e);
            error!(
                "Available tables: {:?}",
                state.ctx.catalog("datafusion").unwrap().schema_names()
            );
            (StatusCode::SERVICE_UNAVAILABLE, "not ready")
        }
    }
}

/// Checks if the table is queryable using DataFrame API with filter on specified column
/// This verifies that the table is registered and has data loaded
///
/// For worker nodes: Uses filter on cache_index >= 0 to satisfy IndexableMemTable requirements
/// For head nodes: Uses filter on worker_ids >= 0 to verify partition metadata is loaded
async fn check_table_ready(
    ctx: &Arc<SessionContext>,
    table_name: &str,
    filter_column: &str,
) -> Result<(), String> {
    // Check if table exists
    let table_exists = ctx
        .table_exist(table_name)
        .map_err(|e| format!("Failed to check table existence: {}", e))?;

    if !table_exists {
        let error_msg = format!("Table '{}' does not exist in catalog", table_name);
        error!("{}", error_msg);
        return Err(error_msg);
    }

    // Use DataFrame API with filter on specified column >= 0
    let df = ctx
        .table(table_name)
        .await
        .map_err(|e| {
            let error_msg = format!("Failed to access table: {}", e);
            error!("{}", error_msg);
            error_msg
        })?
        .filter(col(filter_column).gt_eq(lit(0u64)))
        .map_err(|e| {
            let error_msg = format!("Failed to apply filter: {}", e);
            error!("{}", error_msg);
            error_msg
        })?
        .limit(0, Some(1))
        .map_err(|e| {
            let error_msg = format!("Failed to apply limit: {}", e);
            error!("{}", error_msg);
            error_msg
        })?;

    // Execute the query to verify data is actually accessible
    df.collect().await.map_err(|e| {
        let error_msg = format!("Failed to execute query: {}", e);
        error!("{}", error_msg);
        error_msg
    })?;

    Ok(())
}

/// Creates and returns the readiness check router for head service
pub fn head_router(state: HealthState) -> Router {
    Router::new()
        .route("/ready", get(ready_head_handler))
        .with_state(state)
}

/// Creates and returns the readiness check router for worker service
pub fn worker_router(state: HealthState) -> Router {
    Router::new()
        .route("/ready", get(ready_worker_handler))
        .with_state(state)
}

/// Starts the readiness check HTTP server
/// This runs concurrently with the gRPC service
pub async fn start_health_server(
    router: Router,
    port: u16,
) -> Result<(), Box<dyn std::error::Error>> {
    let addr = SocketAddr::from(([0, 0, 0, 0], port));
    info!("Starting readiness probe server on {}", addr);

    let listener = tokio::net::TcpListener::bind(addr).await?;
    info!("Readiness probe server successfully bound to {}", addr);
    info!("Health check endpoint available at http://{}/ready", addr);

    axum::serve(listener, router).await.map_err(|e| {
        error!("Readiness server error: {}", e);
        format!("Readiness server error: {}", e)
    })?;

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use arrow::array::Int32Array;
    use arrow::datatypes::{DataType, Field, Schema};
    use arrow::record_batch::RecordBatch;
    use axum::body::Body;
    use axum::http::Request;
    use datafusion::datasource::MemTable;
    use datafusion::prelude::SessionContext;
    use std::sync::Arc;
    use tower::ServiceExt;

    fn create_test_context() -> Arc<SessionContext> {
        Arc::new(SessionContext::new())
    }

    async fn register_head_test_table(ctx: &SessionContext, table_name: &str) {
        let schema = Arc::new(Schema::new(vec![Field::new(
            "worker_ids",
            DataType::Int32,
            false,
        )]));
        let batch = RecordBatch::try_new(
            schema.clone(),
            vec![Arc::new(Int32Array::from(vec![1, 2, 3]))],
        )
        .unwrap();

        let mem_table = MemTable::try_new(schema, vec![vec![batch]]).unwrap();
        ctx.register_table(table_name, Arc::new(mem_table)).unwrap();
    }

    async fn register_worker_test_table(ctx: &SessionContext, table_name: &str) {
        let schema = Arc::new(Schema::new(vec![Field::new(
            "cache_index",
            DataType::Int32,
            false,
        )]));
        let batch = RecordBatch::try_new(
            schema.clone(),
            vec![Arc::new(Int32Array::from(vec![1, 2, 3]))],
        )
        .unwrap();

        let mem_table = MemTable::try_new(schema, vec![vec![batch]]).unwrap();
        ctx.register_table(table_name, Arc::new(mem_table)).unwrap();
    }

    #[tokio::test]
    async fn test_ready_endpoint_fails_without_table() {
        let ctx = create_test_context();
        let state = HealthState::new(ctx, "nonexistent_table".to_string());
        let app = head_router(state);

        let response = app
            .oneshot(
                Request::builder()
                    .uri("/ready")
                    .body(Body::empty())
                    .unwrap(),
            )
            .await
            .unwrap();

        assert_eq!(response.status(), StatusCode::SERVICE_UNAVAILABLE);
    }

    #[tokio::test]
    async fn test_ready_endpoint_succeeds_with_table() {
        let ctx = create_test_context();
        register_head_test_table(&ctx, "test_table").await;
        let state = HealthState::new(ctx, "test_table".to_string());
        let app = head_router(state);

        let response = app
            .oneshot(
                Request::builder()
                    .uri("/ready")
                    .body(Body::empty())
                    .unwrap(),
            )
            .await
            .unwrap();

        assert_eq!(response.status(), StatusCode::OK);
    }

    #[tokio::test]
    async fn test_worker_ready_endpoint_succeeds_with_table() {
        let ctx = create_test_context();
        register_worker_test_table(&ctx, "memtable").await;
        let state = HealthState::new(ctx, "memtable".to_string());
        let app = worker_router(state);

        let response = app
            .oneshot(
                Request::builder()
                    .uri("/ready")
                    .body(Body::empty())
                    .unwrap(),
            )
            .await
            .unwrap();

        assert_eq!(response.status(), StatusCode::OK);
    }
}
