# RunCodes

An open-source platform for programming exercises. Students submit code, the
platform compiles and runs it against the exercise's test cases in isolated
containers, and the results are streamed back live.

This repository is the next-generation rewrite of the legacy run.codes platform.
The original compiler engine and language images live in separate repositories
(`compiler-engine`, `compiler-images`); they were folded into this one as
`judge/`, `runners/` and `monitor/`.

> **Documentation:** <https://reisaraujo-miguel.github.io/runcodes/>
>
> The site is the canonical documentation for the platform, the API, deployment
> and operations. Its source lives in [`docs-site/`](docs-site/). The READMEs in
> this repository are short entry points that point there.

## Services

| Service | Path | Stack | Responsibility |
| --- | --- | --- | --- |
| Frontend | `frontend/` | React, Vite, TypeScript, Tailwind, Bun | Web client, editor, live results |
| Backend API | `backend/` | Go, chi, PostgreSQL, JWT, S3 | Auth, classes, submissions, SSE relay |
| Judge | `judge/` | Go, rootless Podman, PostgreSQL, S3 | Claims, executes and grades submissions |
| Runners | `runners/` | Container image per language | Compilation and execution environment |
| Monitor | `monitor/` | C | Per-process limits and runtime report |
| Database | `database/` | PostgreSQL | Schema, seeds, migrations, legacy import |

## Quick start

Requirements: Docker Compose v2, a rootless Podman socket, and about 15 GB of
free disk. Start the socket before creating the judge container:

```bash
systemctl --user start podman.socket
```

Compose refuses to start without the required secrets. Set them in the
environment or in a root `.env` (see [`.env.example`](.env.example)):

```bash
export RUNCODES_JWT_SECRET="$(openssl rand -hex 32)"
export RUNCODES_DB_PASSWORD="$(openssl rand -hex 24)"
export RUNCODES_DB_APP_PASSWORD="$(openssl rand -hex 24)"
export RUNCODES_DB_JUDGE_PASSWORD="$(openssl rand -hex 24)"
export RUNCODES_REDIS_PASSWORD="$(openssl rand -hex 24)"
export RUNCODES_JUDGE_TOKEN="$(openssl rand -hex 32)"
export RUNCODES_S3_CREDENTIALS_KEY="$(openssl rand -hex 16)"
export RUNCODES_S3_CREDENTIALS_SECRET="$(openssl rand -hex 24)"

docker compose up --build
```

| Service | URL |
| --- | --- |
| Frontend | <http://localhost:8080> |
| smtp4dev (caught mail) | <http://localhost:8081> |
| Judge | <http://localhost:9000> (loopback) |
| SeaweedFS | <http://localhost:8333> (loopback) |

The seeded `admin@admin.com` account has no password. The first login, account
verification and troubleshooting steps are in
[Getting started](https://reisaraujo-miguel.github.io/runcodes/guide/getting-started.html).

## Development

```bash
# Backend (needs a .env or environment variables)
cd backend && go run . -debug

# Judge (needs the rootless podman socket and ./exec)
cd judge && make run

# Frontend
cd frontend && bun install && bun run dev
```

Use `docker compose up -d database seaweed redis smtp4dev` for the dependencies,
and point the backend's S3, Redis and judge URLs at `127.0.0.1` (the Compose
environment uses service names that only resolve inside the network). See
[Contributing](CONTRIBUTING.md) for tests, hooks and database changes.

## Deployment

- **Single host, published images:** [Image-based Compose](https://reisaraujo-miguel.github.io/runcodes/deployment/image-compose.html),
  with the workflow in [`deploy/compose/`](deploy/compose/README.md).
- **Single host, built from source:** [Docker Compose from source](https://reisaraujo-miguel.github.io/runcodes/deployment/compose.html).
- **Four machines:** [Ansible deployment](https://reisaraujo-miguel.github.io/runcodes/deployment/ansible-setup.html),
  with the rootless Podman topology, observability stack and operations guide in
  [`deploy/ansible/`](deploy/ansible/README.md).

Application, monitor and runner images are published to GHCR by the workflows in
[`.github/workflows/`](.github/workflows/). See
[CI/CD & releases](https://reisaraujo-miguel.github.io/runcodes/ci-cd.html).

## Documentation

- [Published documentation](https://reisaraujo-miguel.github.io/runcodes/)
- [Student guide](https://reisaraujo-miguel.github.io/runcodes/guide/student-guide.html)
- [Professor guide](https://reisaraujo-miguel.github.io/runcodes/guide/professor-guide.html)
- [Administrator guide](https://reisaraujo-miguel.github.io/runcodes/administration/)
- [Architecture](https://reisaraujo-miguel.github.io/runcodes/guide/architecture.html)
- [How grading works](https://reisaraujo-miguel.github.io/runcodes/guide/grading.html)
- [API reference](https://reisaraujo-miguel.github.io/runcodes/api/)
- [Configuration reference](https://reisaraujo-miguel.github.io/runcodes/reference/configuration.html)
- [Observability](https://reisaraujo-miguel.github.io/runcodes/observability/)
- [Documentation source](docs-site/) — VitePress; run `npm ci && npm run dev`
  there. The site is deployed by [`.github/workflows/docs.yml`](.github/workflows/docs.yml).

## Security

Only Caddy is public. The backend, judge, database, SeaweedFS and mail catcher
are loopback or private; graded code runs without a network namespace and cannot
gain privileges. The backend uses the row-only `runcodes_app` database role;
the judge uses the narrower `runcodes_judge` role. See [Security & scaling](https://reisaraujo-miguel.github.io/runcodes/deployment/security.html).

Container images and the repository are scanned with [Trivy](https://trivy.dev/)
(configuration in [`trivy.yaml`](trivy.yaml)).

## Contributing

Contributions are welcome. Read [CONTRIBUTING.md](CONTRIBUTING.md) and see
[CONTRIBUTORS.md](CONTRIBUTORS.md).

## License

RunCodes is distributed under the **GNU Affero General Public License v3.0** —
see [LICENSE](LICENSE).
