FROM ghcr.io/runcodes-icmc/runcodes-runner-base:latest

# The base image runs as the runcodes user; installing the language toolchain
# and appending to the harness below need root.
USER root

# Configure Timezone
ENV TZ=America/Sao_Paulo
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

# Install language-specific dependencies
RUN apt-get update &&\
    apt-get install -y --no-install-recommends gcc g++ r-base r-base-dev \
    r-recommended littler libssl-dev libcurl4-openssl-dev \
    libssh2-1-dev make &&\
    apt-get clean &&\
    rm -rf /var/lib/apt/lists/*

# Setup R
COPY install_packages.r /bin/install_packages.r
COPY run_codes_functions.r /bin/run_codes_functions.r

RUN Rscript --silent --slave --vanilla /bin/install_packages.r

# Load script
COPY ./r-script.sh /tmp/script.sh

RUN cat /tmp/script.sh >> /usr/bin/runcodes &&\
    rm /tmp/script.sh

WORKDIR /root

USER runcodes

CMD ["/usr/bin/runcodes"]
