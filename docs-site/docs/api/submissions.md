---
title: Submissions & events API
description: Create submissions, consume the SSE stream, and download the source and the generated files.
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
  "number": 10,
  "status": "queued",
  "events_url": "/api/v1/submissions/42/events"
}
```

`number` is the caller's 1-based submission position for this exercise, not the
platform-wide commit id. The same user's tenth submission to another exercise,
and another user's tenth submission to this one, also report `10`.

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
2. checks whether the submission generated files and advertises them to the owner;
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
  "number": 10,
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
  "wall_time": 0.14,
  "mem_usage": 12582912,
  "user_output": "42\n",
  "user_output_type": "text",
  "status": "correct",
  "status_message": "",
  "error_message": ""
}
```

`artifacts` contains `{"kind":"output"}` when the submission generated at least
one file, plus one entry per case whose upload the judge reported. A per-case
entry carries `test_case_id`, `state` (`pending`, `ready`, `failed`) and, for a
failure, `reason` and optionally `limit_bytes`. The files themselves are listed
by `GET /submissions/{id}/outputfiles` or pushed over the output-events
WebSocket.

### Live events

| `event:` | Data fields |
| --- | --- |
| `status` | `type`, `commit_id`, `seq`, `status` (`compiling`/`running`), `at` |
| `compilation` | `type`, `commit_id`, `seq`, `compiled`, `message`, `error`, `at` |
| `case_result` | `type`, `commit_id`, `seq`, `test_case_id`, `cpu_time`, `wall_time`, `mem_usage`, `status`, `status_message`, `user_output`, `user_output_type`, `error_message` |
| `timings` | `type`, `commit_id`, `seq`, `phases[]`; phase names, durations and optional CPU/memory, persisted by the backend |
| `artifact` | `type`, `commit_id`, `seq`, `kind`, `url`; per-case: `test_case_id`, `state` (`pending`, `ready`, `failed`), `reason` and `limit_bytes` on a failure |
| `finished` | `type`, `commit_id`, `seq`, `status`, `num_correct_cases`, `score`, `compilation_message`, `compilation_error`, `started_at`, `finished_at` |
| `error` | `type`, `commit_id`, `seq`, `message`; the stream ends |

`cpu_time` is measured CPU time (container cgroup, with a direct-child rusage
fallback); `wall_time` is the command's wall clock. The exercise's
`cpu_time_limit_seconds` is a wall-clock timeout. `timings` is informational for
live clients; the product statistics read the persisted rows.

Example:

```text
event: case_result
id: 3
data: {"type":"case_result","commit_id":42,"seq":3,"test_case_id":91,"cpu_time":0.12,"wall_time":0.14,"mem_usage":12582912,"status":"correct","status_message":"","user_output":"42\n","user_output_type":"text","error_message":""}

event: finished
id: 5
data: {"type":"finished","commit_id":42,"seq":5,"status":"completed","num_correct_cases":5,"score":100.00,"compilation_message":"","compilation_error":"","started_at":"…","finished_at":"…"}
```

`mem_usage` is the peak memory sampled from the container cgroup, in bytes, or
`-1` when the host exposes no readable memory controller. The monitor samples
every 10 ms; it is a comparison signal, not pure RSS.

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

## `GET /submissions/{id}/outputfiles`

Lists the files the submission generated. Owner only.

```json
[
  {"path": "test_1/result.txt", "size": 12},
  {"path": "test_1/nested/saida.csv", "size": 2140}
]
```

- The list is empty when the submission wrote no file.
- The judge uploads each case's files as soon as that case finishes, so the list
  can grow while the run is still in progress.
- `path` locates the file inside the submission's output namespace:
  `test_<case number>/<relative>` for a file a case produced,
  `outputfiles/<relative>` for one written straight into the runner's output
  directory. The number is the case's 1-based position in the exercise, matching
  the results panel and the downloaded archive.
- Internal reports (`compilation.*`, per-case `.output`, `.error`,
  `.monitor_out`, `.monitor_err`) are excluded.
- A case whose program output the author did not release (`Mostrar saída do
  aluno` off) hides the files it produced: they are omitted from the listing and
  answer `404` individually, so the API never discloses that the case wrote them.
- A case interrupted by a limit or a signal publishes nothing: its program was
  killed mid-write, so the collected bytes are a truncated file rather than a
  result. Its files are excluded the same way.
- Files are kept for `RUNCODES_OUTPUT_FILE_TTL` (default 168h) after the judge
  uploads them; expired files disappear from the listing.
- `GET /submissions/{id}/output` refuses with `413` when the visible files
  total more than `RUNCODES_OUTPUT_ARCHIVE_MAX_BYTES` (default 256 MiB); the
  per-file endpoint has no such cap.

## `GET /submissions/{id}/output-events` (WebSocket)

Pushes the generated-file listing and the per-case upload states. Owner only;
the handshake authenticates with the same session cookie as the SSE stream.

Each text frame is a JSON object:

```json
{
  "type": "outputfiles",
  "files": [{"path": "test_1/result.txt", "size": 12}],
  "artifacts": [
    {"kind": "output", "test_case_id": 91, "state": "failed",
     "reason": "commit_artifact_budget_exceeded", "limit_bytes": 268435456}
  ],
  "complete": false
}
```

- The first frame is the current state.
- The server sends another frame whenever an `artifact` event changes the state.
- The last frame has `complete: true`; the server then closes the socket.
- When the run ends, any still-pending upload is reported as `failed` with
  reason `upload_interrupted`, so a client never keeps a spinner that can no
  longer settle.
- `GET /submissions/{id}/outputfiles` remains available as a one-shot fallback;
  it returns the same `files` array.

The frontend uses this socket for the results panel's generated-files section
instead of polling the listing.

## `GET /submissions/{id}/outputfiles/{path}`

Streams one generated file. Owner only.

- `path` must be one returned by the listing, as a clean relative path: `..`
  segments and absolute paths answer `400`.
- The response is always an opaque download (`application/octet-stream` with
  `Content-Disposition: attachment`), so generated HTML or SVG is never rendered
  by the API's origin.
- An unknown file answers `404`.

```bash
curl -b cookies.txt -OJ \
  http://localhost:8080/api/v1/submissions/42/outputfiles/test_1/result.txt
```

## `GET /submissions/{id}/output`

Streams a zip with every generated file. Owner only.

- The archive is built on the fly from object storage and keeps the same paths
  the listing reports; the files are not stored zipped.
- A submission that generated nothing answers `404`.

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
    "number": 10,
    "status": "completed",
    "num_correct_cases": 5,
    "score": 100,
    "created_at": "2026-03-10T14:22:00Z",
    "filename": "main.c"
  }
]
```

`filename` is derived from the S3 key. `number` is the 1-based position among
this user's submissions to this exercise, ordered by submission time; `id` is
the platform-wide commit id used by the routes. The list never includes another
user's submissions.

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

The generated-file section uses the output WebSocket:

```js
const socket = new WebSocket(
  `${location.origin.replace(/^http/, 'ws')}/api/v1/submissions/${id}/output-events`,
)
socket.onmessage = (message) => {
  const snapshot = JSON.parse(message.data)
  if (snapshot.type !== 'outputfiles') return
  renderFiles(snapshot.files, snapshot.artifacts)
  if (snapshot.complete) socket.close()
}
```

The browser sends the session cookie with the handshake. No polling is needed:
the socket sends the current listing immediately and another frame for every
artifact event, then a final `complete: true` frame.

## Related pages

- [Submissions & live judging](../guide/submissions.md)
- [Judge API](./judge.md)
- [Judge service](../services/judge.md)
