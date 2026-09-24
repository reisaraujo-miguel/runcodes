FROM postgres:18-alpine3.23

COPY schema/01-init.sql /docker-entrypoint-initdb.d/01-init.sql
COPY schema/02-schema.sql /docker-entrypoint-initdb.d/02-schema.sql
COPY schema/03-seed-data.sql /docker-entrypoint-initdb.d/03-seed-data.sql
COPY schema/04-change-password.sh /docker-entrypoint-initdb.d/04-change-password.sh
COPY schema/05-app-role.sh /docker-entrypoint-initdb.d/05-app-role.sh

RUN chmod +x /docker-entrypoint-initdb.d/04-change-password.sh \
  /docker-entrypoint-initdb.d/05-app-role.sh

USER postgres

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=5 \
  CMD pg_isready -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-postgres}" || exit 1
