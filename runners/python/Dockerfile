ARG RUNNER_BASE_IMAGE=ghcr.io/runcodes-icmc/runcodes-runner-base:latest
FROM ${RUNNER_BASE_IMAGE} AS runner-base

FROM python:3.15-slim-trixie AS builder

# Prevent interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Install essential build tools
RUN apt-get update && apt-get install -y \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir --prefix=/install numpy scipy imageio nltk scikit-image matplotlib pyscipopt pymcdc coverage

FROM python:3.15-slim-trixie
# Load base executables
COPY --from=runner-base /usr/bin/monitor /usr/bin/monitor
COPY --from=runner-base /usr/bin/runcodes /usr/bin/runcodes

COPY --from=builder /install /usr/local

# Submissions run as this user, never as root: the judge relies on the run being
# unable to change the image it is graded in. A system user is used so the uid
# comes from the range the distribution keeps free, and the home directory is
# /root because that is where the judge bind-mounts the workspace.
RUN groupadd --system runcodes && \
    useradd --system --gid runcodes --no-create-home --home-dir /root --shell /bin/bash runcodes

# Load script
COPY ./python-script.sh /tmp/script.sh

RUN cat /tmp/script.sh >> /usr/bin/runcodes &&\
    rm /tmp/script.sh

WORKDIR /root

USER runcodes

CMD ["/usr/bin/runcodes"]
