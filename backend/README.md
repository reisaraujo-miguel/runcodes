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

### Migrated (legacy) users

Users imported from the old database by `database/migration/` keep a legacy
`legacy-sha1$` password hash until their first login, when it is transparently
upgraded to bcrypt. For those logins to work, set the old system's fixed
password salt in the environment:

```bash
export RUNCODES_LEGACY_PASSWORD_SALT="<old global salt>"
```

Users created by the new system are unaffected by this variable.
