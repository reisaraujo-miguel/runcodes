---
title: Database
description: PostgreSQL schema, role model, Atlas migrations, seed data, admin bootstrap and legacy import.
---

# Database

RunCodes uses PostgreSQL for all relational state and as the judge's durable
queue. The database directory contains the desired schema, versioned migrations,
seed data and a one-shot importer for the legacy production database.

## Layout

| Path | Purpose |
| --- | --- |
| `schema/` | bootstrap applied by the `database` image on an empty volume |
| `migrations/` | versioned Atlas migrations for existing databases |
| `legacy-migration/` | one-shot import of the old production schema |
| `atlas.hcl` | Atlas project configuration (desired schema, migration directory) |
| `atlas.sh` | launcher for the pinned Atlas CLI |
| `bootstrap-admin.sh` | sets the seeded admin password |

`schema/02-schema.sql` is the **desired state**: a fresh install gets it, and it
is the input Atlas diffs against. `migrations/` holds the SQL that moves an
existing database to that state. CI fails when the two drift apart, so a schema
change is never committed without its migration.

## The schema

The schema is owned by the `runcodes` role and starts with `SET ROLE TO
runcodes`. The main tables:

| Table | Purpose |
| --- | --- |
| `users` | accounts, role, confirmation flag, editor preference |
| `offerings` | classes, owner, end date, enrollment code, discoverability |
| `enrollments` | per-class role (`student`, `monitor`, `professor`) and ban flag |
| `exercises` | title, description, open date, deadline, visibility |
| `exercises_test_cases` | input/expected output and per-case CPU/memory/stack/file limits |
| `exercises_test_cases_files` | extra file attachments for a test case |
| `exercises_compilation_files` | files placed in the workspace before compilation |
| `exercises_attached_files` | statements and datasets visible to students |
| `exercises_allowed_file_types` | extension allowlist per exercise |
| `allowed_file_types` | extension catalogue and compilable flag |
| `commits` | submissions, status, score, compilation timestamps, S3 key |
| `commits_exercise_test_cases_results` | per-case verdict, CPU time, output, diagnostics |
| `platform_settings` | contact email and disclaimer edited by admins |
| `email_verification_tokens` | SHA-256 of the verification token and expiry |
| `password_reset_tokens` | SHA-256 of the password recovery token and expiry |
| `alerts` | class/user announcements retained from the legacy model |
| `system_*`, `mail_logs`, `user_messages` | retained operational/legacy tables |

Enumerated types:

- `user_t`: `student`, `professor`, `admin`, `dev`;
- `enrollment_role_t`: `student`, `professor`, `monitor`;
- `commit_status_t`: `pending`, `queued`, `compiling`, `compilation_error`,
  `running`, `completed`, `uncompleted`, `server_error`, `plagiarism`, `timeout`;
- `commit_exercise_test_case_status_t`: `killed_with_signal`, `correct`,
  `bad_formatted_output`.

Indexes cover foreign keys and a partial index on `commits (created_at) WHERE
status = 'queued'` for the queue claim.

Foreign keys the application always writes are `NOT NULL`: offerings have an
owner, exercises belong to an offering and creator, test cases/files belong to
their exercise, and commits belong to a user and exercise. The exceptions are
intentional: `alerts.offering_id`/`user_id` may be global, and
`system_logs.user_id`/`user_messages.user_id` are nullable for system/broadcast
rows.

## Role model

Three roles matter:

| Role | Used by | Privileges |
| --- | --- | --- |
| `runcodes` | database container, migrations, bootstrap | owner/superuser; can change schema |
| `runcodes_app` | backend | row read/write; cannot change schema, create roles or read `pg_authid` |
| `runcodes_judge` | judge | claim/update commits and read exercises/test cases only |

`database/schema/05-app-role.sh` creates both service roles. `runcodes_app` gets
table and sequence access plus default privileges for future tables;
`runcodes_judge` gets only `SELECT` on the exercise/test-case tables and
`SELECT, UPDATE` on `commits`, with no sequence or future-table defaults. The
script is idempotent and can be re-run to rotate either password:

```bash
RUNCODES_DB_APP_PASSWORD='...' RUNCODES_DB_JUDGE_PASSWORD='...' \
  PGHOST=localhost PGPORT=5432 PGUSER=runcodes \
  PGPASSWORD="$RUNCODES_DB_PASSWORD" PGDATABASE=runcodes \
  ./database/schema/05-app-role.sh
```

The backend uses the application role and the judge uses the judge role.
Migrations and the importer use the owner. In addition, the RLS migration
enables row-level security on backend-only tables (users, tokens, enrollments,
offerings, settings, messages) with a policy for `runcodes_app` only; the judge
is denied those tables even if a grant is later broadened. This is a
service-role boundary, not per-user row filtering.

CI asserts that `runcodes_app` cannot read the migration history or
`pg_authid`.

## Bootstrapping a fresh database

The database image runs the files in `schema/` on an empty data volume:

