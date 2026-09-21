# Compilation image
FROM debian:trixie-slim AS builder

# Update system
RUN apt-get update && apt-get upgrade -y

# Install build tools
RUN apt-get install -y --no-install-recommends build-essential cmake libboost-all-dev pkg-config ninja-build

# Create build directory
RUN mkdir /build
WORKDIR /build

COPY ./src/ ./src/

# Build project
RUN cmake -S ./src -B .
RUN cmake --build .

# Create final image with only the built binary
FROM debian:trixie-slim AS dist
COPY --from=builder /build/monitor /usr/bin/monitor

# The runner images copy this binary out of here and the image is published on
# its own, so it carries a non-root user like they do (see
# runners/base/Dockerfile).
RUN groupadd --system runcodes && \
  useradd --system --gid runcodes --no-create-home --home-dir /root --shell /bin/bash runcodes

USER runcodes
