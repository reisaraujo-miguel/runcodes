FROM debian:trixie-slim AS base

# Load Monitor Binary
COPY --from=ghcr.io/runcodes-icmc/runcodes-monitor:latest /usr/bin/monitor /usr/bin/monitor

# Load Base Execution Script
COPY ./base-script.sh /usr/bin/runcodes

RUN chmod +x /usr/bin/runcodes

# What runs in this image — the harness and every submission it compiles and
# executes — runs as this user, never as root. The judge relies on that: the
# run may write wherever the workspace lets it, but not the image it is being
# graded in, `/usr/bin/runcodes` and `/usr/bin/monitor` included.
#
# A system user is used so the uid comes from the range the distribution keeps
# free: a fixed one would collide, because some of the language images are built
# on a distribution that already occupies 1000 (see `README.md`), and nothing
# outside this image depends on the number. The home directory is `/root`
# because that is where the judge bind-mounts the run's workspace, and because
# the toolchains write their caches relative to `$HOME`.
RUN groupadd --system runcodes && \
    useradd --system --gid runcodes --no-create-home --home-dir /root --shell /bin/bash runcodes

USER runcodes
