---
title: Judge API
description: Internal HTTP contract between the backend and the judge — wake, event stream and output archive.
---

# Judge API

The judge exposes a small internal HTTP API consumed by the backend. It is not
part of the public product API: the backend proxies what users need and the
judge port is never public.

Base URL: `http://judge:9000` in Compose, `http://<compute private ip>:9000` in
the four-machine deployment. The default is configurable with `RUNCODES_JUDGE_URL`.

## Authentication

Every `/v1` route requires:

```http
Authorization: Bearer <JUDGE_AUTH_TOKEN>
```

The token is shared with the backend (`RUNCODES_JUDGE_TOKEN`). The judge compares
it in constant time.

If no token is configured, the judge **fails closed**: every `/v1` request gets
`503` with an explanatory body. A local-only instance can opt out with
`JUDGE_ALLOW_INSECURE=true`, which logs a warning at startup.

| Status | Meaning |
| --- | --- |
| `200` | success |
| `202` | wake accepted |
| `401` | missing or invalid bearer token |
| `503` | no token configured (or readiness failed) |

`/healthz` and `/readyz` are public so container health checks and the backend
can probe them.

## `GET /healthz`

Process liveness. Returns `200` with `text/plain` body `ok`. It checks nothing
else; it is what the container `HEALTHCHECK` uses.

## `GET /readyz`

Checks **PostgreSQL** and **Podman** with a 3-second budget.

Success:

```json
{ "status": "ok" }
```

Failure returns `503` with one of:

```json
{ "status": "database unavailable" }
{ "status": "podman unavailable" }
```

Both legs are deliberately cheap: PostgreSQL is a `Ping`, and Podman is the API
handshake (`GET /_ping` over the socket), not `podman info`. Readiness runs every
few seconds and also gates every submission, so it must not exec conmon or the
OCI runtime. `system.Version` reads like a ping in the client bindings, but the
podman server implements that endpoint as the full info path; it cost ~700ms per
probe in production and was the slowest route on both services.

The backend calls this before registering a submission. The judge's readiness is
authoritative: the Ansible deployment does not declare success until it returns
`200`.

## `POST /v1/runs/{id}`

Idempotent wake-up. Nudges the poll loop; the durable PostgreSQL queue does the
real work. The commit id must be a positive integer.

```json
{ "commit_id": 42, "status": "accepted" }
```

Returns `202 Accepted`. The endpoint does not verify that the commit exists.

## `GET /v1/runs/{id}/events`

A replayable SSE stream for one commit, consumed by the backend and relayed to
the browser.

| Query | Meaning |
| --- | --- |
| `from` | resume after the given per-commit sequence number (default 0) |

Response headers are the same as the backend's SSE endpoint
(`text/event-stream`, `no-cache`, keep-alive, `X-Accel-Buffering: no`). Frames
are formatted as:

```text
id: 3
event: case_result
data: {"type":"case_result", ...}

```

A `: ping` comment is sent every 15 seconds. The stream ends when the
subscription channel closes (the run finished and its backlog was delivered).

The judge keeps a bounded in-memory event log per commit for
`JUDGE_EVENT_RETENTION` (default 10m) after it finishes. After that, the backend
must use its persisted state.

### Event schemas

`status`:

```json
{
  "type": "status",
  "commit_id": 42,
  "seq": 1,
  "status": "compiling",
  "at": "2026-03-10T14:22:01Z"
}
```

`status` is `compiling` or `running`.

`compilation`:

```json
{
  "type": "compilation",
  "commit_id": 42,
  "seq": 2,
  "compiled": true,
  "message": "",
  "error": "",
  "at": "2026-03-10T14:22:04Z"
}
```

`case_result`:

```json
{
  "type": "case_result",
  "commit_id": 42,
  "seq": 3,
  "test_case_id": 91,
  "cpu_time": 0.12,
  "mem_usage": -1,
  "status": "correct",
  "status_message": "",
  "user_output": "42\n",
  "user_output_type": "text",
  "error_message": ""
}
```

`case_result.status` is one of `correct`, `bad_formatted_output`,
`killed_with_signal`. `cpu_time` is measured CPU time (container cgroup, with a
direct-child rusage fallback); the exercise's `cpu_time_limit_seconds` remains a
wall-clock timeout. `mem_usage` is `-1` because the monitor cannot measure it
reliably.

`artifact` (optional):

```json
{
  "type": "artifact",
  "commit_id": 42,
  "seq": 4,
  "kind": "output",
  "url": "/v1/runs/42/output"
}
```

`finished`:

```json
{
  "type": "finished",
  "commit_id": 42,
  "seq": 5,
  "status": "completed",
  "num_correct_cases": 5,
  "score": 100.0,
  "compilation_message": "",
  "compilation_error": "",
  "started_at": "2026-03-10T14:22:01Z",
  "finished_at": "2026-03-10T14:22:09Z"
}
```

`finished.status` is one of `completed`, `uncompleted`, `compilation_error`,
`server_error`, `timeout`. `score` is a percentage in `[0, 100]`.

`error`:

```json
{
  "type": "error",
  "commit_id": 42,
  "seq": 6,
  "message": "…"
}
```

The stream ends after an `error` event.

## `GET /v1/runs/{id}/output`

Serves the output archive generated by the submission's run, if one exists.

| Aspect | Behaviour |
| --- | --- |
| Content type | `application/zip` |
| Disposition | `attachment; filename="<commit-id>.zip"` |
| Missing archive | `404` with `{"error":"no output archive"}` |
| `HEAD` | same headers, no body |

The archive contains the files the submission produced. The runner's internal
reports (`compilation.*`, per-case `.output`, `.error`, `.monitor_out`,
`.monitor_err`) are excluded. The archive is stored next to the workspace and is
removed with the run's workspace unless `JUDGE_KEEP_WORKSPACES=true`.

The backend proxies this route for authenticated owners; browsers never see the
judge URL.

## Contract with the backend

| Direction | Mechanism | Durability |
| --- | --- | --- |
| backend → judge | `POST /v1/runs/{id}` wake | best effort; the queue is durable |
| backend → judge | `GET /readyz` before a submission | required |
| judge → backend | `GET /v1/runs/{id}/events` (backend subscribes) | replayable in memory for `JUDGE_EVENT_RETENTION` |
| backend → judge | `GET`/`HEAD /v1/runs/{id}/output` | file on the judge host |
| both | PostgreSQL `commits` row | durable |

The judge writes **only the claim** to `commits`. Every other write is the
backend's. This split is what lets the judge be replaced or scaled without
inventing a second source of truth.

## Related pages

- [Judge service](../services/judge.md)
- [Submissions & events API](./submissions.md)
- [Architecture](../guide/architecture.md)
