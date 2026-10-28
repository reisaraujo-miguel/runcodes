ARG RUNNER_BASE_IMAGE=ghcr.io/runcodes-icmc/runcodes-runner-base:latest
FROM ${RUNNER_BASE_IMAGE}

# The base image runs as the runcodes user; installing the language toolchain
# and appending to the harness below need root.
USER root

# Install the Verilog toolchain. Keeping this image on the same Debian base as
# the monitor avoids the glibc mismatch of the old third-party iverilog image.
RUN apt-get update &&\
    apt-get install -y --no-install-recommends iverilog make &&\
    apt-get clean &&\
    rm -rf /var/lib/apt/lists/*

# Load script
COPY ./verilog-script.sh /tmp/script.sh

RUN cat /tmp/script.sh >> /usr/bin/runcodes &&\
    rm /tmp/script.sh

WORKDIR /root

USER runcodes

CMD ["/usr/bin/runcodes"]
