FROM ghcr.io/runcodes-icmc/runcodes-runner-base:latest

# The base image runs as the runcodes user; installing the language toolchain
# and appending to the harness below need root.
USER root

# Load base executables
COPY --from=ghcr.io/runcodes-icmc/runcodes-runner-base:latest /usr/bin/monitor /usr/bin/monitor
COPY --from=ghcr.io/runcodes-icmc/runcodes-runner-base:latest /usr/bin/runcodes /usr/bin/runcodes

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
