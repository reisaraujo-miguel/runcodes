To test the server you can start it with the debug flag:

```bash
go run . -debug
```

To make api calls to protected api's you must provide a valid JWT token:

```bash
curl -H"Authorization: BEARER [token]" \
     -d '{"email": "admin@admin", "name": "test", "end_date": "2027-01-01T23:59:59-03:00"}'\
	   -v http://localhost:8443/api/v1/offerings/create
```

You can get a valid JWT token by using the public login api with a valid user:

```bash
curl -d '{"email": "admin@admin", "password": "[password]"}'\
	   -v http://localhost:8443/api/v1/user/login
```

## Configuration

Every setting is read once, at startup, by `config.Load()`
(`config/config.go`), which applies the defaults, validates the result and
publishes it as `config.C`; the rest of the code reads the values from there.
`.env.example` lists the variables a deployment normally sets.

A required variable that is missing, or a value that cannot be parsed (an
invalid port, a malformed duration, a judge URL without a scheme), aborts
startup with a message naming the variable. A typo therefore fails the deploy
instead of silently falling back to a default.

`DEBUG_MODE=true` — or the `-debug` flag, which wins over the variable —
switches logging to the human-readable development handler and drops the
`Secure` flag from the session cookie so it also works over plain HTTP on
localhost. Request and response bodies are only logged in that mode, and only
for requests that ask for them with the `Debug: reveal-body-logs` header.

`RUNCODES_DB_SSLMODE` (default `disable`) exists for a database that requires
TLS; the Compose setup reaches PostgreSQL over the internal network.

Account verification email is sent through `RUNCODES_SMTP_HOST`,
`RUNCODES_SMTP_PORT` (default `587`), `RUNCODES_SMTP_FROM` and, for a relay that
requires authentication, `RUNCODES_SMTP_USERNAME` / `RUNCODES_SMTP_PASSWORD`.
STARTTLS is used automatically when the server advertises it. The link in the
message is built from `RUNCODES_PUBLIC_URL`, the browser-facing origin of the
frontend; it must be an absolute `http(s)` origin without a path.
`RUNCODES_SMTP_HOST`, `RUNCODES_SMTP_FROM` and `RUNCODES_PUBLIC_URL` are
required, so a deployment that forgets them fails at startup rather than
creating accounts nobody can activate. The port defaults to `587`. The Compose
stack points all of them at the bundled smtp4dev catcher; see the root
[`README.md`](../README.md#account-verification) for the local flow.

## Account verification

Signing up (`POST /api/v1/user/signup`) creates an unconfirmed account and sends
a link to `RUNCODES_PUBLIC_URL/verify-email?token=...`. Only the SHA-256 of the
token is stored (`email_verification_tokens`); the link expires after 24 hours
and reissuing one invalidates the previous link. `POST /api/v1/user/login`
refuses an unconfirmed account with `403`, and every protected route checks the
account's `confirmed` flag, so a session issued before an admin unconfirmed the
account stops working too (the logout route is deliberately exempt).

| Route                                     | Who    | What                                                     |
| ----------------------------------------- | ------ | -------------------------------------------------------- |
| `POST /api/v1/user/verify-email`          | public | consume the token and confirm the account                |
| `POST /api/v1/user/resend-verification`   | public | send a new link; always the same answer                  |

The resend endpoint always answers `202`, whether or not the address has an
account, so it cannot be used to enumerate users. Both routes are rate limited
per client IP.

## Submissions & live judging

Submit source code (multipart form with `exercise_id` and `file`, max 10 MiB):

```bash
curl -H"Authorization: BEARER [token]" \
     -F exercise_id=1 \
     -F file=@main.c \
     -v http://localhost:8443/api/v1/submissions
```

On success it returns `201` with the queued commit:

```json
{
  "commit_id": 42,
  "status": "queued",
  "events_url": "/api/v1/submissions/42/events"
}
```

Subscribe to the live judging events as Server-Sent Events (the commit's owner,
or a professor/admin):

```bash
curl -N -H"Authorization: BEARER [token]" \
     http://localhost:8443/api/v1/submissions/42/events
```

The stream starts with a `snapshot` event carrying the persisted state and then
relays the judge events (`status`, `compilation`, `case_result`, `artifact`,
`finished`, `error`), with a `: ping` comment every 15s. Events are persisted to
Postgres before being forwarded, so a reloading client can always resume from
the snapshot.

List the caller's most recent submissions for an exercise (up to 50, newest
first):

```bash
curl -H"Authorization: BEARER [token]" \
     http://localhost:8443/api/v1/exercises/1/submissions
```

Download the original source of one of the caller's own submissions (the
response is streamed with `Content-Disposition: attachment`):

```bash
curl -OJ -H"Authorization: BEARER [token]" \
     http://localhost:8443/api/v1/submissions/42/source
```

The source endpoint also backs the editor's "restore previous submission"
action. It only returns commits owned by the caller, and a missing or foreign
commit is answered with `404` so existence is never disclosed.

Download the archive generated by one of the caller's submissions (the judge
creates it only when the submission wrote at least one output file; its internal
per-case reports are excluded):

```bash
curl -OJ -H"Authorization: BEARER [token]" \
     http://localhost:8443/api/v1/submissions/42/output
```

The API proxies this archive from the judge, so the browser never needs the
judge's internal URL. A missing archive is answered with `404`.

### Judging configuration

