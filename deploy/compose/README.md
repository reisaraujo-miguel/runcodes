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
# fill the required secrets and set the registry/namespace/tag
systemctl --user start podman.socket
./deploy.sh up
```

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
| `./deploy.sh down` | stop and remove the containers; volumes are kept |
| `./deploy.sh config` | render the final Compose configuration |

`ENV_FILE=/path/to/env ./deploy.sh up` reads a different file.

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
