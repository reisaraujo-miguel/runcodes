---
title: Glossary
description: Definitions of the domain and infrastructure terms used throughout the documentation.
---

# Glossary

## Domain

**Offering (class)**
A course instance that groups students, exercises and staff. Stored in
`offerings`; exposed as `offering` in the API and "class" in the UI.

**Enrollment**
A membership row linking a user to an offering with a role (`student`,
`monitor`, `professor`) and an optional ban flag.

**Owner**
The professor (or admin) in `offerings.owner_id`. The owner manages members,
edits and deletes the class, and has every class permission.

**Co-professor**
An enrolled member with the `professor` role. Authors exercises, test cases and
files; cannot manage members.

**Monitor**
An enrolled member who can read the class and exercises and submit, but cannot
author. A legacy teaching-assistant role retained in the schema.

**Exercise**
A programming task inside a class, with an open date, deadline, allowed file
types, optional attached/compilation files and test cases.

**Test case**
One input/expected-output pair plus per-case CPU, memory, stack and file-size
limits and visibility flags.

**Allowed file type**
An extension the exercise accepts for submissions. The judge maps the extension
to a runner image.

**Commit / submission**
A row in `commits` representing one submitted solution and its lifecycle,
status, score and per-case results. The API exposes `commit_id`; the UI calls it
a submission.

**Case result**
One row in `commits_exercise_test_cases_results`, with a verdict (`correct`,
`bad_formatted_output`, `killed_with_signal`), CPU time, output and diagnostics.

**Score**
A percentage in `[0, 100]` computed from the correct test cases.

**Snapshot**
The persisted state (commit, results, artifacts) sent at the start of an SSE
subscription, so a reload can restore the UI.

**Artifact**
A generated output archive. `artifact` events and the snapshot advertise it; the
file itself is served by the judge and proxied by the backend.

## Execution

**Judge**
The compute-only Go service that claims queued commits, runs containers, grades
outputs and streams events. It never writes persistence.

**Runner (image)**
A language-specific container image that compiles and runs a submission. It
contains the harness, the monitor and a toolchain.

**Monitor**
The C program inside a runner that runs one command under time, memory, stack and
file-size limits and reports `exit_status`, `signal` and `time`.

**Harness**
`runners/base/base-script.sh` plus a language script, installed as
`/usr/bin/runcodes`. It reads `container.config`, emits milestones, runs the
compiler and test cases through the monitor, and cleans up processes.

**Workspace**
The per-commit directory under `JUDGE_EXEC_DIR`, bind-mounted at `/root` in the
runner container. It holds the source, inputs, outputs and reports.

**`container.config`**
The file the judge writes into the workspace. It carries the run nonce, source
filename, per-case limits and optional global overrides. The harness deletes it
before submitted code runs.

**Run nonce**
A random token in `container.config`. Every milestone must carry it, so a
submission cannot fake progress on the container log stream.

**Milestone**
A log line of the form `<name> <run_nonce>` (`compilation.start`,
`compilation.done`, `run.start`, `run.done`) that the judge uses to track the
run.

**Rootless Podman**
The container runtime used by the judge and the four-machine deployment. It runs
without a root daemon; container root maps to the unprivileged deployment user.

**Subuid / subgid**
The range rootless Podman uses to map container uids to host uids. The judge does
not depend on a specific container uid, only on a non-root user with
`$HOME=/root`.

**Cgroup cap**
The judge-side limits (`JUDGE_CONTAINER_*`) applied by the runtime, independent
of the in-container monitor.

## Data and operations

**SeaweedFS**
The S3-compatible object store holding submissions, test cases, exercise files
and output metadata.

**`runcodes_app`**
The limited PostgreSQL role the backend uses. It can read and write its rows but
cannot change the schema or read `pg_authid`.

**`runcodes_judge`**
The narrower PostgreSQL role the judge uses for the claim loop and grading
tables; it has the same schema and `pg_authid` restrictions as `runcodes_app`.

**Atlas**
The migration tool pinned by `database/atlas.sh`. It diffs the desired schema and
applies versioned SQL migrations.

**Baseline / stamp**
The first migration version. An existing database is *stamped* at a version to
record that its schema already matches, without executing those migrations.

**SSE (Server-Sent Events)**
The one-way HTTP stream used for live submission events and the judge's internal
event log. Heartbeats are `: ping` comments.

**Snapshot event**
See *Snapshot*.

**Reconciliation sweeper**
The backend background job that marks commits stuck in `compiling`/`running`
longer than `RUNCODES_JUDGE_STALE_TIMEOUT` as `server_error`.

**Readiness (`/readyz`)**
Backend: database required, judge informational. Judge: database and Podman
required. Used before accepting submissions.

**Trace / span**
OpenTelemetry observability primitives. A trace follows one request; a span is
one operation within it.

**Profile**
A continuous CPU profile scraped from pprof by Alloy and stored in Pyroscope.
Spans can link to the profile that covers them.

**GHCR**
GitHub Container Registry, where the application, monitor and runner images are
published.

**Four-machine deployment**
The Ansible reference topology: `edge`, `data`, `compute`, `observability`.

**Edge / data / compute / observability**
The four inventory groups: public entry, stateful services, untrusted execution
and telemetry storage.

## Related pages

- [Architecture](../guide/architecture.md)
- [Judge](../services/judge.md)
- [Runner images & monitor](../services/runners-and-monitor.md)
