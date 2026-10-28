---
title: API overview & authentication
description: Base URLs, authentication, error format, rate limits, trace headers and the complete endpoint index.
---

# API overview & authentication

The backend exposes a JSON API under `/api/v1`. In both supported deployments
it is reached through Caddy at the same origin as the SPA, so the base URL is:

```text
http://localhost:8080/api/v1        # single-host Compose (Podman or Docker)
https://<your-domain>/api/v1        # four-machine deployment
```

The backend itself listens on `:8443` and is loopback-only in Compose. The judge
has a separate internal API; see [Judge API](./judge.md).

## Authentication

The browser uses a session cookie:

```http
Set-Cookie: jwt=<signed JWT>; HttpOnly; SameSite=Strict; Secure; Path=/
```

It is set by `POST /api/v1/user/login` and is valid for 30 minutes, sliding.
`POST /api/v1/auth/refresh` renews it. The frontend calls the API with
`credentials: "include"`; JavaScript cannot read the token.

Scripts can send the token as a bearer header instead:

```bash
curl -H "Authorization: Bearer $TOKEN" https://<host>/api/v1/user/profile
```

There is no API-key or OAuth flow. `POST /api/v1/user/login` returns an empty
body and the cookie; to obtain a token for a script, capture the `jwt` cookie
from the login response:

```bash
curl -c cookies.txt -H 'Content-Type: application/json' \
  -d '{"email":"admin@admin.com","password":"..."}' \
  http://localhost:8080/api/v1/user/login

TOKEN=$(awk '$6 == "jwt" { print $7 }' cookies.txt)
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/user/profile
```

## Authorization

Authentication is checked by the JWT middleware. Authorization is checked per
route:

| Requirement | Routes |
| --- | --- |
| signed in | all protected routes |
| confirmed email | everything except logout |
| professor or admin | class management routes |
| class owner or co-professor | exercise/test-case/file authoring |
| class membership | reading and submitting to a class |
| admin | `/api/v1/admin/*` |

A caller who is authenticated but not allowed receives `403`. A resource whose
existence is hidden from the caller (a gradebook, another user's source or output
download) returns `404` instead. A missing or expired session receives `401`.

## Request and response conventions

- Requests and responses are JSON unless noted (submissions and authoring
  uploads are `multipart/form-data`; source/output downloads are streams).
- Times are RFC 3339 strings with an offset.
- Errors use a single key:

  ```json
  { "error_msg": "the deadline for this exercise has passed" }
  ```

- `201 Created` responses include the created object (or, for a submission, the
  commit handle).
- `204 No Content` responses have no body.
- Most lists are bounded. Admin user and class listings accept `limit` and
  `offset` (default 50, maximum 200). Submission history returns at most 50
  entries. Other lists are small by design and are not paginated.

## Rate limits

| Scope | Limit | Key |
| --- | --- | --- |
| API (all routes) | 600 requests/minute | client IP |
| Signup and login | 20 requests/minute | client IP |
| Email flows (verify, resend, forgot, reset) | 10 requests/minute | client IP |
| Enrollment with a class code | 5 attempts/minute | authenticated account |

The limits are in-memory per process; a multi-instance deployment does not share
them. The API-wide ceiling is sized for a classroom behind one NAT address, so a
whole lab can share it. The credential and mail routes are tighter on purpose.

Request bodies are capped at 1 MiB for non-multipart requests, with a 30-second
read deadline. Multipart uploads have their own caps and a 5-minute read
deadline; see the endpoint page for the exact size. A rejected oversized request
returns `413`.

The client IP comes from the address Caddy appends to `X-Forwarded-For`. The
backend trusts exactly one proxy hop and canonicalizes the address, so a
client-supplied header cannot choose its own rate-limit key.

## Tracing headers

Every response carries:

| Header | Meaning |
| --- | --- |
| `TraceId` | 32 hex-character trace id, even when tracing is disabled |
| `X-Trace-Sampled` | whether the trace was sampled |

The id can be pasted into Tempo or grepped in Loki. Send a W3C `traceparent` to
join an existing trace.

## CORS

The API allows exactly one frontend origin (`FRONTEND_ORIGIN`, default
`http://localhost:5173`), with credentials enabled. The production SPA is
same-origin through Caddy, so CORS is only relevant for a separate development
origin. A wildcard origin is rejected at startup.

## Endpoint index

### Probes and public settings

| Method | Path | Description |
| --- | --- | --- |
| GET | `/healthz` | liveness (no dependencies) |
| GET | `/readyz` | PostgreSQL required, judge informational |
| GET | `/api/v1/settings/public` | contact email and disclaimer |

### Accounts

