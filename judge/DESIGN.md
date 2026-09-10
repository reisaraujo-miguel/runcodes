# Judge service — design & contract

The judge is the execution engine for run.codes submissions. It replaces the
legacy Python `compiler-engine` (see `tmp/compiler-engine` for reference) and
runs the language images from `tmp/compiler-images` with **rootless podman**.

## Responsibility split (agreed)

- **Backend** owns all persistence: it creates the `commits` row, uploads the
  submitted source and the test-case files to S3, writes every status/result it
  receives, and relays SSE to the client.
- **Judge** is compute-only: it **claims** queued commits from Postgres
  (`SELECT ... FOR UPDATE SKIP LOCKED`, writing only the claim), reads what it
  needs from Postgres + S3, runs the container, grades, and **streams events**.
  It never writes results and never uploads artifacts.

Postgres is the durable queue: the backend inserts a commit with
`status = 'queued'`; judge workers claim it by atomically setting
`status = 'compiling'`, `compilation_started = now()`. A crash leaves the row
claimed; the backend detects the missing terminal event and reconciles.

## PostgreSQL (new schema, `database/schema/02-schema.sql`)

Judge reads `commits`, `exercises_test_cases` (+ `exercises_test_cases_files`),
`exercises_compilation_files` and `exercises`. Judge writes **only** the claim:

```sql
UPDATE commits SET status = 'compiling', compilation_started = now()
WHERE id = (
  SELECT id FROM commits WHERE status = 'queued'
  ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1
)
RETURNING id;
```

The commit's language is derived from the extension of `s3_key` (as the legacy
engine derived it from the filename). The judge does **not** read
`allowed_file_types` for images: the language→image mapping lives in the judge
(`internal/language`).

## S3 layout (SeaweedFS, path-style)

| Bucket                     | Object                      | Contents                       |
| -------------------------- | --------------------------- | ------------------------------ |
| `<prefix>-commits`         | `commit.s3_key`             | submitted source (or `.zip`)   |
| `<prefix>-cases`           | `<case_id>/in`              | test-case stdin                |
| `<prefix>-cases`           | `<case_id>/out`             | expected stdout                |
| `<prefix>-cases`           | `<case_id>/files/<name>`    | extra files for the case       |
| `<prefix>-files`           | `compilationfiles/<exercise_id>/<path>` | exercise compilation files |

## State machine (mirrors the legacy engine)

```
queued --claim--> compiling --(compilation.err non-empty)--> compilation_error
                       |                                              ^
                       +--(compilation.ok)--> running --> completed   |
                                                        \-> uncompleted
   any unexpected failure ----------------------------------> server_error
   wall-clock budget exceeded ------------------------------> timeout
```

## Judge HTTP API (consumed by the backend)

All endpoints are JSON. Auth: a shared bearer token (`JUDGE_AUTH_TOKEN`), sent by
the backend as `Authorization: Bearer <token>`. If the token is unset the API is
unauthenticated (dev mode).

### `GET /healthz`

`200` when the process is up. `GET /readyz` additionally verifies podman and
Postgres are reachable. The backend calls `/readyz` before registering a
submission.

### `POST /v1/runs/{commitID}`

Idempotent wake-up: asks the judge to make sure it processes the commit (it
nudges the poll loop; the durable queue does the real work). `202 Accepted`.

### `GET /v1/runs/{commitID}/events` — SSE

A replayable stream of events for one commit. The judge keeps a bounded in-memory
event log per commit for `JUDGE_EVENT_RETENTION` (default 10m) after it finishes,
so the backend can (re)subscribe and catch up. Query `?from=<seq>` resumes after
the last seen event id. `id:` is the per-commit monotonically increasing
sequence. Comment heartbeats (`: ping`) are sent every 15s.

Event types (SSE `event:` field):

| event         | data                                                                                              |
| ------------- | ------------------------------------------------------------------------------------------------- |
| `status`      | `{"type":"status","commit_id":1,"seq":1,"status":"compiling","at":"RFC3339"}`                     |
| `compilation` | `{"type":"compilation","commit_id":1,"seq":2,"compiled":true,"message":"...","error":"...","at":"..."}` |
| `case_result` | `{"type":"case_result","commit_id":1,"seq":3,"test_case_id":7,"cpu_time":0.12,"mem_usage":-1,"status":"correct","status_message":"","user_output":"...","user_output_type":"text","error_message":""}` |
| `artifact`    | `{"type":"artifact","commit_id":1,"seq":4,"kind":"output","url":"/v1/runs/1/output"}` (optional)   |
| `finished`    | `{"type":"finished","commit_id":1,"seq":5,"status":"completed","num_correct_cases":5,"score":100.00,"compilation_message":"...","compilation_error":"...","started_at":"...","finished_at":"..."}` |
| `error`       | `{"type":"error","commit_id":1,"seq":6,"message":"..."}` (stream ends)                             |

`status` values: `compiling`, `running`.
`case_result.status` maps to the DB enum: `correct`, `bad_formatted_output`,
`killed_with_signal`.
`finished.status` maps to the DB enum: `completed`, `uncompleted`,
`compilation_error`, `server_error`, `timeout`.
`score` is a percentage in `[0,100]` (the new `commits.score` CHECK).

### `GET /v1/runs/{commitID}/output`

Authenticated download of the generated output zip (monitor output, per-case
stdout/stderr), if produced. The backend may fetch this and store it in S3.

## Container contract (from `tmp/compiler-images`)

The per-run workspace is bind-mounted at `/root`; the image's `CMD
/usr/bin/runcodes` sources `container.config` there. The judge writes:

```
monitor_max_fs=5242880
monitor_max_ms=268435456
compilation_timeout=10
src_file='submission.c'
t_<case_id>=5
```

and expects these log milestones: `compilation.start` / `compilation.done` (only
for compilable languages — the base script skips `compile` otherwise) and
`run.start` / `run.done`. Outputs are read from the shared workspace afterwards
(`compilation.out/.err`, `<id>.output/.error/.monitor_out`).

## Configuration

See `.env.example`. Build requires podman's remote/storage opt-out tags (the
local storage drivers need `gpgme`/`btrfs` headers that a judge host does not
have); use `make build`.