1. `01-init.sql` sets the schema owner.
2. `02-schema.sql` creates types, tables and indexes.
3. `03-seed-data.sql` inserts default languages and the admin account (with the
   unusable password hash `!`).
4. `04-change-password.sh` and `05-app-role.sh` finish the setup.

A freshly created Compose volume therefore has the current schema but **no Atlas
history**. Stamp it before applying the next migration (see below).

## Migrations

Migrations run as the owner (`runcodes`), never as `runcodes_app`. They are
forward-only: the Community Edition has no `migrate down`. Fix a bad change with
a compensating migration or restore a backup.

### Applying migrations

```bash
export DATABASE_URL="postgres://runcodes:${RUNCODES_DB_PASSWORD}@127.0.0.1:5432/runcodes?sslmode=disable"

./database/atlas.sh migrate status --url "$DATABASE_URL"
./database/atlas.sh migrate apply  --url "$DATABASE_URL"
```

Applied versions are recorded in the `atlas_schema_revisions` schema. A
statement that cannot run in a transaction (for example `CREATE INDEX
CONCURRENTLY`) needs `migrate apply --tx-mode none` for that run.

### Changing the schema

1. Edit `schema/02-schema.sql`.
2. Generate a migration against a disposable dev database:

   ```bash
   ATLAS_DEV_URL='postgres://runcodes:PASSWORD@localhost:5432/atlas_dev?sslmode=disable&search_path=public' \
     ./database/atlas.sh migrate diff add_the_column
   ```

   The first run downloads the pinned Atlas Community Edition into
   `database/.tools/` (gitignored) and verifies its checksum. Atlas replays
   `migrations/` in the dev database and writes
   `migrations/<timestamp>_add_the_column.sql`.

3. Review the SQL. Atlas cannot infer data backfills or statements that must run
   outside a transaction; add those yourself and refresh the checksum with
   `./database/atlas.sh migrate hash`. For a change that is not a schema diff,
   start from `./database/atlas.sh migrate new <name>`.
4. Commit `schema/02-schema.sql`, the new migration and `migrations/atlas.sum`
   together.

Seed data in `schema/03-seed-data.sql` is not diffed: it only runs when the
volume is created. A seed change that existing databases also need must be a
manual migration.

### Dev database

Atlas needs an empty, disposable database to replay the migration directory.
**Never point `ATLAS_DEV_URL` at the application database**: Atlas executes every
migration in it.

```bash
docker compose exec database createdb -U runcodes atlas_dev

export ATLAS_DEV_URL="postgres://runcodes:${RUNCODES_DB_PASSWORD}@127.0.0.1:5432/atlas_dev?sslmode=disable&search_path=public"
```

The connection must use the `runcodes` owner role. If an interrupted run leaves
objects behind, recreate the database:

```bash
docker compose exec database dropdb -U runcodes atlas_dev
docker compose exec database createdb -U runcodes atlas_dev
```

### Existing databases

The history starts with `20261020150022_baseline.sql`, the schema before the
code-editor preference was added.

**Created before the migration tooling** (no `users.code_editor_enabled`): mark
the baseline as applied, then apply the pending migration.

```bash
./database/atlas.sh migrate set 20261020150022 --url "$DATABASE_URL"
./database/atlas.sh migrate apply --url "$DATABASE_URL"
```

**Created from the current `schema/02-schema.sql`** (for example a fresh Compose
volume): it already contains every change, so stamp it at the latest version
instead of executing the files.

```bash
version=$(basename "$(ls -1 database/migrations/*.sql | tail -n1)" | cut -d_ -f1)
./database/atlas.sh migrate set "$version" --url "$DATABASE_URL"
```

Remember this after `docker compose down -v`: the new volume comes from the
schema file and must be stamped once before the next migration.

In both cases, `migrate status` must report `Migration Status: OK` with no
pending files before you apply anything.

### Migration files

Atlas writes plain SQL named `<UTC timestamp>_<name>.sql` and an `atlas.sum`
integrity file. Do not edit a migration that has already been applied; add a new
one. If you edit a migration that has not been applied, run
`./database/atlas.sh migrate hash`.

### CI checks

The `Database (migrations)` job in `.github/workflows/tests.yml`:

- verifies `atlas.sum` and replays every migration in a disposable database;
- regenerates a diff and fails if `schema/02-schema.sql` changed without a
  matching migration;
- applies the migrations to an empty database, runs the seed and the app-role
  setup, and asserts that `runcodes_app` cannot read the migration history.

## Admin bootstrap

The seed creates `admin@admin.com` without a password. Set one before the first
login:

```bash
source .env && RUNCODES_ADMIN_PASSWORD="<admin-password>" \
  PGHOST=localhost PGPORT=5432 PGUSER=runcodes PGDATABASE=runcodes \
  PGPASSWORD="$RUNCODES_DB_PASSWORD" ./database/bootstrap-admin.sh
```

After that, the script preserves a password changed through the UI. In the
four-machine deployment the data stage runs it only while the hash is still the
seed value `!`.

## Legacy migration

