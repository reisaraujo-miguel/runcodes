FROM postgres:18-alpine

COPY schema/01-init.sql /docker-entrypoint-initdb.d/01-init.sql
COPY schema/02-schema.sql /docker-entrypoint-initdb.d/02-schema.sql
COPY schema/03-seed-data.sql /docker-entrypoint-initdb.d/03-seed-data.sql
COPY schema/04-change-password.sh /docker-entrypoint-initdb.d/04-change-password.sh

RUN chmod +x /docker-entrypoint-initdb.d/04-change-password.sh
