# Backend API

The Go HTTP service behind RunCodes. It owns authentication, classes and
exercises, submissions, all PostgreSQL/S3 persistence, and the SSE relay that
streams judge events to the browser.

**Full documentation:** [Backend API](https://reisaraujo-miguel.github.io/runcodes/services/backend.html)
· [API reference](https://reisaraujo-miguel.github.io/runcodes/api/)
· [Configuration](https://reisaraujo-miguel.github.io/runcodes/reference/configuration.html#backend)

## Run

```bash
cd backend
go run . -debug     # or DEBUG_MODE=true
```

The service reads its configuration from the environment (or `backend/.env`);
required values are listed in [`backend/.env.example`](.env.example) and the
[configuration reference](https://reisaraujo-miguel.github.io/runcodes/reference/configuration.html#backend).
For a complete stack, run `podman compose up --build` from the repository root.
Docker Compose v2 works too with `docker compose up --build`.

The backend and judge log in as `runcodes_app`, the row-only database role. Use
the owner role only for migrations and bootstrap.

## Test

```bash
gofmt -l .
go vet ./...
go test -count=1 -race ./...
```

The route table is in [`router.go`](router.go). Handlers parse HTTP; the business
logic and access rules live in `services/`.

## Related

- [Architecture](https://reisaraujo-miguel.github.io/runcodes/guide/architecture.html)
- [Database](https://reisaraujo-miguel.github.io/runcodes/services/database.html)
- [Judge](https://reisaraujo-miguel.github.io/runcodes/services/judge.html)
- [Observability](https://reisaraujo-miguel.github.io/runcodes/observability/)
