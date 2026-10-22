---
title: Frontend application
description: The React SPA — routes, features, development commands, telemetry and testing.
---

# Frontend application

The frontend is a single-page application built with **React 19**, **React
Router**, **TypeScript**, **Vite** and **Tailwind CSS 4**, managed with **Bun**.
Caddy serves the production build and proxies `/api/*` to the backend, so the
browser sees one origin and no CORS.

## Stack

| Concern | Choice |
| --- | --- |
| Framework | React 19 with the React Compiler |
| Routing | React Router (data router, lazy routes) |
| Build | Vite with Rolldown |
| Styling | Tailwind CSS 4, shadcn/ui conventions |
| Components | Radix/Base UI primitives, Lucide icons |
| Editor | CodeMirror 6, with language modes for the supported languages |
| Validation | Zod, React Hook Form |
| Markdown/HTML | `marked` and DOMPurify (sanitized disclaimers) |
| Telemetry | OpenTelemetry Web SDK (fetch instrumentation, opt-in at build time) |
| Tests | Bun test, Playwright for the telemetry e2e test |
| Package manager | Bun |

## Routes

| Path | Access | Purpose |
| --- | --- | --- |
| `/login` | public | sign in; `?mode=signup` shows registration |
| `/signup` | public | redirects to `/login?mode=signup` |
| `/verify-email` | public | consumes the email verification link |
| `/` | signed in | dashboard of open exercises and classes |
| `/exercises` | signed in | exercises across the caller's classes |
| `/exercises/:exerciseId/submit` | signed in | full-width submission workspace with editor |
| `/classes` | signed in | the caller's classes |
| `/classes/:offeringId` | signed in | class detail and exercise progress |
| `/profile` | signed in | name, email, organization, editor preference |
| `/professor` | professor/admin | classes the caller owns or teaches |
| `/professor/class/:offeringId` | professor/admin | class authoring and membership |
| `/professor/exercise/:exerciseId` | professor/admin | exercise, test cases and files |
| `/admin` | admin | admin dashboard |
| `/admin/users` | admin | user search and management |
| `/admin/courses` | admin | all classes, ownership transfer |
| `/admin/settings` | admin | contact email and disclaimer |

Route access is enforced twice: the UI hides routes by role, and every API
endpoint enforces its own authorization. Hiding a link is not a security
control.

## User-facing features

- **Dashboard** — classes and exercises currently open for the signed-in user,
  with deadline and submission status.
- **Submission workspace** — a resizable editor/output layout, language-aware
  syntax highlighting, local drafts, and restore-from-previous-submission. The
  live SSE stream drives per-case results as they arrive.
- **Submission history** — the last 50 submissions for an exercise, with score
  and status, source download and output archive when available.
- **Class pages** — exercises, deadlines, staff, member list for owners, and
  progress for students.
- **Professor authoring** — create and edit exercises, test cases, attached
  files and compilation files, assign monitors and co-professors.
- **Admin panel** — account search and role management, class management,
  platform settings.
- **Theming** — light/dark mode preference.
- **Verification flow** — resend a verification link from the login page and
  land on a dedicated `/verify-email` page.

## Development

### Prerequisites

- [Bun](https://bun.sh/) (recommended) or Node.js 18+.
- The backend and (for submissions) the judge running locally. The easiest path
  is `docker compose up -d database seaweed redis smtp4dev` plus the backend and
  judge; see [Getting started](../guide/getting-started.md).

### Commands

```bash
cd frontend
bun install
bun run dev        # Vite dev server on http://localhost:5173
bun run build      # type-check and production build
bun run preview    # preview the production build
bun run lint       # ESLint
bun test           # unit tests
```

`bun install` also enables the repository's Git hooks (`.githooks/`). The
`pre-commit` hook runs `bun run check-svg` and refuses a commit when a source SVG
is not optimized; fix it with `bun run optimize-svg`. To enable the hooks
without installing dependencies:

```bash
git config core.hooksPath .githooks
```

### Local environment

Copy `frontend/.env.example` to `frontend/.env`. The important settings:

| Variable | Default in dev | Meaning |
| --- | --- | --- |
| `VITE_API_ENDPOINT` | `http://localhost:8443` | absolute API origin for the Vite dev server |
| `VITE_CONTACT_EMAIL` | `contact@example.com` | fallback until the public settings request answers |
| `VITE_CONTACT_DISCLAIMER_HTML` | example HTML | fallback disclaimer |
| `VITE_OTEL_ENABLED` | `true` in the example | enable browser tracing |
| `VITE_OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4318` | browser-reachable collector URL |
| `VITE_OTEL_SERVICE_NAME` | `runcodes-frontend` | browser `service.name` |
| `VITE_OTEL_TRACES_SAMPLER_ARG` | `1.0` | sampling ratio |
| `VITE_DEPLOYMENT_ENVIRONMENT` | `development` | deployment environment attribute |

The Compose build sets `VITE_API_ENDPOINT` empty, so the SPA uses relative
`/api` paths through Caddy. The four-machine image sets
`VITE_OTEL_EXPORTER_OTLP_ENDPOINT=/otel`, so browser traces go to the
same-origin edge route and no hostname is baked into the image.

## Telemetry

Browser tracing is **build-time opt-in**. When `VITE_OTEL_ENABLED=true`, the
SPA lazy-loads the OpenTelemetry Web SDK; otherwise it is a no-op.

`FetchInstrumentation` creates a span for every API call and injects the W3C
`traceparent` header. The backend continues the trace, so one trace can contain
the browser fetch, the API request, the judge work and the database/S3 calls. The
collector must allow the SPA's origin in its OTLP HTTP CORS configuration
(`http://localhost:8080` and `http://localhost:5173` by default).

The browser test loads the SPA, issues a fetch, extracts the injected
`traceparent`, and waits for the same trace to appear in Tempo with both
`runcodes-frontend` and `runcodes-backend` spans:

```bash
# first run only
bun run test:e2e:install

# with the observability stack running
cd ..
make observability-browser-smoke-test
```

It runs headless Chromium and keeps a trace and screenshot on failure. Set
`PLAYWRIGHT_BROWSERS_PATH` if browsers live outside the default cache.

## Production image

The Dockerfile builds the SPA with Bun/Vite and serves it with Caddy. Caddy:

- serves the static bundle and falls back to `index.html` for client-side routes;
- proxies `/api/*` to `BACKEND_UPSTREAM` (`http://backend:8443`);
- optionally enables automatic HTTPS when `RUNCODES_DOMAIN` is a hostname;
- proxies `/otel/v1/traces` to the collector in the four-machine deployment.

The image contains no deployment-specific API URL. The same image works for
Compose, the Ansible deployment and a local preview.

## Repository layout

```text
frontend/
├── src/
│   ├── assets/              logo, SVG icons
│   ├── components/          app shell, editor, submission, admin, UI primitives
│   ├── hooks/               auth, drafts, layout, theme, platform settings
│   ├── lib/                 API client, telemetry, formatting, roles, deadlines
│   ├── routes/              login, verify-email, home, exercises, classes,
│   │                        professor, admin, profile
│   ├── App.tsx              root layout and providers
│   └── routes.ts            route table
├── e2e/                     Playwright telemetry test
├── public/                  static assets
├── Caddyfile
└── Dockerfile
```

## Related pages

- [Using RunCodes](./using-runcodes.md)
- [API reference](../api/index.md)
- [Local observability stack](../observability/local.md)
- [Configuration: frontend](../reference/configuration.md#frontend)
