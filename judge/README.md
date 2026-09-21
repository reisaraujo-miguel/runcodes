# RunCodes — Judge

The execution engine for run.codes submissions. It is the successor to the
legacy Python `compiler-engine` (`tmp/compiler-engine`) and runs the language
images from `tmp/compiler-images` with **rootless podman** instead of Docker.

The judge is **compute-only**: it claims queued commits from PostgreSQL, reads
its inputs from S3, runs the container, grades the outputs and streams the
results to the backend over SSE. It never writes results or artifacts itself —
the backend owns all Postgres/S3 persistence (see `DESIGN.md` for the full
contract).

## How a submission flows

1. The backend registers the commit (`status = 'queued'`), uploads the source
   and test-case files to S3, and calls `POST /v1/runs/{id}` to wake the judge.
2. A worker claims the commit from Postgres with
   `SELECT ... FOR UPDATE SKIP LOCKED` (writing only the claim), so at most
   `JUDGE_CONCURRENCY` containers run at once and no submission is ever lost.
3. The worker prepares a per-commit workspace, mounts it into the language
   image at `/root` and drives the image's script through the
   `compilation.start/done` and `run.start/done` milestones.
4. Each status change, per-case verdict and the terminal `finished` event is
   published to the per-commit event log, which the backend consumes through
   `GET /v1/runs/{id}/events` (SSE, replayable).

## Build & run

```sh
make build        # produces bin/judge
make test
make run
```

The build **requires** the podman remote/storage opt-out tags (see the
`Makefile`): without them the podman bindings need `gpgme`/`btrfs` development
headers that a judge host does not have.

Rootless podman must expose its API socket:

```sh
systemctl --user start podman.socket   # unix:///run/user/$UID/podman/podman.sock
```

### Shared execution directory

The judge creates one workspace per commit under `JUDGE_EXEC_DIR` (mounted at
`/exec`) and shares it with the podman service through `JUDGE_EXEC_DIR_REMOTE`,
so the language containers can bind-mount it. The directory must exist and be
writable by the judge's UID.

When running through the root `docker-compose.yml` this is handled
automatically: the one-shot `judge-exec-init` service creates `./exec` and makes
it writable before the judge starts, so a fresh checkout needs no manual setup.
When running the judge directly (outside Compose), create it yourself:

```sh
mkdir -p ./exec && chmod 0777 ./exec
```

## Configuration

All configuration is via environment variables; see `.env.example`.

Zip submissions are unpacked with a per-entry limit
(`JUDGE_MAX_EXTRACT_FILE_BYTES`, default 64 MiB) and a total expanded-size limit
(`JUDGE_MAX_EXTRACT_BYTES`, default 256 MiB) so an archive cannot fill the
shared execution directory.

## Endpoints

| Method | Path                   | Description                                     |
| ------ | ---------------------- | ----------------------------------------------- |
| GET    | `/healthz`             | liveness                                        |
| GET    | `/readyz`              | Postgres + podman reachable (backend pre-check) |
| POST   | `/v1/runs/{id}`        | wake the poll loop for a commit                 |
| GET    | `/v1/runs/{id}/events` | SSE event stream (`?from=<seq>` to resume)      |
| GET    | `/v1/runs/{id}/output` | the generated output archive                    |

`/v1` endpoints require `Authorization: Bearer $JUDGE_AUTH_TOKEN` when the token
is set.

## Limitations

- A run that is in flight when the judge crashes stays `compiling`/`running` in
  Postgres; the backend's reconciliation sweeper marks such rows
  `server_error`. Claiming is otherwise crash-safe and multi-instance safe.
- Finished runs remain replayable in memory for `JUDGE_EVENT_RETENTION`; after
  that, clients must read the terminal state from the backend's database.
