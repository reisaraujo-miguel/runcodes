---
title: Contributing
description: Report issues, set up a development environment, run the tests and submit a pull request.
---

# Contributing

Contributions are welcome. This page covers the practical setup; the canonical
short guide is [`CONTRIBUTING.md`](https://github.com/reisaraujo-miguel/runcodes/blob/main/CONTRIBUTING.md).

## Reporting bugs and suggesting features

Use [GitHub Issues](https://github.com/reisaraujo-miguel/runcodes/issues). A good
report includes:

- what you expected and what happened;
- the commit or release you ran;
- the relevant service logs (`docker compose logs backend`, judge, browser
  console);
- a trace id if observability is enabled (`TraceId` response header);
- reproduction steps, ideally with a minimal submission.

Security issues should not be filed publicly; contact the maintainers first.

## Making a change

1. Fork the repository (or create a branch if you have write access).
2. Make the change with tests.
3. Run the relevant checks locally.
4. Open a pull request describing **what** changed and **why**. Reference an
   issue when one exists.

Pull requests are expected to keep the test suite green:

| Area | Commands |
| --- | --- |
| Backend | `cd backend && gofmt -l . && go vet ./... && go test -race ./...` |
| Judge | `cd judge && make test` (uses the Podman build tags) |
| Frontend | `cd frontend && bun run lint && bun run check-svg && bun test && bun run build` |
| Monitor & harness | `./monitor/test/monitor.test.sh && ./runners/test/harness.test.sh` |
| Database | apply the schema and assert the app role; CI does this |
| Docs site | `cd docs-site && npm run build` |

## Development setup

### Frontend

```bash
cd frontend
bun install
bun run dev      # http://localhost:5173
```

`bun install` also enables the repository's Git hooks (`.githooks/`). The
`pre-commit` hook runs `bun run check-svg` and refuses a commit when a source SVG
is not optimized; fix it with `bun run optimize-svg` and stage the result. To
enable the hooks without installing dependencies:

```bash
git config core.hooksPath .githooks
```

### Backend

```bash
cd backend
# needs a .env or environment variables; see .env.example
go run . -debug
```

Required values and their meanings are in
[Configuration § Backend](./reference/configuration.md#backend). For a full
stack, run PostgreSQL, Redis, SeaweedFS and smtp4dev with Compose.

### Judge

```bash
systemctl --user start podman.socket
mkdir -p ./exec && chmod 0777 ./exec
cd judge
make run
```

The build requires the Podman remote/storage opt-out tags (handled by the
Makefile). See [Judge](./services/judge.md#running-the-judge).

### Full stack

```bash
docker compose up --build
```

See [Getting started](./guide/getting-started.md) for secrets, admin bootstrap
and verification mail.

## Database changes

Schema changes are generated as versioned Atlas migrations:

1. edit `database/schema/02-schema.sql`;
2. generate a migration against a disposable dev database with
   `database/atlas.sh migrate diff <name>`;
3. review the SQL, add backfills that Atlas cannot infer, and refresh
   `atlas.sum`;
4. commit the schema, the migration and `atlas.sum` together.

CI fails when the schema and migration directory disagree, and asserts that
`runcodes_app` cannot change the schema. The full workflow is in
[Database](./services/database.md#migrations).

## Observability changes

Changes under `observability/`, `scripts/observability/` or the instrumentation
paths trigger the `Observability` workflow. Before opening a PR:

```bash
make observability-validate
make observability-smoke-test
```

If you change a pinned image version, update `scripts/observability/validate.sh`
and the dashboard/rules if semantic conventions changed. See
[Local observability stack](./observability/local.md).

## Runner and judge contract changes

A change to `container.config`, milestone formatting, the monitor report or the
`USER runcodes` shape is a **coordinated** change:

1. update the judge;
2. rebuild the base and every language image;
3. release the monitor if it changed;
4. update [Judge](./services/judge.md) and
   [Runner images & monitor](./services/runners-and-monitor.md).

A new judge against old images logs nonce-less milestones and times out.

## Code style

- Go: `gofmt` and `go vet` are enforced in CI. Keep handlers thin and business
  logic in services.
- TypeScript/React: ESLint and the existing component conventions. Run
  `bun run lint`.
- SQL: keep `schema/02-schema.sql` as the desired state; migrations are
  forward-only.
- Shell: the harness tests and observability scripts are checked by CI.
- SVG assets: optimized by `bun run optimize-svg`.

## Contributing to these docs

The published site is the canonical documentation; repository READMEs are short
entry points. The source is in `docs-site/` — see
[Documentation site](./documentation.md) for the structure, local commands and
deployment. When behaviour changes, update the relevant page and the
configuration reference if a variable changed. Do not duplicate the content in a
service README; link to the page instead. The
[documentation map](./reference/source-map.md) lists every page and retired file.

## Contributors

The people behind RunCodes, from [`CONTRIBUTORS.md`](https://github.com/reisaraujo-miguel/runcodes/blob/main/CONTRIBUTORS.md):

**Original authors (legacy run.codes)**

- Felipe Simões Lage Gomes Duarte
- Fábio Henrique Gomes Sikansi
- Samuel Gomes Fadel

**Core contributors**

- Lívia Lelis (Lelis)
- João do Espírito Santo Batista
- Miguel Reis de Araújo (Leug)

Additional contributors are visible in the repository history and on GitHub.

## License

By contributing, you agree that your contribution is distributed under the
**GNU Affero General Public License v3.0**, the license of the project.
