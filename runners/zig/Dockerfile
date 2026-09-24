FROM fedora:44
# fedora <3

# Load base executables
COPY --from=ghcr.io/runcodes-icmc/runcodes-runner-base:latest /usr/bin/monitor /usr/bin/monitor
COPY --from=ghcr.io/runcodes-icmc/runcodes-runner-base:latest /usr/bin/runcodes /usr/bin/runcodes

# shadow-utils is what provides useradd and groupadd, and the Fedora base image
# ships neither: without it the run cannot be dropped to a non-root user.
RUN dnf install -y zig make shadow-utils && \
    groupadd --system runcodes && \
    useradd --system --gid runcodes --no-create-home --home-dir /root --shell /bin/bash runcodes && \
    dnf clean all

# Load script
COPY ./zig-script.sh /tmp/script.sh

RUN cat /tmp/script.sh >> /usr/bin/runcodes &&\
    rm /tmp/script.sh

WORKDIR /root

USER runcodes

CMD [ "/usr/bin/runcodes" ]
