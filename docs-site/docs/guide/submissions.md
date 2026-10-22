---
title: Submissions & live judging
description: Submission flow, statuses, Server-Sent Events, artifacts and failure modes.
---

# Submissions & live judging

A submission is represented by a **commit** row. The backend creates it, the
judge claims and executes it, and the browser follows it over Server-Sent Events
(SSE). This page documents the lifecycle from the user's point of view; the
service-level contract is in [Judge](../services/judge.md).

## Submitting

`POST /api/v1/submissions` accepts a multipart form:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `exercise_id` | integer | yes | must be open and belong to a class the caller can access |
| `file` | file | yes | at most 10 MiB; extension must be allowed by the exercise |

The source is uploaded to SeaweedFS before the commit is registered. A zip
submission is unpacked by the judge under per-entry and total expanded-size
limits.

A successful request returns `201`:

```json
{
  "commit_id": 42,
  "status": "queued",
  "events_url": "/api/v1/submissions/42/events"
}
```

The API refuses the request before creating anything in these cases:

| Status | Meaning |
| --- | --- |
| `400` | invalid form, missing file, or a file type not allowed by the exercise |
| `403` | the caller is not enrolled in the exercise's class |
| `404` | the exercise does not exist or is not visible to the caller |
| `413` | the file exceeds 10 MiB |
| `422` | the deadline has passed for the caller |
| `503` | the judge or S3 is unavailable |

Staff (class owner or co-professor) can submit to their own exercises without
being enrolled and after the deadline: the deadline is a rule for graded
students.

## Following a submission

`GET /api/v1/submissions/{id}/events` is an SSE stream. It requires the commit
owner, a professor or an admin. The stream always starts with a **snapshot**
event containing the persisted state:

```text
event: snapshot
id: 0
data: {"type":"snapshot","commit":{...},"results":[...],"artifacts":[...]}
```

Then it relays judge events for the live run. Heartbeat comments (`: ping`) are
sent every 15 seconds. If the commit is already in a terminal state, the stream
ends after the snapshot.

```bash
curl -N -H "Authorization: Bearer $TOKEN" \
  http://localhost:8443/api/v1/submissions/42/events
```

The frontend's submission page also uses this endpoint directly. A browser
session authenticates with the `jwt` HttpOnly cookie, so no token needs to be
handled by JavaScript.

### Event types

| Event | When | Key data |
| --- | --- | --- |
| `snapshot` | immediately | persisted commit, case results, artifacts |
| `status` | judge claims or starts the run | `status`: `compiling` or `running` |
| `compilation` | compilation ends | `compiled`, `message`, `error` |
| `case_result` | each test case ends | `test_case_id`, `status`, `cpu_time`, `user_output`, diagnostics |
| `artifact` | an output archive exists | `kind: "output"` |
| `finished` | run finishes | terminal `status`, `num_correct_cases`, `score`, timestamps |
| `error` | unexpected stream-side failure | `message`; the stream ends |

Case result statuses:

| Status | Meaning |
| --- | --- |
| `correct` | output matched the expected output |
| `bad_formatted_output` | the program ran but produced the wrong output |
| `killed_with_signal` | the process was killed (crash, signal, or a limit enforced by the monitor) |

Terminal commit statuses:

| Status | Meaning |
| --- | --- |
| `completed` | all test cases were correct |
| `uncompleted` | the run finished with one or more failed cases |
| `compilation_error` | the compiler rejected the submission |
| `server_error` | an infrastructure or harness failure |
| `timeout` | the wall-clock budget for the commit was exceeded |

The `score` is a percentage in `[0, 100]`.

## Reloading and replay

The browser does not need to keep the connection open to see the final result.
Every event is persisted before it is relayed, so reconnecting returns the
snapshot of the persisted state. The live relay is a convenience on top of that;
the database remains the source of truth.

The judge keeps a bounded in-memory event log for `JUDGE_EVENT_RETENTION`
(default 10 minutes) after a run finishes, allowing the backend to resume a
subscription with `?from=<seq>`. After that, the backend's persisted state is
the only source.

## History, source and output files

| Endpoint | Purpose |
| --- | --- |
| `GET /api/v1/exercises/{id}/submissions` | the caller's last 50 submissions for an exercise, newest first |
| `GET /api/v1/submissions/{id}/source` | download the original source; restores it into the editor |
| `GET /api/v1/submissions/{id}/output` | download the archive of files the submission produced |

All three only return the caller's own commits. A missing commit and another
user's commit both answer `404`, so the API never discloses that a commit exists.
The output archive is generated only when the submission wrote at least one
output file; internal monitor and per-case reports are excluded. The backend
proxies the archive from the judge, so the browser never needs the judge's
internal address.

## Failure handling

| Situation | What happens |
| --- | --- |
| Judge is down when submitting | `503`; no commit row and no uploaded object are kept |
| Judge crashes mid-run | the row stays `compiling`/`running`; the backend's stale sweeper marks it `server_error` after `RUNCODES_JUDGE_STALE_TIMEOUT` |
| Wake call fails after insert | the transaction is rolled back and the uploaded source is deleted |
| Compilation exceeds the image's timeout | judge reports `compilation_error` (or a milestone timeout if `JUDGE_COMPILATION_WAIT` is too short) |
| Run exceeds `JUDGE_MAX_RUN_DURATION` | `timeout` |
| Output exceeds the comparison cap | the case fails rather than being truncated |
| Submission left a daemon running | the harness kills the container's remaining processes after each case and after compilation |

The reconciliation sweeper is the safety net for judge failures; it never
interferes with a live run. See [Judge](../services/judge.md#failure-handling)
for the service side.

## Observability

Each claimed commit starts a `judge.process_commit` root span, with
`judge.prepare_workspace`, `judge.run_container` and one `judge.grade_test_case`
span per test case. The judge exports `runcodes.judge.queue.depth` and
`runcodes.judge.queue.oldest_age` to expose backlog. In Grafana, a submission
trace can be followed from the browser fetch through the backend to the judge
and its per-case spans. See [Observability](../observability/index.md).
