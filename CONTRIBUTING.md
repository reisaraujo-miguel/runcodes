# Contributing to RunCodes

Thanks for taking the time to contribute. The full contributor guide, including
development setup, tests and the documentation workflow, is on the docs site:
[Contributing](https://reisaraujo-miguel.github.io/runcodes/contributing.html).

## Issues and pull requests

- Report bugs and suggest features on
  [GitHub Issues](https://github.com/reisaraujo-miguel/runcodes/issues).
- Fork the repository, make the change in a branch, and open a pull request.
- Describe **what** changed and **why**; reference an issue when there is one.
- Keep the relevant checks green (see below).

## Development

```bash
# Frontend
cd frontend && bun install && bun run dev

# Backend (needs a .env or environment variables)
cd backend && go run . -debug

# Judge (needs the rootless Podman socket and ./exec)
cd judge && make run
```

For the complete stack use `podman compose up --build` (Podman is the default)
or `docker compose up --build` with Docker Compose v2. The
[contributor guide](https://reisaraujo-miguel.github.io/runcodes/contributing.html#development-setup)
covers dependencies, the observability overlay and each service's commands.

## Checks

| Area | Command |
| --- | --- |
| Backend | `cd backend && gofmt -l . && go vet ./... && go test -race ./...` |
| Judge | `cd judge && make test` |
| Frontend | `cd frontend && bun run lint && bun run check-svg && bun test && bun run build` |
| Monitor & harness | `./monitor/test/monitor.test.sh && ./runners/test/harness.test.sh` |
| Docs site | `cd docs-site && npm ci && npm run build` |

`bun install` enables the repository Git hooks. To enable them without installing
dependencies, run `git config core.hooksPath .githooks`. The pre-commit hook runs
`bun run check-svg`; fix a failure with `bun run optimize-svg`.

## Database changes

Schema changes are generated as versioned Atlas migrations; do not edit a live
database by hand. The workflow is in
[`database/README.md`](database/README.md) and the
[Database guide](https://reisaraujo-miguel.github.io/runcodes/services/database.html#migrations).
CI fails when the schema and migrations disagree.

## Documentation changes

The published site is the canonical documentation; the service READMEs are short
entry points. Documentation source lives in [`docs-site/`](docs-site/), built with
VitePress. When behaviour changes, update the relevant page and run
`npm run build` in `docs-site/`. See the
[documentation guide](https://reisaraujo-miguel.github.io/runcodes/documentation.html).

## Questions

Ask in the comments of the relevant issue or pull request.

## License

Contributions are distributed under the GNU Affero General Public License v3.0
(see [LICENSE](LICENSE)).