The submission endpoints require a running judge and S3-compatible storage
(SeaweedFS); see `.env.example`. Notable variables:

- `RUNCODES_JUDGE_URL` (default `http://judge:9000`) and `RUNCODES_JUDGE_TOKEN`.
  The judge is health-checked (`GET /readyz`) before a submission is registered;
  if it is down the API returns `503` without creating a row.
- `RUNCODES_JUDGE_STALE_TIMEOUT` (default `15m`): commits stuck in
  `compiling`/`running` longer than this are reconciled to `server_error` by a
  background sweeper.
- `RUNCODES_S3_*`: endpoint, region, credentials and bucket prefix for the
  `<prefix>-commits`, `<prefix>-cases`, `<prefix>-files` and
  `<prefix>-outputfiles` buckets.

## Classes, enrollments and accounts

Every route below requires the session cookie (`POST /api/v1/user/login`) unless
it is marked public.

| Route                                                  | Who              | What                                              |
| ------------------------------------------------------ | ---------------- | ------------------------------------------------- |
| `GET /api/v1/settings/public`                          | public           | contact address and disclaimer for the login page |
| `GET`/`PUT /api/v1/user/profile`                       | signed-in user   | own name, email and organization id               |
| `PUT /api/v1/user/password`                            | signed-in user   | change the password (the current one is required) |
| `POST /api/v1/offerings/enroll`                        | signed-in user   | join a class with its enrollment code             |
| `DELETE /api/v1/offerings/{id}/enrollment`             | signed-in user   | leave a class                                     |
| `GET /api/v1/user/offerings`                           | signed-in user   | classes the caller belongs to or owns             |
| `GET /api/v1/user/exercises`                           | signed-in user   | exercises open right now in those classes         |
| `GET /api/v1/offerings`                                | professor, admin | classes the caller owns or teaches                |
| `PUT`/`DELETE /api/v1/offerings/{id}`                  | class owner      | edit or delete a class                            |
| `GET`/`POST /api/v1/offerings/{id}/members`            | class owner      | list, add monitors and co-professors              |
| `PUT`/`DELETE /api/v1/offerings/{id}/members/{userId}` | class owner      | change a role, ban, unban or remove a member      |
| `GET /api/v1/admin/users`                              | admin            | search accounts                                   |
| `PUT`/`DELETE /api/v1/admin/users/{id}`                | admin            | change a role or delete an account                |
| `GET`/`PUT /api/v1/admin/offerings`                    | admin            | list, edit and transfer any class                 |
| `DELETE /api/v1/admin/offerings/{id}`                  | admin            | delete any class                                  |
| `GET /api/v1/admin/offerings/{id}/members`             | admin            | list the members of any class                     |
| `GET`/`PUT /api/v1/admin/settings`                     | admin            | contact address and disclaimer                    |

Enrollments carry a role (`student`, `monitor`, `professor`). Students join
with the class code; the owner assigns the other two. Access is decided by
`offeringRelation` in `services/offerings-service.go`:

- the **owner** (`offerings.owner_id`, a professor or admin account) may do
  everything with the class, including deleting it and assigning its members;
- a **co-professor** (an enrolled `professor`) authors the class: exercises, test
  cases and files. The member list stays owner-only;
- a **monitor** and an **enrolled student** read the class and its exercises
  (students only the visible ones) and submit solutions;
- a **banned** member keeps their role and loses the access that goes with it;
- the **admin panel** acts on classes it does not own.

A class owner (or a co-professor) also submits to its exercises without being
enrolled, and after the deadline: the deadline is a rule for the students being
graded, and staff need to try an exercise before handing it out.

## Platform settings

The contact address and the disclaimer shown on the login page live in the
`platform_settings` key/value table rather than in the frontend build. The table
is created by `database/schema/02-schema.sql` and seeded by
`03-seed-data.sql`; an admin edits it in the admin panel (`/admin/settings`),
and the public route above serves it. The frontend keeps the build-time
`VITE_CONTACT_EMAIL` / `VITE_CONTACT_DISCLAIMER_HTML` values as the fallback for
the moment before that request answers.

The disclaimer is HTML and is sanitized with DOMPurify in the browser before it
is rendered. An empty disclaimer is valid: it hides the paragraph instead of
falling back to the default.

`docker compose` only runs `database/schema/*` on an empty data volume. New
schema changes are generated as versioned Atlas migrations (see
[`database/README.md`](../database/README.md)); the manual SQL below is the
historical change that predates that tooling, kept for databases provisioned
before `platform_settings` existed.

To add the table to a database that was provisioned before this feature, apply:

```sql
CREATE TABLE IF NOT EXISTS platform_settings (
  key text PRIMARY KEY,
  value text NOT NULL,
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
);
GRANT SELECT, INSERT, UPDATE, DELETE ON platform_settings TO runcodes_app;
INSERT INTO platform_settings (key, value)
VALUES ('contact_email', 'contact@example.com')
ON CONFLICT (key) DO NOTHING;
```

### Migrated (legacy) users

Users imported from the old database by `database/migration/` keep a legacy
`legacy-sha1$` password hash until their first login, when it is transparently
upgraded to bcrypt. For those logins to work, set the old system's fixed
password salt in the environment:

```bash
export RUNCODES_LEGACY_PASSWORD_SALT="<old global salt>"
```

Users created by the new system are unaffected by this variable.
