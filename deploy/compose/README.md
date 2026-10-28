# Image-based Compose deployment

This directory deploys the single-host stack from published images instead of
building the application from source. It pulls the four application images
(frontend, backend, judge and database) from the configured registry and lets
the judge pull the runner images.

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

- Docker Engine with Compose v2.
- Rootless Podman with its API socket started.
- `RUNCODES_EXEC_DIR` on a filesystem shared with the Podman service.
- Access to the configured registry (`docker login ghcr.io` when private).

The image-based Compose deployment runs a single replica of each service. It
does not include the observability overlay; see
[the deployment guide](https://reisaraujo-miguel.github.io/runcodes/deployment/image-compose.html#observability)
for adding it.

## Related

- [Deployment overview](https://reisaraujo-miguel.github.io/runcodes/deployment/)
- [Ansible deployment](../ansible/README.md)
- [Platform operations](https://reisaraujo-miguel.github.io/runcodes/administration/operations.html)
