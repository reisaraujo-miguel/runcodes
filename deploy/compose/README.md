# Image-based Compose deployment

This directory deploys the single-host stack from published images instead of
building the application from source. It pulls the four application images
(frontend, backend, judge and database) from the configured registry and lets
the judge pull the runner images.

The deployment runs on **Podman by default** and also supports **Docker**. Both
engines use the same Compose file; the judge always drives the host's rootless
Podman socket.

**Full documentation:** [Image-based Compose](https://reisaraujo-miguel.github.io/runcodes/deployment/image-compose.html)

## Quick start

```bash
cp .env.example .env
# fill the required secrets, the public hostname and the registry/namespace/tag
systemctl --user start podman.socket
./deploy.sh up
```

`.env.example` publishes the frontend on 80/443 for a public hostname. Rootless
Podman cannot bind those ports until the host allows it, and Caddy needs DNS and
inbound 80/443 to issue the certificate. See
[Public hostname and TLS](https://reisaraujo-miguel.github.io/runcodes/deployment/image-compose.html#public-hostname-and-tls).

The script pulls the images, creates the host execution directory, checks the
rootless Podman socket and starts the stack. It never builds from source.
`deploy.sh` uses Podman when both engines are usable. Select the engine
explicitly when needed:

```bash
CONTAINER_ENGINE=podman ./deploy.sh up   # require Podman
CONTAINER_ENGINE=docker ./deploy.sh up   # use Docker Compose v2
```

| Command | Action |
| --- | --- |
| `./deploy.sh up` | pull, start or update, show status (default) |
| `./deploy.sh pull` | pull the configured images |
| `./deploy.sh status` | show container status |
| `./deploy.sh logs` | follow the stack logs |
| `./deploy.sh restart` | restart the containers |
| `./deploy.sh down` | stop and remove the containers, including the observability stack when available; volumes are kept |
| `./deploy.sh config` | render the final Compose configuration |
| `./deploy.sh observability` | start the stack plus the telemetry stack |
| `./deploy.sh observability-logs` | follow the telemetry stack logs |
| `./deploy.sh recreate <service...>` | recreate services in place after a config or `.env` change |

`ENV_FILE=/path/to/env ./deploy.sh up` reads a different file.

## Observability

The telemetry stack is opt-in and off by default:

```bash
./deploy.sh observability
```

It starts the application plus an OpenTelemetry collector, Prometheus,
Alertmanager, Loki, Tempo, Pyroscope, Grafana and the infrastructure exporters.
Every UI and API binds to `127.0.0.1`; Grafana is at
<http://127.0.0.1:3000> and its admin password is generated into the
repository's `.observability/grafana_admin_password` on the first run.

The overlay and its configuration live at the repository root
(`docker-compose.observability.yml`, `observability/`), so this command needs a
repository checkout even though `up` only needs this directory. Point
`RUNCODES_OBSERVABILITY_DIR` and `RUNCODES_OBSERVABILITY_STATE` elsewhere to use
a copy.

After editing a telemetry configuration file, apply it with
`./deploy.sh recreate alertmanager` (or the affected service). Do not use a bare
`podman compose -f docker-compose.yml -f ../../docker-compose.observability.yml`
here: the overlay's configuration paths are absolute only while `deploy.sh`
exports them, and a bare run creates empty directories for the missing files and
fails with `not a directory`.

`down` removes the telemetry containers too; `up` never starts them.

## Compose engine

`CONTAINER_ENGINE` selects the engine. An environment value overrides the value
in `.env`.

| Value | Behaviour |
| --- | --- |
| `auto` (default) | uses `podman compose` when available, otherwise `docker compose` |
| `podman` | requires Podman and a Compose provider |
| `docker` | requires Docker Engine and Compose v2 |

The judge still needs a rootless Podman socket with either engine, because it
creates the runner containers through the Podman API.

## Image settings

```bash
RUNCODES_IMAGE_REGISTRY=ghcr.io
RUNCODES_IMAGE_NAMESPACE=runcodes-icmc
RUNCODES_IMAGE_TAG=latest
RUNCODES_RUNNER_IMAGE_PREFIX=runcodes-icmc/runcodes-runner
RUNCODES_RUNNER_IMAGE_TAG=latest
```

Application images:

```text
<registry>/<namespace>/runcodes-frontend:<tag>
<registry>/<namespace>/runcodes-backend:<tag>
<registry>/<namespace>/runcodes-judge:<tag>
<registry>/<namespace>/runcodes-database:<tag>
```

Runner images (pulled by the judge):

```text
<registry>/<runner prefix>-<language>:<runner tag>
```

`JUDGE_IMAGE_FORMAT` remains available as a full template override.

## Requirements

- Podman with a Compose provider (default), or Docker Engine with Compose v2.
- Rootless Podman on the host with `podman.socket` started. The judge needs the
  socket even when Compose runs on Docker.
- `RUNCODES_EXEC_DIR` on a filesystem shared with the Podman service.
- Access to the configured registry. `podman login ghcr.io` authenticates the
  application pulls and the judge's runner pulls. With Docker Compose, also use
  `docker login ghcr.io` for the application images.

The image-based Compose deployment runs a single replica of each service. It
does not include the observability overlay; see
[the deployment guide](https://reisaraujo-miguel.github.io/runcodes/deployment/image-compose.html#observability)
for adding it.

## Related

- [Deployment overview](https://reisaraujo-miguel.github.io/runcodes/deployment/)
- [Ansible deployment](../ansible/README.md)
- [Platform operations](https://reisaraujo-miguel.github.io/runcodes/administration/operations.html)
