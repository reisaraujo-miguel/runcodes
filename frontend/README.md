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

The SPA is not instrumented with OpenTelemetry yet: browser requests are not
traced, so traces start at Caddy/backend. The backend returns a `TraceId`
response header and its CORS configuration allows `traceparent`/`tracestate`, so
adding `@opentelemetry/sdk-trace-web` (or Grafana Faro) is a contained follow-up.
See [`../docs/observability/README.md`](../docs/observability/README.md#known-limitations)
for the full-stack context and the collector CORS requirement.

## License

For information on the license of this project, please see our [license file](../LICENSE).

## Contributors

For information of the contributors of this project, please see our [contributors file](../CONTRIBUTORS.md).

## Contributing

For information on contributing to this project, please see our [contribution guidelines](../CONTRIBUTING.md).
