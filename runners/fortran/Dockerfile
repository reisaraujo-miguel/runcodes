FROM ghcr.io/runcodes-icmc/runcodes-runner-base:latest

# The base image runs as the runcodes user; installing the language toolchain
# and appending to the harness below need root.
USER root

# Install language-specific dependencies
RUN apt update &&\
    apt-get install -y --no-install-recommends gcc gfortran make &&\
    apt-get clean &&\
    rm -rf /var/lib/apt/lists/*

# Load script
COPY ./fortran-script.sh /tmp/script.sh

RUN cat /tmp/script.sh >> /usr/bin/runcodes &&\
    rm /tmp/script.sh

WORKDIR /root

USER runcodes

CMD ["/usr/bin/runcodes"]
