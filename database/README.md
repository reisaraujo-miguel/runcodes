## Developing

To complete the admin bootstrap you must run the bootstrap script. You can run it from the root of the project with the following command:

```bash
source .env && RUNCODES_ADMIN_PASSWORD="<admin-password>" PGHOST=localhost PGPORT=5432 PGUSER=runcodes PGDATABASE=runcodes PGPASSWORD="$RUNCODES_DB_PASSWORD" ./database/bootstrap-admin.sh
```

Note: you must have the `RUNCODES_DB_PASSWORD` environment variable set on the `.env` file.
