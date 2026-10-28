ARG RUNNER_BASE_IMAGE=ghcr.io/runcodes-icmc/runcodes-runner-base:latest
FROM ${RUNNER_BASE_IMAGE}

# The base image runs as the runcodes user; installing the language toolchain
# and appending to the harness below need root.
USER root

# System Upgrade
RUN apt-get update -y &&\
    apt-get install -y --no-install-recommends make ghc &&\
    apt-get clean &&\
    rm -rf /var/lib/apt/lists/*


# Load script
COPY ./haskell-script.sh /tmp/script.sh

RUN cat /tmp/script.sh >> /usr/bin/runcodes &&\
    rm /tmp/script.sh

WORKDIR /root

USER runcodes

CMD ["/usr/bin/runcodes"]
