# RunCodes Frontend (WIP)

This project houses the future new and improved RunCodes frontend.

## The Stack

- **Frontend Framework**: React
- **Build Tool**: Vite with Rolldown
- **Styling**: Tailwind CSS
- **Language**: TypeScript
- **Package Manager**: Bun
- **Linting**: ESLint with TypeScript support
- **UI Library**: shadcn/ui with Radix UI primitives

## Build & Run

### Prerequisites

- [Bun](https://bun.sh/) (recommended) or Node.js 18+

### Installation

1. Install dependencies:

```bash
bun install
```

2. Start the development server:

```bash
bun run dev
```

3. Open your browser and navigate to `http://localhost:5173`

### Other Commands

- Build for production:

```bash
bun run build
```

- Preview the production build:

```bash
bun run preview
```

- Run linting:

```bash
bun run lint
```

## Observability

The SPA has an opt-in OpenTelemetry Web SDK integration
(`src/lib/telemetry/`). With the observability overlay, the frontend is built
with:

```bash
VITE_OTEL_ENABLED=true
VITE_OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
VITE_OTEL_SERVICE_NAME=runcodes-frontend
```

`FetchInstrumentation` creates a span for every API call and injects
`traceparent`; the backend continues the trace, so a browser click and its API
request (and, for submissions, the judge work) appear in one trace. The
collector must allow the SPA's origin in its OTLP HTTP CORS configuration
(`http://localhost:8080` and `http://localhost:5173` by default). The SDK is
lazy-loaded and is a no-op when the variables are absent.

For local `bun run dev`, copy the telemetry variables from `.env.example` and
start the observability overlay. For a split deployment, set
`VITE_API_ENDPOINT` to the absolute API origin as well; the SDK propagates trace
headers only to the SPA origin and that API origin.

The browser path has a real Playwright test:

```bash
# first run only
bun run test:e2e:install

# with the observability stack running
cd ..
make observability-browser-smoke-test
```

The test loads the SPA at `http://localhost:8080`, issues a fetch, extracts the
`traceparent` injected by `FetchInstrumentation`, and waits for the same trace
to appear in Tempo with both `runcodes-frontend` and `runcodes-backend` spans.
It runs Chromium headless and keeps a trace/screenshot on failure. Set
`PLAYWRIGHT_BROWSERS_PATH` if the browsers live outside the default cache.

See [`../docs/observability/README.md`](../docs/observability/README.md) for the
full stack, collector CORS and browser configuration.

## License

For information on the license of this project, please see our [license file](../LICENSE).

## Contributors

For information of the contributors of this project, please see our [contributors file](../CONTRIBUTORS.md).

## Contributing

For information on contributing to this project, please see our [contribution guidelines](../CONTRIBUTING.md).
