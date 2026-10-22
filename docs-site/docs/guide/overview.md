---
title: What is RunCodes?
description: Overview of the RunCodes platform, its services, technology stack and repository layout.
---

# What is RunCodes?

RunCodes is an open-source platform for programming exercises. Students write
code in the browser, submit it, and the platform compiles and runs it against an
exercise's test cases inside isolated containers. Per-test-case verdicts and the
final score stream back to the browser while the run is happening.

It is the next-generation rewrite of the legacy **run.codes** platform. The
original compiler engine and language images lived in separate repositories
(`compiler-engine`, `compiler-images`); they were folded into this repository as
[`judge/`](https://github.com/reisaraujo-miguel/runcodes/tree/main/judge),
[`runners/`](https://github.com/reisaraujo-miguel/runcodes/tree/main/runners) and
[`monitor/`](https://github.com/reisaraujo-miguel/runcodes/tree/main/monitor).

## What it does

- **Courses and classes.** Professors create offerings (classes), enroll
  students with a code, and add co-professors and monitors.
- **Exercises and test cases.** Each exercise has a deadline, allowed file
  types, optional compilation files and attached files, and one or more test
  cases with per-case CPU, memory, stack and file-size limits.
- **Submissions.** A student uploads a source file (or a zip) and receives a
  commit id immediately. The judge compiles and runs it, and the browser follows
  the run over Server-Sent Events.
- **Grading.** Every test case produces a verdict (`correct`,
  `bad_formatted_output`, or `killed_with_signal`), CPU time, output and
  diagnostics. The commit receives a score and a terminal status.
- **Authoring and administration.** Professors manage their classes and
  exercises; admins manage users, any class, and platform settings such as the
  contact address shown on the login page.
- **Optional observability.** Traces, metrics, logs, continuous CPU profiles and
  alerts can be enabled with an opt-in OpenTelemetry stack.

## Services

| Service     | Path        | Stack                                     | Responsibility                                     |
| ----------- | ----------- | ----------------------------------------- | -------------------------------------------------- |
| Frontend    | `frontend/` | React 19, Vite, TypeScript, Tailwind, Bun | The web client: auth, classes, exercises, editor.  |
| Backend API | `backend/`  | Go, chi, PostgreSQL, JWT, S3              | Auth, classes, submissions, SSE relay.              |
| Judge       | `judge/`    | Go, rootless Podman, PostgreSQL, S3       | Claims queued commits, executes and grades them.    |
| Runners     | `runners/`  | Container image per language              | The image a submission is compiled and run in.      |
| Monitor     | `monitor/`  | C                                         | Enforces limits and reports runtime for one process. |
| Database    | `database/` | PostgreSQL                                | Schema, seeds, migrations and legacy import.        |

The frontend is served by Caddy, which also reverse-proxies `/api/*` to the
backend. Only the frontend is published on a network interface in the local
Compose stack; the backend, judge, SeaweedFS and the mail catcher are bound to
loopback.

## Technology at a glance

| Layer          | Choices                                                                 |
| -------------- | ----------------------------------------------------------------------- |
| Web client     | React 19, React Router, TypeScript, Vite (Rolldown), Tailwind CSS 4      |
| UI             | shadcn/ui-style components, Radix/Base UI primitives, CodeMirror editor |
| API            | Go 1.26+, chi, `jwtauth`, `lib/pq`                                       |
| Queue          | PostgreSQL (`commits` table, `FOR UPDATE SKIP LOCKED`)                  |
| Object storage | SeaweedFS, S3-compatible API                                            |
| Cache          | Redis (optional; the API degrades to a no-op when unreachable)          |
| Execution      | Rootless Podman + one container image per language                     |
| Telemetry      | OpenTelemetry Go/Web SDK, Collector, Prometheus, Loki, Tempo, Pyroscope, Grafana, Alertmanager |
| Deployment     | Docker Compose (single host) or Ansible + rootless Podman (four hosts)  |

## Supported languages

The judge maps a submission's file extension to one of 22 language images:
C, C++, C#, Fortran, Go, Haskell, Java, Octave, Pascal, Portugol, Python, R,
Rust, Lua, Julia, Prolog, C/C++ with OpenMP, C/C++ with OpenMP and MPI, Verilog
and Zig. See [Languages & images](../reference/languages.md) for the full
extension and image matrix.

## Repository layout

```text
.
├── backend/          Go API (auth, offerings, submissions, SSE relay)
├── frontend/         React client served by Caddy
├── judge/            Execution engine (rootless Podman)
├── runners/          Language images the judge runs submissions in
├── monitor/          In-container process monitor (limits, timing, report)
├── database/         PostgreSQL schema, seeds, migrations and legacy import
├── observability/    Collector, Prometheus, Loki, Tempo, Grafana, Pyroscope config
├── docs/             Pointer to this site (old guides consolidated)
├── docs-site/        This documentation website (VitePress)
├── scripts/          Developer and observability validation scripts
├── deploy/ansible/   Four-machine rootless Podman deployment
├── docker-compose.yml
└── docker-compose.observability.yml
```

## Design principles

The platform is built around a few boundaries that explain most of its design:

1. **Persistence and execution are separate.** The backend owns every database
   write and every object-store upload. The judge is compute-only: it claims a
   commit, executes it, and streams events. It never writes results.
2. **PostgreSQL is the queue.** A submission is durable before any worker sees
   it. Claiming uses `FOR UPDATE SKIP LOCKED`, which is crash-safe and safe
   across multiple judge instances.
3. **Untrusted code is isolated outside the container too.** Graded runs are
   capped by cgroups, have no network namespace, cannot gain privileges, and see
   only their workspace. Limits are also enforced from inside by the monitor,
   but the judge does not rely on them.
4. **The browser is a view, not the source of truth.** Events are persisted
   before they are relayed. A reload reconnects and receives a snapshot, so the
   UI can never end up in a state the database does not agree with.
5. **Observability is opt-in.** Both Go services are instrumented, but export
   nothing unless `OTEL_EXPORTER_OTLP_ENDPOINT` is set. The normal `docker
   compose up` has no telemetry dependency.

## Where to go next

- [Architecture](./architecture.md) — the full component and data-flow picture.
- [Getting started](./getting-started.md) — run the stack locally.
- [Using RunCodes](./using-runcodes.md) — student, professor and admin workflows.
- [Deployment](../deployment/index.md) — put it on real hosts.
- [API reference](../api/index.md) — integrate with or test the platform.
- [Documentation map](../reference/source-map.md) — the canonical pages and the
  repository entry points that point to them.

## License and community

RunCodes is distributed under the **GNU Affero General Public License v3.0**.
See [LICENSE](https://github.com/reisaraujo-miguel/runcodes/blob/main/LICENSE).
Contributions are welcome; read [Contributing](../contributing.md) first.
