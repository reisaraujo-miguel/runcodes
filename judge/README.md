# RunCodes — Judge

The execution engine for RunCodes submissions. It is the successor to the
legacy Python `compiler-engine` and runs the language images from `runners/`
with **rootless podman** instead of Docker.
The image a run uses is `ghcr.io/runcodes-icmc/runcodes-runner-<language>`
(`JUDGE_IMAGE_FORMAT` overrides the format), built together with the in-container
monitor from `monitor/` — the three parts are released as one contract.

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

**API authentication.** `JUDGE_AUTH_TOKEN` (or `RUNCODES_JUDGE_TOKEN`) is the
shared bearer token the backend presents. While it is unset the judge refuses
every `/v1` request instead of serving them to anyone who can reach the port,
because that API exposes other people's submissions and outputs. A local
instance that is not reachable from anywhere else can opt out with
`JUDGE_ALLOW_INSECURE=true`, which logs a warning at startup.

**Container limits.** Graded code is untrusted, so every container is capped from
outside with cgroup limits, independently of the in-container monitor (which the
submission shares privileges with and can defeat):

| Variable                       | Default  | Purpose                                 |
| ------------------------------ | -------- | --------------------------------------- |
| `JUDGE_CONTAINER_MEMORY_BYTES` | 1536 MiB | memory and swap cap, per concurrent run |
| `JUDGE_CONTAINER_PIDS_LIMIT`   | 256      | process/thread cap (fork bombs)         |
| `JUDGE_CONTAINER_CPU_QUOTA`    | 100000   | CPU per 100 ms period (1 full core)     |

The memory cap is deliberately above the largest default a language image sets
(1 GiB, for the memory-hungry toolchains): a run that reaches the image's own
limit is reported as exceeding it, while one that reaches the cgroup cap is only
seen as a process killed by a signal. It applies per concurrent run, so
`JUDGE_CONCURRENCY` multiplies it.

The equivalent limits _inside_ the container are the image's business. The judge
writes `monitor_max_fs` / `monitor_max_ms` / `compilation_timeout` into
`container.config` only when `JUDGE_MONITOR_MAX_FILE_SIZE` /
`JUDGE_MONITOR_MAX_MEM_SIZE` / `JUDGE_DEFAULT_COMPILATION_TIMEOUT` are set (all
default to 0, which sends nothing and leaves each language image's own value in
place — `monitor_max_ms` is 1 GiB for Python and unset for the JVM images, and Go
and C# compile with a 60 s timeout). An operator who sets one overrides every
language, which is what those variables are for.

The judge's own patience for the compilation phase is a separate number:
`JUDGE_COMPILATION_WAIT` (default 2m) is how long it waits for the
`compilation.*` milestones. It has to outlast the image's compilation timeout
plus container startup, or the judge abandons a compilation the container is
still allowed to finish and reports a milestone timeout instead of the
compiler's error. The two are validated against each other when both are set.

Containers are also created with **no network namespace**, so submitted code
cannot reach the internet or the other services on the compose network, and with
**no-new-privileges**, so a setuid binary or a file capability in any language
image cannot hand a submission rights the monitor does not have. Both are
asserted in `internal/podman/spec_test.go`, together with the namespaces and the
workspace mount: the entire boundary between a submission and the rest of the
platform is that spec.

**Read bounds.** Expected outputs are downloaded into a private directory beside
the workspace (never into it), and the two files a case is graded from are read
under `JUDGE_MAX_COMPARE_FILE_BYTES` (default 16 MiB). An output larger than that
is a failed case rather than a truncation. The published output archive is capped
by `JUDGE_MAX_ARTIFACT_BYTES` (default 64 MiB).

**Run budget.** `JUDGE_MAX_RUN_DURATION` (default 30m) bounds one claimed commit
end to end, so a stalled image pull, container create or S3 read cannot pin a
worker slot forever. Keep it above the longest legitimate run and aligned with
the backend's `RUNCODES_JUDGE_STALE_TIMEOUT`.

**Container images.** The judge writes `run_nonce` into `container.config` and only
accepts progress milestones that carry it, and it sends the exercise's per-case
limits as `ms_<id>` / `fs_<id>` / `stack_<id>`. Both are part of the contract in
`DESIGN.md`, so the language images must be rebuilt from `runners/`
together with the judge: an image built from an older base script prints bare
milestones, which the judge logs as a warning and ignores (the run then times
out).

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

`/v1` endpoints require `Authorization: Bearer $JUDGE_AUTH_TOKEN`. Without a
configured token they return `503` unless `JUDGE_ALLOW_INSECURE=true` is set.

## Limitations

- A run that is in flight when the judge crashes stays `compiling`/`running` in
  Postgres; the backend's reconciliation sweeper marks such rows
  `server_error`. Claiming is otherwise crash-safe and multi-instance safe.
- Finished runs remain replayable in memory for `JUDGE_EVENT_RETENTION`; after
  that, clients must read the terminal state from the backend's database.
