---
title: Getting started
description: Run the complete RunCodes stack locally with Docker Compose, bootstrap the admin account and verify a new account.
---

# Getting started

This guide brings up the complete stack locally with Docker Compose and gets
you from a fresh clone to a working submission. It takes roughly ten minutes plus
image builds.

## Choose a path

| Goal | Start here |
| --- | --- |
| Try the platform, demo it, or run a small single-host install | this page |
| Change the backend, judge or frontend | [Development setup](./development-setup.md) |
| Deploy for real | [Deployment overview](../deployment/index.md) and the [production checklist](../deployment/checklist.md) |
| Inspect or integrate with the API | [API reference](../api/index.md) |

The rest of this page is the local Compose path.

## Prerequisites

- **Docker Engine with Compose v2**, or a Docker-compatible API. Podman's socket
  also works (`DOCKER_HOST=unix://$XDG_RUNTIME_DIR/podman/podman.sock`).
- **Rootless Podman on the host**, with its API socket. The judge runs the
  language images through it.
- **About 15 GB of free disk** for images (the language runners are large),
  plus space for PostgreSQL and SeaweedFS volumes.
- `make`, `curl` and `jq` for the optional observability tooling.

The judge uses the host's rootless Podman socket. Start it **before** creating
the judge container:

```bash
systemctl --user start podman.socket
# Check:
ls -l /run/user/$(id -u)/podman/podman.sock   # must be a socket ('s'), not a directory
```

::: warning Start the socket first
The socket is bind-mounted into the judge container when the container is
created. If the container is created before the socket exists, the mount points
at a directory and the judge reports `podman unavailable` for every readiness
probe. Recreating the judge after starting the socket fixes it:

```bash
docker compose up -d --force-recreate judge
```
:::

## 1. Configure secrets

Compose refuses to start without the required secrets. Export them (or put them
in a root `.env` file, which is gitignored):

```bash
export RUNCODES_JWT_SECRET="$(openssl rand -hex 32)"
export RUNCODES_DB_PASSWORD="$(openssl rand -hex 24)"
export RUNCODES_DB_APP_PASSWORD="$(openssl rand -hex 24)"
export RUNCODES_DB_JUDGE_PASSWORD="$(openssl rand -hex 24)"
export RUNCODES_REDIS_PASSWORD="$(openssl rand -hex 24)"
export RUNCODES_JUDGE_TOKEN="$(openssl rand -hex 32)"
export RUNCODES_S3_CREDENTIALS_KEY="$(openssl rand -hex 16)"
export RUNCODES_S3_CREDENTIALS_SECRET="$(openssl rand -hex 24)"
```

| Variable | Used by | Why it is required |
| --- | --- | --- |
| `RUNCODES_JWT_SECRET` | backend | signs session tokens; a default would be a forgeable session |
| `RUNCODES_DB_PASSWORD` | database, migrations | owner role; initializes the cluster |
| `RUNCODES_DB_APP_PASSWORD` | backend | `runcodes_app` role the backend logs in as |
| `RUNCODES_DB_JUDGE_PASSWORD` | judge | narrower `runcodes_judge` role |
| `RUNCODES_REDIS_PASSWORD` | Redis, backend | authenticates the cache; an open cache is writable by any container on the bridge |
| `RUNCODES_JUDGE_TOKEN` | backend, judge | shared bearer token for the judge API |
| `RUNCODES_S3_CREDENTIALS_KEY` / `_SECRET` | backend, judge, SeaweedFS | S3 identity for submissions and cases |

Optional variables include `RUNCODES_PUBLIC_URL` (origin used in verification
and recovery links, default `http://localhost:8080`), `RUNCODES_SMTP_FROM`, SMTP relay
credentials and `RUNCODES_LEGACY_PASSWORD_SALT`. See the
[configuration reference](../reference/configuration.md).

::: tip Keep secrets out of git
`.env`, `.observability/` and `.cache/` are gitignored. Use hex or base64url
values; a literal `$` in a value can be interpolated by Compose.
:::

## 2. Start the stack

```bash
docker compose up --build
```

This builds the frontend, backend, judge and database images, starts
PostgreSQL, Redis, SeaweedFS, smtp4dev and the judge, and publishes the
frontend. The first build takes a while; subsequent starts are fast.

To run detached:

```bash
docker compose up -d --build
docker compose ps
```

### Service URLs

| Service | URL | Notes |
| --- | --- | --- |
| Frontend | <http://localhost:8080> | SPA and `/api/*` proxy |
| Backend | `http://localhost:8443` | loopback only; normally used through `/api` |
| Judge | <http://localhost:9000> | loopback only |
| SeaweedFS | <http://localhost:8333> | loopback only |
| smtp4dev | <http://localhost:8081> | caught mail; SMTP on `127.0.0.1:1025` |

Only the frontend is published on a network interface. The other services are
bound to loopback because they must not be reachable from another host.

## 3. Bootstrap the admin account

The seed creates `admin@admin.com` with a deliberately unusable password hash
(`!`), so a repository cannot ship a published credential. Set a password before
the first login:

