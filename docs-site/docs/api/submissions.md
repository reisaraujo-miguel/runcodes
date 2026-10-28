---
title: Submissions & events API
description: Create submissions, consume the SSE stream, and download source and output archives.
---

# Submissions & events API

Paths are relative to `/api/v1`. The user-facing lifecycle is described in
[Submissions & live judging](../guide/submissions.md); this page is the wire
contract.

## `POST /submissions`

`multipart/form-data`:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `exercise_id` | integer | yes | must be visible to the caller and before its deadline (students) |
| `file` | file | yes | ≤ 10 MiB; extension must be allowed by the exercise |

```bash
curl -b cookies.txt -X POST http://localhost:8080/api/v1/submissions \
  -F exercise_id=15 \
  -F file=@main.c
```

Response `201`:

```json
{
  "commit_id": 42,
  "status": "queued",
  "events_url": "/api/v1/submissions/42/events"
}
```

| Status | Meaning |
| --- | --- |
| `201` | queued |
| `400` | malformed form, missing file, or file type not allowed |
| `401` | no session |
| `403` | not enrolled in the class |
| `404` | exercise not found or not visible |
| `413` | file exceeds 10 MiB |
| `422` | deadline passed, or the exercise has no test cases |
| `429` | the caller already has two unfinished submissions |
| `500` | server error |
| `503` | judge or object storage unavailable |

The backend checks `/readyz` on the judge **before** uploading or inserting
anything. If the judge is unreachable, no row is created and the API answers
`503`.

## `GET /submissions/{id}/events`

Server-Sent Events. Requires the commit owner or a member of staff who teaches
the commit's exercise (class owner or co-professor). A professor who teaches
another class receives `403`.

```bash
curl -N -b cookies.txt \
  http://localhost:8080/api/v1/submissions/42/events
```

Response headers:

```http
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
X-Accel-Buffering: no
```

The stream:

1. sends a `snapshot` event with `id: 0`;
2. checks whether an output artifact exists and advertises it to the owner;
3. ends if the commit is already terminal;
4. otherwise relays live judge events and sends `: ping` every 15 seconds;
5. ends on the terminal `finished`/`error` event or when the client disconnects.

### Snapshot

```text
event: snapshot
id: 0
data: {"type":"snapshot","commit":{...},"results":[...],"artifacts":[...]}
```

`commit` is the persisted commit:

```json
{
  "id": 42,
  "user_id": 7,
  "exercise_id": 15,
  "status": "running",
  "num_correct_cases": 3,
  "score": 60.00,
  "compiled": true,
  "compilation_message": "",
  "compilation_error": "",
  "compilation_started": "2026-03-10T14:22:01Z",
  "compilation_finished": "2026-03-10T14:22:04Z",
  "created_at": "2026-03-10T14:22:00Z",
  "s3_key": "commits/…",
  "ip": "203.0.113.5"
}
```

`results` are persisted case results:

```json
{
  "exercise_test_case_id": 91,
  "cpu_time": 0.12,
  "mem_usage": -1,
  "user_output": "42\n",
  "user_output_type": "text",
  "status": "correct",
  "status_message": "",
  "error_message": ""
}
```

`artifacts` contains `{"kind":"output"}` when an output archive exists.

### Live events

| `event:` | Data fields |
| --- | --- |
| `status` | `type`, `commit_id`, `seq`, `status` (`compiling`/`running`), `at` |
| `compilation` | `type`, `commit_id`, `seq`, `compiled`, `message`, `error`, `at` |
| `case_result` | `type`, `commit_id`, `seq`, `test_case_id`, `cpu_time`, `mem_usage`, `status`, `status_message`, `user_output`, `user_output_type`, `error_message` |
| `artifact` | `type`, `commit_id`, `seq`, `kind`, `url` |
| `finished` | `type`, `commit_id`, `seq`, `status`, `num_correct_cases`, `score`, `compilation_message`, `compilation_error`, `started_at`, `finished_at` |
| `error` | `type`, `commit_id`, `seq`, `message`; the stream ends |

`cpu_time` is measured CPU time (container cgroup, with a direct-child rusage
fallback); the exercise's `cpu_time_limit_seconds` is a wall-clock timeout.

Example:

```text
event: case_result
id: 3
data: {"type":"case_result","commit_id":42,"seq":3,"test_case_id":91,"cpu_time":0.12,"mem_usage":-1,"status":"correct","status_message":"","user_output":"42\n","user_output_type":"text","error_message":""}

event: finished
id: 5
data: {"type":"finished","commit_id":42,"seq":5,"status":"completed","num_correct_cases":5,"score":100.00,"compilation_message":"","compilation_error":"","started_at":"…","finished_at":"…"}
```

`mem_usage` is `-1` (unknown) because the monitor is not the submission's parent
and cannot measure it reliably.

### Event ordering

- `seq` is per-commit and monotonically increasing.
- Persisted events are forwarded in order; a reconnect starts from a fresh
  snapshot rather than replaying from an arbitrary point.
- The judge's own event log supports `?from=<seq>` for the backend's internal
  subscription. The public backend endpoint does not expose `from`.

## `GET /submissions/{id}/source`

Streams the original submitted file with
`Content-Disposition: attachment`. Owner only. A missing source object or a
commit belonging to another user both answer `404`.

```bash
curl -b cookies.txt -OJ \
  http://localhost:8080/api/v1/submissions/42/source
```

The editor uses this endpoint to restore a previous submission.

## `GET /submissions/{id}/output`

Streams the output archive generated by the judge. Owner only.

- The archive exists only when the submission wrote at least one output file.
- Archives are kept for `JUDGE_ARTIFACT_TTL` (default 168h); after the judge's
  sweep removes one, the endpoint answers `404`.
- Internal reports (`compilation.*`, per-case `.output`, `.error`,
  `.monitor_out`, `.monitor_err`) are excluded.
- A missing archive answers `404`.
- The backend proxies the archive from the judge; the browser never needs the
  judge's address.

```bash
curl -b cookies.txt -OJ \
  http://localhost:8080/api/v1/submissions/42/output
```

## `GET /exercises/{id}/submissions`

Returns the caller's last 50 submissions for an exercise, newest first.

```json
[
  {
    "id": 42,
    "status": "completed",
    "num_correct_cases": 5,
    "score": 100,
    "created_at": "2026-03-10T14:22:00Z",
    "filename": "main.c"
  }
]
```

`filename` is derived from the S3 key. The list never includes another user's
submissions.

## Streaming from JavaScript

```js
const response = await fetch(`/api/v1/submissions/${id}/events`, {
  credentials: 'include',
  headers: { Accept: 'text/event-stream' }
})

const reader = response.body.getReader()
const decoder = new TextDecoder()
let buffer = ''

while (true) {
  const { value, done } = await reader.read()
  if (done) break
  buffer += decoder.decode(value, { stream: true })

  // Split complete SSE frames on a blank line, then parse `event:`/`data:`.
  const frames = buffer.split('\n\n')
  buffer = frames.pop() ?? ''
  for (const frame of frames) handleFrame(frame)
}
```

The frontend uses an equivalent parser and reduces the events into the
submission view. The initial snapshot is enough to restore the page after a
reload; the live events update it as the run progresses.

## Related pages

- [Submissions & live judging](../guide/submissions.md)
- [Judge API](./judge.md)
- [Judge service](../services/judge.md)
