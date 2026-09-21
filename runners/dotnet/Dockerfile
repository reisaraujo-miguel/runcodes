FROM mcr.microsoft.com/dotnet/sdk:10.0-noble

# Load base executables
COPY --from=ghcr.io/runcodes-icmc/runcodes-runner-base:latest /usr/bin/monitor /usr/bin/monitor
COPY --from=ghcr.io/runcodes-icmc/runcodes-runner-base:latest /usr/bin/runcodes /usr/bin/runcodes

# Submissions run as this user, never as root: the judge relies on the run being
# unable to change the image it is graded in. A system user is used so the uid
# comes from the range the distribution keeps free, and the home directory is
# /root because that is where the judge bind-mounts the workspace.
RUN groupadd --system runcodes && \
    useradd --system --gid runcodes --no-create-home --home-dir /root --shell /bin/bash runcodes

# System Upgrade
RUN apt-get update -y &&\
    apt-get install -y --no-install-recommends make &&\
    apt-get clean &&\
    rm -rf /var/lib/apt/lists/*

# Load script
COPY ./dotnet-script.sh /tmp/script.sh

RUN cat /tmp/script.sh >> /usr/bin/runcodes &&\
    rm /tmp/script.sh

WORKDIR /root

USER runcodes

CMD ["/usr/bin/runcodes"]
