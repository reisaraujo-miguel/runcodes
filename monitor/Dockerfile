# Compilation image
FROM debian:trixie-slim AS builder

# Update system
RUN apt-get update && apt-get upgrade -y

# Install build tools
RUN apt-get install -y build-essential cmake libboost-all-dev pkg-config ninja-build

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