`database/legacy-migration/` imports the old production schema into a fresh
instance of the new one. The old schema and data are restored side by side in an
`old` schema, then ordered SQL scripts transform rows into `public`, preserving
integer ids (only `users` get new ids because the old primary key was the email;
`_migration.users_map` re-links foreign keys). `12-validate.sql` asserts that
every row arrived and that no foreign keys are orphaned.

Prerequisites:

1. a fresh instance of the new database;
2. optionally the seed data (the importer reuses default languages and the admin
   account instead of duplicating them);
3. `psql` and connection variables as the owner.

Dump the old data and run the importer:

```bash
pg_dump --data-only --inserts --disable-triggers \
  -h <old-prod-host> -U <user> -d runcodes -f old-data.sql

cd database/legacy-migration
PGHOST=127.0.0.1 PGPORT=5432 PGUSER=runcodes PGPASSWORD=... PGDATABASE=runcodes \
  ./migrate.sh ../old_schema/schema.old.sql /path/to/old-data.sql
```

Mapping decisions:

| Topic | Mapping |
| --- | --- |
| Timestamps | old naive values are interpreted as `America/Sao_Paulo` and stored as `timestamptz` |
| `users.type` → `role` | 0/1=`student`, 2=`professor`, 3=`admin`, 4=`dev` |
| `users.identifier` → `org_id` | copied; NULL when NULL or duplicated (the new column is unique) |
| `users.password` | `legacy-sha1$<hash>`, upgraded to bcrypt on first login with `RUNCODES_LEGACY_PASSWORD_SALT` |
| `enrollments.role` | 0=`student`, 1=`monitor`, 2=`professor` |
| `alerts.type` | 0=`warning`, 1=`danger`, 2=`info`, 3=`success`, unknown=`info` |
| `alerts.recipients` | 0=`students`, 1=`monitors`, 2=`professors`, unknown=`all` |
| `alerts.valid` | end of that day at 23:59:59 São Paulo, stored as `valid_until` |
| `commits.status` | 0=`queued`, 1=`compiling`, 2/3/10=`running`, 4=`uncompleted`, 5=`completed`, 6=`compilation_error`, 7=`pending`, 8=`plagiarism`, 9=`server_error`, 11=`queued` |
| Case result | 1=`correct`, 2=`bad_formatted_output`, everything else=`killed_with_signal` |
| `input_type` / `output_type` | 0=`text`, 1=`file`, unknown=`text` |
| Resource limits | legacy monitoring data is discarded and inserted as 0 |
| `commits.score` | clamped to 0..100 by the new CHECK constraint |
| `offerings.name` | composed as `<course code> - <course title> (<year>/<term>)` |
| `offerings.owner_id` | first professor enrollment; NULL if none |
| `offerings.enrollment_code` | replaced with a fresh six-character value when unique; duplicates become NULL |
| `exercises.open_date` | falls back to `deadline` when NULL (the new column is NOT NULL) |
| `created_at` / `updated_at` | migration time, or `last_update` for test cases |
| IPs | cast to `inet`; invalid values become NULL (`0.0.0.0` for NOT NULL columns) |
| Monitoring tables | `disk_reports` and `system_disk_reports` are discarded |
| Dropped tables | `courses`, `universities`, `blacklist_mails`, `cria_schools`, `cria_students`, `droplets`, `ganglia`, `jail_status`, `public_exercises`, `questions`, `tickets` |
| Dropped columns | `exercises.public/markdown/cases_change`, `commits.hash/compiled_signal`, `exercise_cases.abs_error/input_md5/output_md5`, and other legacy-only fields |

After validation, drop `old` and `_migration` once the data is verified. Run
`99-optional-archive-legacy.sql` first if the legacy-only tables must be kept in
a `legacy` schema. The migration runner skips `99` because the archive is
optional, not an import step. It is one-shot and refuses to run if `legacy`
already exists; drop the schema explicitly before retrying. The importer is not
incremental either: re-running on an already-migrated database duplicates rows.
It covers the relational database, **not** SeaweedFS objects. Text test-case
inputs and expected outputs are read from their relational columns by the judge;
file-based cases still require their objects to be copied separately. The
enrollment-code migration rotates every surviving code shorter than six
characters, so codes distributed to students before the upgrade stop working;
the class owner can read the new code on the class page. Apply this migration
before deploying the backend version that rejects short codes. Cached class
pages can retain the previous code for up to the 30-second offering TTL; flush
Redis after the migration if that window matters.

## Backups

The database is the backup boundary for relational state; the SeaweedFS volume
is the other half. For a single-host Compose deployment:

```bash
docker compose exec -T database pg_dump -U runcodes -d runcodes -Fc > runcodes.dump
```

For the four-machine deployment, see
[Ansible operations § Backups](../deployment/ansible-operations.md#backups-and-restore).
Stop writers for a consistent backup window, encrypt dumps, store them off-host
and test restores.

## Related pages

- [Architecture](../guide/architecture.md)
- [Backend](./backend.md)
- [Ansible operations: migrations](../deployment/ansible-operations.md#database-lifecycle-and-migrations)
- [Configuration: database](../reference/configuration.md#backend)
