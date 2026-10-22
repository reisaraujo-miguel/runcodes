# PR: observability next steps — browser traces, span profiles and submission checks

## Branch

`feat/observability-next` (on top of `feat/observability-stack`).

## Summary

This follow-up closes the remaining observability gaps without changing the
application's normal startup path:

1. **Browser tracing** with an opt-in OpenTelemetry Web SDK, lazy-loaded into a
   separate bundle, collector OTLP/HTTP CORS, and W3C propagation into the
   backend and judge.
2. **A real browser test** with Playwright/Chromium that loads the SPA, issues a
   fetch, extracts the injected `traceparent`, and asserts the same trace
   contains frontend and backend spans in Tempo.
3. **Continuous profiling** with Pyroscope + Alloy, internal pprof listeners,
   seven-day profile retention, an S3 configuration example, and the Grafana
   Pyroscope datasource.
4. **Span-to-profile correlation** with `otel-profiling-go`: spans carry
   `pyroscope.profile.id`; pprof samples carry trace/span labels; the Tempo
   datasource opens the matching profile.
5. **Judge visibility**: async queue-depth/oldest-age gauges and
   `judge.prepare_workspace`, `judge.run_container` and per-test-case
   `judge.grade_test_case` spans.
6. **An authenticated submission test**: logs in as an admin, creates a
   disposable offering/exercise/test case, submits a Python solution, waits for
   the judge, and verifies the per-case span, its verdict, and the profile id.
7. **Secret-backed alert delivery**: the Alertmanager webhook URL comes from a
   mounted, gitignored secret file; a production receiver template covers
   Slack, PagerDuty and generic webhooks.
8. **Retention/tuning**: Pyroscope keeps 7 days, and Prometheus retention is
   configurable through environment variables.

## What changed

### Browser tracing and Playwright

- Added pinned OTel Web packages (`api@1.9.0`, `sdk-trace-web@2.12.0`,
  `exporter-trace-otlp-http@0.223.0`, `instrumentation-fetch@0.223.0`, etc.).
- `frontend/src/lib/telemetry/config.ts`: pure, tested configuration for
  `VITE_OTEL_ENABLED`, endpoint normalization, sampling ratio and API origin.
- `frontend/src/lib/telemetry/index.ts`: `WebTracerProvider`,
  `BatchSpanProcessor`, `FetchInstrumentation`, W3C propagation, ignored
  collector URL and `browser.url.path` attribute.
- `frontend/src/main.tsx`: lazy-loads the telemetry chunk only when enabled and
  awaits it before React mounts; failures never block the app.
- `frontend/Dockerfile` and the Compose overlay bake the telemetry build args.
- Collector OTLP/HTTP CORS allows `localhost`/`127.0.0.1` on ports 8080/5173.
- `frontend/playwright.config.ts` and
  `frontend/e2e/browser-telemetry.pw.ts` run Chromium headless, capture the
  browser request, and verify the trace in Tempo.
- Added `make observability-browser-smoke-test` and a `test:e2e:install`
  script. The test is excluded from Bun's unit-test discovery by its `.pw.ts`
  suffix.

### Profiling and span correlation

- `backend/telemetry/pprof.go` and `judge/internal/telemetry/pprof.go` start an
  internal pprof listener only when `PPROF_ADDR` is set; it is never published.
- Added Pyroscope 2.4.1 and Alloy 1.21.1 to the Compose overlay. Alloy scrapes
  `backend:6060` and `judge:6060` and writes to Pyroscope.
- Wrapped the Go tracer providers with `otel-profiling-go` v0.7.0. The judge
  uses `ScopeAllSpans` so per-test-case spans get their own
  `pyroscope.profile.id`; the backend keeps root-span scope to limit
  cardinality.
- Grafana provisions the Pyroscope datasource and the Tempo
  `tracesToProfiles` link (`service.name` -> `service_name`, CPU profile type).
- Pyroscope retains 7 days on the local filesystem
  (`limits.retention_period: 168h`); `config.s3.example.yaml` documents object
  storage for production, selectable with `PYROSCOPE_CONFIG_FILE`.
- `PROMETHEUS_RETENTION_TIME`/`PROMETHEUS_RETENTION_SIZE` replace the hardcoded
  Prometheus flags (defaults `15d`/`8GB`).

### Judge queue and per-test-case spans

- `Store.QueueStats` and async gauges `runcodes.judge.queue.depth` /
  `oldest_age`, with a 3s query timeout.
