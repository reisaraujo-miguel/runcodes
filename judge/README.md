# Judge

The compute-only execution engine for RunCodes submissions. The judge claims
queued commits from PostgreSQL, reads source and test cases from SeaweedFS, runs
the matching language image with **rootless Podman**, grades the output, and
streams events to the backend. It never writes results or artifacts.

**Full documentation:** [Judge](https://reisaraujo-miguel.github.io/runcodes/services/judge.html)
· [Judge design & API](https://reisaraujo-miguel.github.io/runcodes/api/judge.html)

## Build and run

```bash
cd judge
make build        # bin/judge
make test
make run
```

The build requires the Podman remote/storage opt-out tags handled by the
`Makefile`. The judge needs a rootless Podman socket and a writable shared
execution directory:

```bash
systemctl --user start podman.socket
mkdir -p ../exec && chmod 0777 ../exec
```

In Compose, the `judge-exec-init` service prepares `./exec` and the socket is
mounted automatically. Read `readyz` to confirm both dependencies:

```bash
curl -s http://localhost:9000/readyz
```

## Configuration

All configuration is environment-based; see [`judge/.env.example`](.env.example)
and the [configuration reference](https://reisaraujo-miguel.github.io/runcodes/reference/configuration.html#judge).
`JUDGE_AUTH_TOKEN` (or `RUNCODES_JUDGE_TOKEN`) is required for `/v1`; the judge
refuses those routes when it is unset.

## Integration test

```bash
make integration
```

Starts ephemeral PostgreSQL and SeaweedFS containers and runs the real engine
against a real language image through rootless Podman. See the
[integration notes](integration/README.md).

## Layout

`internal/worker` claims and processes commits; `internal/podman` builds the
container spec; `internal/engine` runs and grades; `internal/events` is the
replayable event hub; `internal/language` maps file extensions to images.

## Related

- [Runner images & monitor](https://reisaraujo-miguel.github.io/runcodes/services/runners-and-monitor.html)
- [Architecture](https://reisaraujo-miguel.github.io/runcodes/guide/architecture.html)
- [Observability](https://reisaraujo-miguel.github.io/runcodes/observability/)
