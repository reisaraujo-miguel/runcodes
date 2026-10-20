# Atlas project configuration for the versioned schema migrations.
#
# `src` is the desired state. database/schema/02-schema.sql stays the source of
# truth for a fresh install, and `atlas migrate diff` turns changes to it into
# files under database/migrations/. CI fails when the two drift apart.
#
# `dev` is a disposable PostgreSQL database Atlas uses to replay the migration
# directory. Set ATLAS_DEV_URL to it. It must connect as the project's owner
# role (`runcodes`), because the schema file starts with `SET ROLE TO runcodes`.
#
#   ATLAS_DEV_URL='postgres://runcodes:PASSWORD@localhost:5432/atlas_dev?sslmode=disable&search_path=public'
#
# The applied versions are recorded in their own schema so the application role
# (`runcodes_app`) cannot read or change the migration history.
env "local" {
  src = "file://schema/02-schema.sql"
  dev = getenv("ATLAS_DEV_URL")

  migration {
    dir              = "file://migrations"
    revisions_schema = "atlas_schema_revisions"
  }
}
