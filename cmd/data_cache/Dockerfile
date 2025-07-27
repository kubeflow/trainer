FROM rust:1.85-bullseye AS builder

WORKDIR /workspace

ENV RUST_LOG=info

# Install system dependencies
RUN apt-get update && \
    apt-get -y install libssl-dev openssl zlib1g zlib1g-dev libpq-dev cmake protobuf-compiler netcat curl && \
    rm -rf /var/lib/apt/lists/*

# Install cargo-chef for better caching
RUN rustup update && cargo install cargo-chef --version 0.1.62

# Copy manifests for dependency caching
COPY pkg/data_cache/Cargo.toml pkg/data_cache/Cargo.lock ./

# Cache dependencies
RUN cargo fetch

# Copy source code
COPY pkg/data_cache/ .

# Run tests
RUN cargo test --tests

# Build binaries in release mode
RUN cargo build --release --bin head --bin worker

# Stage 2: Create a minimal runtime image
FROM debian:bookworm-slim AS runtime

# Install runtime dependencies
RUN apt-get update && \
    apt-get -y install ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Copy binaries from builder stage
COPY --from=builder /workspace/target/release/head /usr/local/bin/head
COPY --from=builder /workspace/target/release/worker /usr/local/bin/worker

# Create non-root user
RUN groupadd -r cache_user && useradd -r -g cache_user cache_user

# Change ownership and switch to non-root user
RUN chown -R cache_user:cache_user /usr/local/bin/
USER cache_user
