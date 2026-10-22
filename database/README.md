# Database

PostgreSQL schema, seed data, migrations and the one-shot legacy import.

**Full documentation:** [Database](https://reisaraujo-miguel.github.io/runcodes/services/database.html)
· [Configuration](https://reisaraujo-miguel.github.io/runcodes/reference/configuration.html#backend)

## Layout

| Path | Purpose |
| --- | --- |
| `schema/` | bootstrap applied by the database image on an empty volume |
| `migrations/` | versioned Atlas migrations for existing databases |
| `legacy-migration/` | one-shot import of the old production schema |
| `atlas.hcl`, `atlas.sh` | Atlas configuration and pinned CLI |

`schema/02-schema.sql` is the desired state and the input Atlas diffs against.
`migrations/` moves an existing database to that state; CI fails when they
drift apart.

## Schema changes

1. Edit `schema/02-schema.sql`.
2. Generate a migration against a disposable dev database:

   ```bash
   ATLAS_DEV_URL='postgres://runcodes:PASSWORD@localhost:5432/atlas_dev?sslmode=disable&search_path=public' \
     ./database/atlas.sh migrate diff add_the_column
   ```

3. Review the SQL, add any backfill, and run
   `./database/atlas.sh migrate hash` if you edited the migration.
4. Commit the schema, the migration and `migrations/atlas.sum` together.

## Apply migrations

Migrations run as the owner (`runcodes`), never as `runcodes_app`:

```bash
export DATABASE_URL="postgres://runcodes:${RUNCODES_DB_PASSWORD}@127.0.0.1:5432/runcodes?sslmode=disable"
./database/atlas.sh migrate status --url "$DATABASE_URL"
./database/atlas.sh migrate apply  --url "$DATABASE_URL"
```

A fresh Compose volume already contains the current schema but has no Atlas
history. Stamp it before applying the next migration. The baseline/stamp
commands, dev database setup and the `runcodes_app` role are documented in the
[Database guide](https://reisaraujo-miguel.github.io/runcodes/services/database.html#migrations).

## Admin bootstrap

The seed creates `admin@admin.com` with the unusable hash `!`. Set a password
before the first login:

```bash
RUNCODES_ADMIN_PASSWORD='...' \
  PGHOST=localhost PGPORT=5432 PGUSER=runcodes PGDATABASE=runcodes \
  PGPASSWORD="$RUNCODES_DB_PASSWORD" ./database/bootstrap-admin.sh
```

## Legacy import

See [`legacy-migration/README.md`](legacy-migration/README.md) and the
[legacy migration section](https://reisaraujo-miguel.github.io/runcodes/services/database.html#legacy-migration).
The importer covers the relational database, not SeaweedFS objects.

## Related

- [Backend](https://reisaraujo-miguel.github.io/runcodes/services/backend.html)
- [Ansible operations: migrations](https://reisaraujo-miguel.github.io/runcodes/deployment/ansible-operations.html#database-lifecycle-and-migrations)