```bash
RUNCODES_ADMIN_PASSWORD='choose-a-strong-password' \
  PGHOST=localhost PGPORT=5432 PGUSER=runcodes PGDATABASE=runcodes \
  PGPASSWORD="$RUNCODES_DB_PASSWORD" ./database/bootstrap-admin.sh
```

The account cannot be used until this runs. The bootstrap is idempotent and
preserves a password changed later through the UI; the Ansible deployment only
bootstraps while the stored hash is still the seed value.

## 4. Create and verify an account

New accounts start **unconfirmed**. Signing up sends a verification link; until
it is opened, `POST /api/v1/user/login` answers `403` and every protected route
rejects the session.

1. Open <http://localhost:8080> and sign up.
2. The backend sends the mail to the bundled **smtp4dev** catcher. Open
   <http://localhost:8081>, open the message, and click the verification link.
3. The `/verify-email` page confirms the account. Sign in afterwards.

The link points at `RUNCODES_PUBLIC_URL/verify-email?token=...`, expires after
24 hours, and is reissued from the login page or
`POST /api/v1/user/resend-verification` (which invalidates the previous link).
The resend route always answers `202`, whether or not the address exists, so it
cannot be used to enumerate users.

If someone forgets their password, **Esqueceu a senha?** on the login page sends
a recovery link instead. It points at
`RUNCODES_PUBLIC_URL/reset-password?token=...`, expires after 1 hour, is single
use, and follows the same always-`202` rule. Resetting does not confirm an
account, so an unconfirmed user still needs the verification link above.

::: info Using a real mail relay
Set `RUNCODES_SMTP_HOST`, `RUNCODES_SMTP_PORT`, `RUNCODES_SMTP_FROM` and
optionally `RUNCODES_SMTP_USERNAME` / `RUNCODES_SMTP_PASSWORD`. STARTTLS is used
when the server advertises it. `RUNCODES_PUBLIC_URL` must be the browser-facing
origin, because it is used to build the link.
:::

## 5. Run a submission

1. Sign in as the admin (or promote a professor account from the admin panel).
2. Create a class (offering), then an exercise inside it.
3. Add a test case with an input, an expected output and the per-case limits.
4. Open the exercise as a student, write or upload a solution, and submit.
5. The submission page opens the live event stream; verdicts appear per test
   case as the judge produces them.

If submissions stay queued or fail with `503`, check the judge:

```bash
curl -s http://localhost:9000/readyz | jq
docker compose logs --tail=100 judge
```

The judge's readiness endpoint verifies **both** PostgreSQL and Podman. While it
is not ready, the backend refuses new submissions with `503` instead of queueing
work that cannot run.

## First-run troubleshooting

### `podman unavailable` in the judge

The socket probably did not exist when the container was created, or
`podman.socket` was restarted afterwards:

```bash
systemctl --user status podman.socket
docker compose up -d --force-recreate judge
docker compose exec judge ls -l /run/podman/podman.sock   # must be 's'
curl -s http://localhost:9000/readyz
```

### `connect: permission denied` on the Podman socket

On SELinux hosts the rootless socket is labelled `user_tmp_t`; a container
cannot connect to it. On rootless Podman, a container running as uid 1000 maps
to a host subuid and cannot open the 0660 socket either. The Compose file
already handles both: the judge runs as root inside its namespace and disables
SELinux separation for itself. If you use an override file, keep those settings:

```yaml
judge:
  user: "0:0"
  security_opt:
    - label=disable
```

### Port already in use

The frontend publishes `8080` and `8444`, the backend `127.0.0.1:8443`, the judge
`127.0.0.1:9000`, SeaweedFS `127.0.0.1:8333`, smtp4dev `127.0.0.1:8081` and
`127.0.0.1:1025`, and PostgreSQL `127.0.0.1:5432`. Every one of them is
overridable: `FRONTEND_HTTP_PORT`, `FRONTEND_HTTPS_PORT`, `BACKEND_PORT`,
`JUDGE_PORT`, `DATABASE_PORT`, `SEAWEED_S3_PORT`, `SMTP4DEV_WEB_PORT` and
`SMTP4DEV_SMTP_PORT`. Uncomment the ones you need in `.env` to run this stack
next to another one; when the frontend moves, also set `RUNCODES_PUBLIC_URL` to
the same origin so verification links match.

### Database was created before a feature

`docker compose` only runs `database/schema/*` on an empty volume. Apply pending
migrations explicitly; see [Database](../services/database.md#migrations).

## Next steps

- [Student guide](./student-guide.md) — submit a solution and read the result.
- [Professor guide](./professor-guide.md) — author a class and grade it.
- [Administrator guide](../administration/index.md) — manage the running platform.
- [Submissions & live judging](./submissions.md) — understand statuses and events.
- [How grading works](./grading.md) — scores and comparisons.
- [Backend](../services/backend.md) and [Judge](../services/judge.md) — service
  internals.
- [Observability local stack](../observability/local.md) — add traces, metrics,
  logs and profiles to the same stack.
