# Legacy database migration

One-shot import of the legacy production schema and data into the new schema.
It is not an incremental migration; ongoing schema changes use Atlas in
[`../migrations`](../migrations).

**Full guide:** [Database § Legacy migration](https://reisaraujo-miguel.github.io/runcodes/services/database.html#legacy-migration)

## Prerequisites

- A fresh instance of the new database (build the `database` image or apply
  `../schema/01-init.sql` and `../schema/02-schema.sql`).
- Optionally the seed data (`../schema/03-seed-data.sql`); the importer reuses
  the default languages and admin row.
- `psql` and connection variables as the database **owner** (`runcodes`).

## Run

Dump the old data:

```bash
pg_dump --data-only --inserts --disable-triggers \
  -h <old-host> -U <user> -d runcodes -f old-data.sql
```

Then import:

```bash
cd database/legacy-migration
PGHOST=127.0.0.1 PGPORT=5432 PGUSER=runcodes PGPASSWORD=... PGDATABASE=runcodes \
  ./migrate.sh ../old_schema/schema.old.sql /path/to/old-data.sql
```

The script creates an `old` schema, loads the old schema and data, runs the
numbered transformation scripts, and finishes with `12-validate.sql`, which
fails if rows were lost or foreign keys are orphaned.

Set `RUNCODES_LEGACY_PASSWORD_SALT` on the backend so imported users can upgrade
their `legacy-sha1$` hashes on first login.

## Cleanup

After reviewing the validation output, drop `old` and `_migration`. Run
`99-optional-archive-legacy.sql` first if you want to keep legacy-only tables in
a `legacy` schema. The migration runner skips `99` because it is optional, not a
step in the import. The archive is one-shot: it refuses to run when the `legacy`
schema already exists, because copying the rows again would duplicate them. Drop
`legacy` explicitly before retrying.

The importer is not incremental: re-running on an already-migrated database
duplicates rows. It does not migrate SeaweedFS objects.

See the full mapping table, assumptions and cleanup procedure in the
[Database guide](https://reisaraujo-miguel.github.io/runcodes/services/database.html#legacy-migration).
