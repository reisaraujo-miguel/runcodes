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
./deploy.sh linger           # once per host
./deploy.sh up               # pull the images and start the stack
./deploy.sh install-service  # once per host: the unit that starts it at boot
```

The last two commands exist because a stack started from an SSH session belongs
to that session: when the connection closes, logind stops the user manager and
SIGTERMs every rootless container. The stack then exits `0`/`143` with no OOM and
no application error, about ten seconds after logout — see
[Surviving logout and reboot](#surviving-logout-and-reboot).

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
| `./deploy.sh stop` | stop the containers but keep them (used by the systemd unit) |
| `./deploy.sh down` | stop and remove the containers, including the observability stack when available; volumes are kept |
| `./deploy.sh config` | render the final Compose configuration |
| `./deploy.sh linger` | enable systemd lingering for this user |
| `./deploy.sh install-service [--observability]` | install and start the user unit that owns the stack; `--observability` makes it own the telemetry stack too |
| `./deploy.sh uninstall-service` | stop the unit, remove it, keep the data |
| `./deploy.sh db-grants` | re-apply the service-role grants and passwords to an existing database volume |
| `./deploy.sh observability` | start the stack plus the telemetry stack |
| `./deploy.sh observability-logs` | follow the telemetry stack logs |
| `./deploy.sh recreate <service...>` | recreate services in place after a config or `.env` change |

`ENV_FILE=/path/to/env ./deploy.sh up` reads a different file.

## Surviving logout and reboot

Rootless containers live in the systemd **user manager** (`user@<uid>.service`),
not in the SSH session. Without lingering, logind stops that manager as soon as
the last session closes and SIGTERMs everything inside it. The failure looks
like a crash and is not one:

- every container stops within a few seconds of each other, about ten seconds
  after the last logout;
- `podman ps -a` shows `Exited (0)` or `Exited (143)`;
- `podman inspect` reports `OOMKilled=false`, `Error=""`;
- PostgreSQL logs `received fast shutdown request` and the backend logs
  `Shutdown requested` — they caught SIGTERM and exited cleanly.

Two commands, once per host, make the stack independent of any login session:

```bash
./deploy.sh linger            # sudo loginctl enable-linger $USER if polkit refuses
./deploy.sh install-service   # user unit: starts at boot, restarts on reboot
```

`linger` sets `Linger=yes`, which keeps the user manager (and its containers)
running with nobody logged in. `install-service` writes
`~/.config/systemd/user/runcodes-compose.service` and enables it, so the stack is
supervised by systemd instead of by your terminal:

```bash
systemctl --user status runcodes-compose.service
systemctl --user restart runcodes-compose.service
journalctl --user -u runcodes-compose.service -n 50
```

The unit owns the **application** by default. A deployment that also runs the
telemetry overlay has to say so, otherwise a reboot brings back the application
and leaves Prometheus, Grafana, Loki, Tempo and the collector stopped:

```bash
./deploy.sh install-service --observability
```

Re-run it without the flag to take the telemetry stack back out of the unit. A
plain `./deploy.sh up` never prunes a telemetry stack it did not start: only a
unit that owns every service of the project runs `--remove-orphans`.

Verify with `loginctl show-user "$USER" -p Linger` → `Linger=yes`, then log out
completely, wait a minute and run `./deploy.sh status`.

`up`, `restart` and `observability` refuse to start a rootless stack without
lingering; `RUNCODES_SKIP_LINGER_CHECK=1` overrides that and the stack will stop
at the next logout. `RUNCODES_SKIP_PULL=1` starts the images already on the host
(the unit sets it, so a boot never depends on the registry).

## Observability

The telemetry stack is opt-in and off by default:

```bash
./deploy.sh observability
```

To make it survive a reboot, install the unit with
`./deploy.sh install-service --observability` instead of plain
`install-service`.

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
