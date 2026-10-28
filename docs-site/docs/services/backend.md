---
title: Backend API
description: Responsibilities, internal layout, configuration, rate limits, readiness and observability of the Go backend.
---

# Backend API

The backend is a Go HTTP service built on **chi**. It is the only service that
writes application state: it owns authentication, authorization, every
PostgreSQL write, every S3 upload, and the SSE relay that carries judge events to
the browser.

## Responsibilities

- **Authentication and authorization** — JWT sessions in an `HttpOnly` cookie,
  role and class-relation checks on every route.
- **Classes and exercises** — offerings, enrollments, exercises, test cases and
  file metadata.
- **Submissions** — validate the upload, check the judge, upload the source to
  SeaweedFS, insert the queued commit, wake the judge.
- **Event persistence and relay** — persist judge events, serve snapshots, relay
  live SSE to the browser.
- **Reconciliation** — a background sweeper marks stale `compiling`/`running`
  commits as `server_error`.
- **Platform settings** — contact email and disclaimer.
- **Readiness** — `/healthz` and `/readyz`.

It deliberately does **not** run submitted code, generate output archives, or
read test-case contents for grading.

## Internal layout

```text
backend/
├── main.go            startup, telemetry, DB, server lifecycle
├── router.go          route table, middleware, rate limits, CORS
├── config/            environment loading, defaults and validation
├── handlers/          HTTP handlers (request parsing, status codes)
├── services/          business logic and SQL
├── models/            JSON and domain types
├── database/          connection pool, InitDB
├── cache/             optional Redis cache (no-op when unreachable)
├── storage/           SeaweedFS S3 client and bucket helpers
├── judge/             judge HTTP client, SSE hub, reconciliation
├── mailer/            SMTP client
├── validation/        input validation, JWT setup
└── telemetry/         OpenTelemetry, pprof, logging
```

Handlers are thin: they decode and validate the request, call a service, and map
service errors to HTTP status codes. Services own the SQL and the access rules.
This split is what makes the service tests (for offerings, exercises,
submissions and verification) meaningful without an HTTP server.

## Startup sequence

`main.go`:

1. parses `-debug` and loads `.env` if present;
2. calls `config.Load()` (fails fast on a missing or malformed required value);
3. sets up OpenTelemetry and the logger;
4. starts the optional internal pprof listener (`PPROF_ADDR`);
5. initializes the PostgreSQL pool and JWT;
6. pings Redis (a failure is a warning; the cache degrades to a no-op);
7. starts the judge reconciliation sweeper;
8. serves HTTP with a 10 s read-header timeout and 120 s idle timeout.

`WriteTimeout` is intentionally unset because the SSE submission stream is a
long-lived response. Graceful shutdown gives the HTTP server 10 s and telemetry
exporters 5 s to flush.

## API surface

All routes are under `/api/v1`, except the probes and the public settings route.
The full endpoint reference starts at [API](../api/index.md).

| Group | Examples | Who |
| --- | --- | --- |
| Public | signup, login, verify email, resend, password recovery, public settings | anyone |
| Session | auth, refresh, logout | signed in |
| Self-service | profile, password, my classes, open exercises | signed in + confirmed |
| Submissions | create, events, source, output, history | signed in + confirmed |
| Authoring | exercises, test cases, compilation/attached files | staff of the class |
| Class management | list, create, edit, delete, members | professor/admin; owner for edit/delete/members |
| Admin | users, any class, settings, file types, role preview | admin |