- Added `RunCodesJudgeQueueDepthHigh` and `RunCodesJudgeQueueStalled`.
- Dashboard queue panels and `judge.prepare_workspace` / `judge.run_container`
  / `judge.grade_test_case` spans with commit, exercise, case id, verdict, CPU
  time and memory usage.
- `make observability-submission-smoke-test` creates a temporary authenticated
  submission, waits for completion, finds the judge trace by commit id, and
  asserts the per-case span and profile id. It deletes the temporary offering on
  success unless `SMOKE_KEEP=1`.

### Alert delivery

- Alertmanager now reads its webhook URL from
  `.observability/alertmanager_webhook_url` mounted as `url_file`.
- Added `alertmanager.production.example.yml` with Slack/PagerDuty/webhook
  receivers using `*_file` credentials.
- Documented webhook rotation; the regular smoke test still proves delivery
  through `alert-sink`.

### Documentation and validation

- Updated the observability README, runbook, root README, service READMEs and
  environment examples.
- `validate.sh` checks shell scripts, JSON, compose, Prometheus rules,
  Alertmanager and the collector.
- CI path filters include the new browser test and profiling files.

## How to run

```bash
make observability-init
make observability-up
make observability-smoke-test
make observability-validate

# Real browser trace; install Chromium once:
(cd frontend && bun run test:e2e:install)
make observability-browser-smoke-test

# Authenticated submission and per-case span:
SMOKE_ADMIN_PASSWORD='...' make observability-submission-smoke-test
```

The submission test needs the seeded admin password set with
`database/bootstrap-admin.sh`; it never uses a pre-existing exercise or
submission and deletes its temporary offering on success.

## Validation

| Check | Result |
| --- | --- |
| Backend `gofmt -l .` / `go test -count=1 ./...` | clean / all packages pass |
| Judge `gofmt -l .` / `go test -tags ... -count=1 ./...` | clean / all packages pass |
| Frontend `bun test` | **137 pass, 0 fail** |
| Frontend `bun run lint` | pass |
| Frontend `bun run build` | pass; telemetry remains a separate ~83 kB chunk |
| `make observability-validate` | pass: compose, 16 Prometheus rules, amtool, collector, JSON/shell |
| Clean `make observability-down` + `make observability-up` | pass |
| `make observability-smoke-test` | pass, all checks, 33s after the clean restart |
| Playwright browser test | **1 passed**; browser trace contains frontend + backend spans |
| Authenticated submission test | **pass**: temporary commit 28, per-case span `correct`, `pyroscope.profile.id` present |
| Pyroscope retention | config reports `retention_period: 1w` |
| Prometheus retention flags | `15d` / `8GB` by default, env-overridable |
| Alertmanager delivery | file-backed webhook delivers to `alert-sink` |

The regular smoke test checks: every service, backend/judge metrics, the judge
queue metric, profile ingestion, Loki logs + exact trace correlation, Tempo,
browser OTLP CORS + a synthetic frontend trace, Grafana datasources (including
Pyroscope), the dashboard, the Grafana alert/contact point, and Alertmanager
delivery.

## Known limitations

- The browser test is opt-in and needs Playwright Chromium plus a running stack;
  it is not part of the Docker smoke test because the collector and frontend must
  be reachable from the browser.
- The submission test needs an admin password and a working judge/Podman
  environment. It creates a temporary class and deletes it on success.
- Span profiles are CPU-only. A span shorter than the profiler sample interval
  can have a valid `pyroscope.profile.id` but no samples, so the flame graph can
  be empty. Goroutine/heap profiles remain service-level.
- The S3 Pyroscope config is a documented example, not exercised in this
  repository because there is no object store.
- The overlay is still single-host, without HA or TLS; production deployment
  needs persistent storage, an authenticating reverse proxy and managed or
  distributed backends.
- The local Alertmanager sink remains a test fixture; the receiver URL is
  file-backed so a real integration can be swapped in without editing YAML.

## Rollback

- `make observability-down` keeps volumes; `make observability-destroy` removes
  only observability volumes.
- Disable browser tracing with `FRONTEND_OTEL_ENABLED=false` (or
  `VITE_OTEL_ENABLED=false` for local dev).
- Leave `PPROF_ADDR` empty to disable pprof; the profile bridge is then inert.
- Point `.observability/alertmanager_webhook_url` back at `alert-sink` and
  recreate Alertmanager.
- Revert Prometheus retention/size env vars to the previous values.
