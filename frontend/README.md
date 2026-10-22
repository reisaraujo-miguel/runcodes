# Frontend

The RunCodes single-page application: React 19, React Router, TypeScript, Vite,
Tailwind CSS and CodeMirror, managed with Bun. Caddy serves the production build
and proxies `/api/*` to the backend, so the browser uses one origin.

**Full documentation:** [Frontend application](https://reisaraujo-miguel.github.io/runcodes/guide/frontend.html)

## Run

```bash
cd frontend
bun install
bun run dev        # http://localhost:5173
```

Copy [`frontend/.env.example`](.env.example) to `.env` and point
`VITE_API_ENDPOINT` at the backend (default `http://localhost:8443`). The
production image leaves it empty and uses the relative `/api` path through
Caddy.

## Check

```bash
bun run lint
bun run check-svg
bun test
bun run build
```

`bun install` also enables the repository's Git hooks. If installing is not
possible, enable them directly:

```bash
git config core.hooksPath .githooks
```

The pre-commit hook runs `bun run check-svg`; fix a failure with
`bun run optimize-svg`.

## Structure

Routes are in [`src/routes.ts`](src/routes.ts). Components, hooks, API client and
telemetry live under `src/`. Browser tracing is opt-in at build time; see the
[frontend guide](https://reisaraujo-miguel.github.io/runcodes/guide/frontend.html#telemetry).

## Related

- [Using RunCodes](https://reisaraujo-miguel.github.io/runcodes/guide/using-runcodes.html)
- [API reference](https://reisaraujo-miguel.github.io/runcodes/api/)
- [Contributing](../CONTRIBUTING.md)