The route table is the source of truth: [`backend/router.go`](https://github.com/reisaraujo-miguel/runcodes/blob/main/backend/router.go).

## Middleware

Order matters. The backend installs, in order:

1. **OpenTelemetry HTTP middleware** — creates the request span.
2. **Trace-context middleware** — guarantees a `TraceId` response header even
   when tracing is disabled.
3. **HTTP metrics** — request duration and active requests, labelled with the
   chi route pattern.
4. **Request logger** — structured JSON/pretty logs; request/response bodies
   only in debug mode when the caller sends `Debug: reveal-body-logs`.
5. **Recoverer** — converts a panic into a `500` instead of killing the process.
6. **CORS** — one explicit frontend origin, credentials allowed.
7. **Trusted-proxy hop count** — one proxy (Caddy) in front of the API.
8. **Rate limiting** — 600 requests/minute per client IP, with tighter limits:
   20/minute for signup and login, 10/minute for verification and
   password-recovery routes, and 5/minute per account for enrollment codes.

A malformed payload is logged with a replayable `curl` command (the
`Authorization` header is removed) when body logging is enabled.

## Submissions and SSE

`POST /api/v1/submissions`:

1. caps the multipart body at 10 MiB + 1 MiB of overhead;
2. validates the exercise, deadline and file extension;
3. calls the judge's `/readyz` — if it fails, returns `503` **before** creating
   anything;
4. uploads the source to the `<prefix>-commits` bucket;
5. opens a transaction, inserts the `queued` commit, calls the judge's wake
   endpoint, and commits only if the wake succeeded;
6. on any failure, rolls back and deletes the uploaded object.

The ordering is deliberate: the judge must never see a commit it cannot be woken
for. If the wake fails, both the row and the object are removed.

`GET /api/v1/submissions/{id}/events`:

- authorizes the commit owner or a member of staff who teaches the exercise;
- loads the persisted snapshot (commit, case results, artifacts);
- checks whether the judge has an output archive and advertises it to the owner;
- writes the `snapshot` event and, if the commit is not terminal, subscribes to
  the judge's SSE stream and relays events;
- sends a `: ping` comment every 15 seconds;
- ends when the commit reaches a terminal state.

Events are persisted before they are forwarded. The relay is a convenience on
top of the database, not the source of truth.

## Reconciliation

If a judge process dies mid-run, the commit row stays `compiling` or `running`.
`judge.StartReconciliation` periodically looks for rows older than
`RUNCODES_JUDGE_STALE_TIMEOUT` (default 35 minutes) and marks them
`server_error`. Claiming is still crash-safe: a restarted judge can claim other
queued commits immediately.

## Readiness

| Endpoint | Behaviour |
| --- | --- |
| `GET /healthz` | process liveness; always `200` when the process is up |
| `GET /readyz` | checks PostgreSQL (required) and the judge (informational) |

`/readyz` returns `503` only when PostgreSQL is unavailable. The judge is reported
as `ok` or `unavailable` in the body; a judge outage breaks submissions and
grading but not login, classes or browsing, so it must not remove the API from a
load balancer.

```json
{
  "status": "ok",
  "database": "ok",
  "judge": "ok"
}
```

## Configuration

Every setting is read once by `config.Load()` and published as `config.C`. A
missing required variable or an unparseable value aborts startup with a message
naming the variable. Required values are `RUNCODES_API_PORT`,
`RUNCODES_JWT_SECRET`, the database host/port/user/password/name,
`RUNCODES_PUBLIC_URL`, `RUNCODES_SMTP_HOST` and `RUNCODES_SMTP_FROM`.

`DEBUG_MODE=true` (or `-debug`, which wins) switches to human-readable logs and
drops the `Secure` flag from the session cookie for plain-HTTP localhost.
Request/response bodies are logged only in debug mode and only when asked.

See [Configuration](../reference/configuration.md#backend) for the full table.

## Cache and storage

Redis is **optional**. `cache.Ping` failure is logged as a warning and the cache
becomes a no-op; every request still works. Cached entries include class
metadata and exercise lists, invalidated on writes.

SeaweedFS is accessed over the S3 API with four buckets derived from
`RUNCODES_S3_BUCKET_PREFIX`:

| Bucket | Contents |
| --- | --- |
| `<prefix>-commits` | submitted source / zip |
| `<prefix>-cases` | test-case inputs, expected outputs, extra files |
| `<prefix>-files` | exercise compilation files and attachments |
| `<prefix>-outputfiles` | output archives generated by the judge |

## Observability

The backend is instrumented but exports nothing unless
`OTEL_EXPORTER_OTLP_ENDPOINT` is set.

- Inbound HTTP spans (`otelchi`), outbound judge/S3 HTTP spans (`otelhttp`) and
  database spans (`otelsql`).
- HTTP metrics use the chi **route pattern** (`/api/v1/exercises/{id}`) rather
  than the raw path, keeping cardinality bounded.
- Responses include `TraceId` (32 hex characters) and `X-Trace-Sampled`.
- Logs keep the stdout format and add `trace_id`/`span_id` when a span is
  active; the same records are exported over OTLP to Loki.
- `PPROF_ADDR` starts an internal pprof listener for Alloy/Pyroscope. It is never
  exposed on the API listener.

See [Observability](../observability/index.md) for the whole pipeline.

## Related pages

- [API reference](../api/index.md)
- [Judge](./judge.md)
- [Database](./database.md)
- [Backend configuration](../reference/configuration.md#backend)