| Method | Path | Who |
| --- | --- | --- |
| POST | `/api/v1/user/signup` | public |
| POST | `/api/v1/user/login` | public |
| POST | `/api/v1/user/verify-email` | public |
| POST | `/api/v1/user/resend-verification` | public |
| POST | `/api/v1/user/forgot-password` | public |
| POST | `/api/v1/user/reset-password` | public |
| POST | `/api/v1/user/logout` | signed in |
| GET | `/api/v1/auth` | signed in |
| POST | `/api/v1/auth/refresh` | signed in |
| POST | `/api/v1/auth/stop-preview` | preview session |
| GET / PUT | `/api/v1/user/profile` | signed in |
| PUT | `/api/v1/user/password` | signed in |

### Classes and exercises

| Method | Path | Who |
| --- | --- | --- |
| GET | `/api/v1/user/offerings` | signed in |
| GET | `/api/v1/user/offerings/{id}` | class member |
| GET | `/api/v1/user/exercises` | signed in |
| POST | `/api/v1/offerings/enroll` | signed in |
| DELETE | `/api/v1/offerings/{id}/enrollment` | signed in |
| GET | `/api/v1/offerings` | professor/admin |
| POST | `/api/v1/offerings/create` | professor/admin |
| POST | `/api/v1/offerings/import` | professor/admin |
| GET | `/api/v1/offerings/{id}` | owner / co-professor |
| PUT / DELETE | `/api/v1/offerings/{id}` | owner |
| POST | `/api/v1/offerings/{id}/enrollment-code/rotate` | owner |
| GET / POST | `/api/v1/offerings/{id}/members` | owner |
| PUT / DELETE | `/api/v1/offerings/{id}/members/{userId}` | owner |
| GET | `/api/v1/offerings/{id}/grades` | owner / co-professor |
| GET | `/api/v1/offerings/{id}/grades.csv` | owner / co-professor |
| GET | `/api/v1/offerings/{id}/export` | owner / co-professor |
| GET | `/api/v1/offerings/{id}/submissions/export` | owner / co-professor |
| POST | `/api/v1/offerings/{id}/submissions/recompile` | owner / co-professor |
| POST | `/api/v1/offerings/{id}/notify` | owner / co-professor |
| POST | `/api/v1/offerings/{offeringId}/exercises` | staff |
| GET | `/api/v1/offerings/{offeringId}/exercises` | member/staff |
| GET | `/api/v1/exercises/{id}` | member/staff |
| PUT / DELETE | `/api/v1/exercises/{id}` | staff |
| GET | `/api/v1/exercises/{id}/test-cases` | member/staff |
| POST | `/api/v1/exercises/{id}/test-cases` | staff |
| POST | `/api/v1/exercises/{id}/test-cases/batch` | staff |
| GET | `/api/v1/exercises/{id}/test-cases/export` | staff |
| PUT / DELETE | `/api/v1/exercises/{id}/test-cases/{caseId}` | staff |
| GET | `/api/v1/exercises/{id}/compilation-files` | member/staff |
| POST | `/api/v1/exercises/{id}/compilation-files` | staff |
| GET | `/api/v1/exercises/{id}/compilation-files/{fileId}` | member/staff |
| DELETE | `/api/v1/exercises/{id}/compilation-files/{fileId}` | staff |
| GET | `/api/v1/exercises/{id}/attached-files` | member/staff |
| POST | `/api/v1/exercises/{id}/attached-files` | staff |
| GET | `/api/v1/exercises/{id}/attached-files/{fileId}` | member/staff |
| DELETE | `/api/v1/exercises/{id}/attached-files/{fileId}` | staff |
| GET | `/api/v1/allowed-file-types` | signed in |

### Submissions

| Method | Path | Who |
| --- | --- | --- |
| POST | `/api/v1/submissions` | enrolled student or staff |
| GET | `/api/v1/submissions/{id}/events` | owner or staff of the exercise |
| GET | `/api/v1/submissions/{id}/source` | owner |
| GET | `/api/v1/submissions/{id}/output` | owner |
| GET | `/api/v1/exercises/{id}/submissions` | owner |

### Administration

| Method | Path | Who |
| --- | --- | --- |
| GET | `/api/v1/admin/users` | admin |
| PUT / DELETE | `/api/v1/admin/users/{id}` | admin |
| GET | `/api/v1/admin/offerings` | admin |
| PUT / DELETE | `/api/v1/admin/offerings/{id}` | admin |
| GET | `/api/v1/admin/offerings/{id}/members` | admin |
| GET / PUT | `/api/v1/admin/settings` | admin |
| GET | `/api/v1/admin/file-types` | admin |
| POST | `/api/v1/admin/file-types` | admin |
| PUT / DELETE | `/api/v1/admin/file-types/{id}` | admin |
| POST | `/api/v1/admin/preview` | admin |

The detailed pages document request bodies, responses and edge cases:

- [Users & accounts](./auth.md)
- [Classes & exercises](./classes.md)
- [Submissions & events](./submissions.md)
- [Administration](./admin.md)
- [Judge API](./judge.md)
