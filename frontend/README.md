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
bun run format:check
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

## Design language

The interface is deliberately quiet: content sits on the page instead of a stack
of framed boxes.

- A page or a tab is a `Section` — heading, optional description and action, then
  the content, with no border or background.
- A `Card` is for a discrete object that stands on its own (an exercise in a
  list, a class card, a dialog, a callout). Never put a card inside another card.
- Metrics are a `StatStrip` (one band, rules, plain columns), not one card per
  number.
- Equally weighted actions are a `List` of `ListRow`s (title, description,
  control on the right), not a grid of tiles.
- A table is its own card: `Table` already renders the scroll region on a card
  surface. Lists and forms sit directly under their section heading; empty
  states are placeholders, not cards.
- A section spaces its content (`Section` defaults to `space-y-4`; override it
  with `contentClassName` when the rhythm needs to be tighter or looser).
- Labels are micro-labels: small, uppercase, muted (`Field`, `FieldRow`, table
  headers and the stat strip share that voice). A field list must never read as
  a stack of headings — the section title is the only strong element.
- Type scale: page title 24px, section 18px, sub-section 16px, body 14px.
- Long methodological explanations are a `Hint` disclosure, closed by default,
  not a banner above the content.

The primitives are in `src/components/app/` (`Section`, `StatStrip`, `List`,
`Field`, `Hint`).

## Structure

Routes are in [`src/routes.ts`](src/routes.ts). Components, hooks, API client and
telemetry live under `src/`. Browser tracing is opt-in at build time; see the
[frontend guide](https://reisaraujo-miguel.github.io/runcodes/guide/frontend.html#telemetry).

## Related

- [Using RunCodes](https://reisaraujo-miguel.github.io/runcodes/guide/using-runcodes.html)
- [API reference](https://reisaraujo-miguel.github.io/runcodes/api/)
- [Contributing](../CONTRIBUTING.md)
