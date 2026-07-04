# Copyright The Kubeflow Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM rust:1.97-bullseye AS builder

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

# Create a non-root user that matches the runtime security context.
RUN groupadd --gid 1000 cache_user && useradd --uid 1000 --gid cache_user cache_user

# Change ownership and switch to non-root user
RUN chown -R cache_user:cache_user /usr/local/bin/
USER cache_user
