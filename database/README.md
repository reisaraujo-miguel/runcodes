# Database

PostgreSQL schema, seed data and migrations.

## Layout

| Path                | Purpose                                                                                                                         |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| `schema/`           | Bootstrap applied by the `database` image when its volume is empty: init, schema, seed data, password and the application role. |
| `migrations/`       | Versioned migrations for databases that already exist.                                                                          |
| `legacy-migration/` | One-shot import of the legacy production schema (see its README).                                                               |
| `atlas.hcl`         | Atlas project configuration: desired schema, migration directory and revision schema.                                           |
| `atlas.sh`          | Launcher for the pinned Atlas CLI.                                                                                              |

`schema/02-schema.sql` is the desired state: it is what a fresh install gets, and
it is the input Atlas diffs against. `migrations/` holds the SQL that moves an
existing database to that state. CI fails when the two drift apart, so a schema
change is never committed without its migration.

## Changing the schema

1. Edit `schema/02-schema.sql`.
2. Generate the migration:

   ```bash
   ATLAS_DEV_URL='postgres://runcodes:PASSWORD@localhost:5432/atlas_dev?sslmode=disable&search_path=public' \
     ./database/atlas.sh migrate diff add_the_column
   ```

   The first run downloads the pinned Atlas Community Edition into
   `database/.tools/` (gitignored) and verifies its checksum. Atlas replays
   `migrations/` in the dev database and writes
   `migrations/<timestamp>_add_the_column.sql`.

3. Review the generated SQL. Atlas cannot infer data backfills or statements
   that must run outside a transaction; add those yourself and refresh the
   checksum with `./database/atlas.sh migrate hash`. For a change that is not a
   schema diff at all, start from `./database/atlas.sh migrate new <name>`
   instead.
4. Commit `schema/02-schema.sql`, the new migration and `migrations/atlas.sum`
   together.

Seed data in `schema/03-seed-data.sql` is not diffed: it only runs when the
database volume is created. A seed change that existing databases also need must
be a manual migration (`migrate new`).

## Dev database

Atlas needs an empty, disposable database to replay the migration directory.
Never point `ATLAS_DEV_URL` at the application database: Atlas executes every
migration in it.

With the Compose stack running:

```bash
docker compose exec database createdb -U runcodes atlas_dev

export ATLAS_DEV_URL="postgres://runcodes:${RUNCODES_DB_PASSWORD}@127.0.0.1:5432/atlas_dev?sslmode=disable&search_path=public"
```

The connection must use the `runcodes` owner role: the schema file starts with
`SET ROLE TO runcodes`, and a fresh Compose database creates that role as its
owner. If an interrupted run leaves objects behind, recreate the database:

```bash
docker compose exec database dropdb -U runcodes atlas_dev
docker compose exec database createdb -U runcodes atlas_dev
```

## Applying migrations

Migrations run as the owner (`runcodes`). The application role (`runcodes_app`)
has row access only and cannot change the schema.

```bash
export DATABASE_URL="postgres://runcodes:${RUNCODES_DB_PASSWORD}@127.0.0.1:5432/runcodes?sslmode=disable"

./database/atlas.sh migrate status --url "$DATABASE_URL"
./database/atlas.sh migrate apply  --url "$DATABASE_URL"
```

Applied versions are recorded in the `atlas_schema_revisions` schema, which
`runcodes_app` cannot read. Migrations are forward-only: the Community Edition
has no `migrate down`, so fix a bad change with a new compensating migration (or
restore a backup). A statement that cannot run inside a transaction, such as
`CREATE INDEX CONCURRENTLY`, needs `migrate apply --tx-mode none` for that run.

## Existing databases

The history starts with `20261020150022_baseline.sql`, the schema as it was
before the code-editor preference was added. Pick the case that matches the
database:

- Created **before** the migration tooling and still at that schema (no
  `users.code_editor_enabled`): mark the baseline as applied, then apply the
  pending migration.

  ```bash
  ./database/atlas.sh migrate set 20261020150022 --url "$DATABASE_URL"
  ./database/atlas.sh migrate apply --url "$DATABASE_URL"
  ```

- Created **from the current `schema/02-schema.sql`** (for example a freshly
  created Compose volume): it already contains every change, so stamp it at the
  latest version instead of executing the files.

  ```bash
  version=$(basename "$(ls -1 database/migrations/*.sql | tail -n1)" | cut -d_ -f1)
  ./database/atlas.sh migrate set "$version" --url "$DATABASE_URL"
  ```

  Remember this after `docker compose down -v`: the new volume comes from the
  schema file and must be stamped once before the next migration is applied.

In both cases, `./database/atlas.sh migrate status --url "$DATABASE_URL"` must
report `Migration Status: OK` with no pending files before you apply anything.

## Migration files

Atlas writes plain SQL files named `<UTC timestamp>_<name>.sql` and an
`atlas.sum` integrity file. Do not edit a migration that has already been
applied: add a new one. If you edit a migration that has not been applied yet,
run `./database/atlas.sh migrate hash` so `atlas.sum` matches again.

## CI

The `Database (migrations)` job in `.github/workflows/tests.yml`:

- verifies `atlas.sum` and replays every migration in a disposable database;
- regenerates a diff and fails if `schema/02-schema.sql` has changes without a
  matching migration;
- applies the migrations to an empty database, then runs the seed and the
  application-role setup against it and checks that `runcodes_app` cannot read
  the migration history.

## Admin bootstrap

The seed creates the admin account without a password. Set one before the first
login:

```bash
source .env && RUNCODES_ADMIN_PASSWORD="<admin-password>" \
  PGHOST=localhost PGPORT=5432 PGUSER=runcodes PGDATABASE=runcodes \
  PGPASSWORD="$RUNCODES_DB_PASSWORD" ./database/bootstrap-admin.sh
```

`RUNCODES_DB_PASSWORD` must be set in `.env` (or the environment).
