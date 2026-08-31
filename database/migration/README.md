# Legacy database migration

One-shot migration from the old production schema (`database/old_schema/schema.old.sql`)
to the new schema (`database/schema/02-schema.sql`).

## How it works

- The old production schema **and data** are restored side by side with the
  new schema: old tables go into an `old` schema, the new schema stays in
  `public`.
- Ordered SQL scripts transform old rows into new rows, preserving old
  integer ids wherever the table has one (only `users` gets new ids, because
  the old PK was the email — an `email -> id` map in `_migration.users_map`
  re-links every foreign key).
- `12-validate.sql` asserts that every old row arrived and that no foreign
  keys are orphaned.

## Prerequisites

1. A fresh instance of the **new** database (build the `database` image, or
   apply `database/schema/01-init.sql` + `02-schema.sql`).
2. Optional but recommended: apply `database/schema/04-seeding.sql` first
   (default admin user + default languages). The migration reuses those rows
   instead of duplicating them.
3. `psql` client; connection env vars (`PGHOST`, `PGPORT`, `PGUSER`,
   `PGPASSWORD`, `PGDATABASE`) pointing at that instance, as the database
   owner (`runcodes`).

## Running

Dump the production data (schema-only is already in this repo):

```bash
pg_dump --data-only --inserts --disable-triggers -h <old-prod-host> -U <user> -d runcodes -f old-data.sql
```

(`--disable-triggers` makes the rows load regardless of table order; the old
foreign keys are still enforced afterwards.)

Then run:

```bash
cd database/migration
PGHOST=127.0.0.1 PGPORT=5432 PGUSER=runcodes PGPASSWORD=... PGDATABASE=runcodes \
  ./migrate.sh ../old_schema/schema.old.sql /path/to/old-data.sql
```

The script (see `migrate.sh` for details):

1. creates the `old` schema,
2. loads the old schema into `old` (the dump's `CREATE USER`/`CREATE DATABASE`
   and `public` qualifiers are stripped/remapped),
3. loads the production data into `old`,
4. runs `00-*.sql` … `12-*.sql` in order, finishing with validation.

You can also run the SQL scripts manually with `psql -f` if you prefer to
stage the old schema yourself.

## Legacy passwords (lazy migration)

The old hashes (SHA-1 of a fixed global salt + plaintext) cannot be converted
to bcrypt offline. Migrated users are stored with
`password_hash = 'legacy-sha1$<hex>'`, and the backend upgrades such hashes to
bcrypt on the first successful login. For that to work, set the old salt in
the backend environment:

```
RUNCODES_LEGACY_PASSWORD_SALT=<the old fixed salt>
```

## Cleanup

- Review the `12-validate.sql` summaries (row counts, role/status
  distributions). The assertions in that script fail the migration if rows
  were lost.
- If you want to keep the legacy-only tables after dropping `old`, run
  `99-optional-archive-legacy.sql` first (it copies them into a `legacy`
  schema, including the lossy-migrated `allowed_files` and `exercise_cases`).
- When everything checks out: `DROP SCHEMA old CASCADE;` (and
  `DROP SCHEMA _migration CASCADE;`).

## Mapping decisions and assumptions

| Topic                                  | Decision                                                                                                                                                                                                                                                                                          |
| -------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Timestamps                             | Old naive `timestamp` values are interpreted as `America/Sao_Paulo` (the old app's timezone) and stored as `timestamptz`.                                                                                                                                                                         |
| `users.type` → `role`                  | 0=`student`, 1=`student` (legacy 'assistant professor' was unused; demoted), 2=`professor`, 3=`admin`, 4=`dev`                                                                                                                                                                                    |
| `users.identifier` → `org_id`          | Copied directly; set to NULL when NULL or duplicated (new column is UNIQUE).                                                                                                                                                                                                                      |
| `users.password` → `password_hash`     | `'legacy-sha1$' + old hash`; upgraded to bcrypt on first login (see above).                                                                                                                                                                                                                       |
| `enrollments.role`                     | 0=`student`, 1=`monitor`, 2=`professor`                                                                                                                                                                                                                                                           |
| `alerts.type`                          | 0=`warning`, 1=`danger`, 2=`info`, 3=`success`, unknown=`info`                                                                                                                                                                                                                                    |
| `alerts.recipients`                    | 0=`students`, 1=`monitors`, 2=`professors`, unknown=`all`                                                                                                                                                                                                                                         |
| `alerts.valid` (date) → `valid_until`  | End of that day (23:59:59 São Paulo).                                                                                                                                                                                                                                                             |
| `commits.status`                       | 0=`queued`, 1=`compiling`, 2=`running` (legacy 'compiled'), 3=`running`, 4=`uncompleted`, 5=`completed`, 6=`compilation_error`, 7=`pending`, 8=`plagiarism`, 9=`server_error`, 10=`running`, 11=`queued` (legacy 'initializing'), unknown=`queued`                                                |
| Case result status                     | 1=`correct`, 2=`bad_formatted_output`, everything else=`killed_with_signal` (signal was in `status_message`)                                                                                                                                                                                      |
| `input_type` / `output_type`           | 0=`text`, 1=`file`, unknown=`text`                                                                                                                                                                                                                                                                |
| Test-case resource limits              | Discarded (legacy monitoring data was unreliable) — inserted as 0.                                                                                                                                                                                                                                |
| `commits.score`                        | Clamped to 0..100 (new CHECK constraint).                                                                                                                                                                                                                                                         |
| `offerings.name`                       | `"<course code> - <course title> (<year>/<term>)"` composed from `courses`.                                                                                                                                                                                                                       |
| `offerings.owner_id`                   | First professor enrollment (earliest `enrollments.id`); NULL if none.                                                                                                                                                                                                                             |
| `offerings.enrollment_code`            | Kept only when unique (new column is UNIQUE); duplicates become NULL.                                                                                                                                                                                                                             |
| `exercises.open_date`                  | Falls back to `deadline` when NULL (new column is NOT NULL).                                                                                                                                                                                                                                      |
| `created_at` / `updated_at`            | Set to the migration time where the old schema had no equivalent (or to `last_update` for test cases).                                                                                                                                                                                            |
| IPs                                    | Text columns cast to `inet`; invalid values become NULL (`0.0.0.0` for NOT NULL columns).                                                                                                                                                                                                         |
| `disk_reports` / `system_disk_reports` | Discarded entirely (monitoring data was unreliable).                                                                                                                                                                                                                                              |
| Dropped tables                         | `courses` (merged), `universities`, `blacklist_mails`, `cria_schools`, `cria_students`, `droplets`, `ganglia`, `jail_status`, `public_exercises`, `questions`, `tickets`.                                                                                                                         |
| Dropped columns                        | `exercises.public/markdown/cases_change`, `commits.hash/compiled_signal`, `exercise_cases.abs_error/input_md5/output_md5`, `allowed_files.compile_command/run_command`, `mail_logs.opened/first_opened_time`, `offerings.classroom/max_students/max_exercises`, `alerts.user_email→user_id`, etc. |

## Idempotency

Scripts are meant to run once against a fresh copy of the new database. They
can be re-run from scratch (the helper drops and recreates `_migration`, and
`migrate.sh` refuses to overwrite an existing `old` schema), but they are not
incremental: re-running on an already-migrated database will duplicate rows.
