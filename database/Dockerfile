FROM postgres:18-alpine

ENV POSTGRES_USER=runcodes
ENV POSTGRES_DB=runcodes
ENV POSTGRES_INITDB_ARGS="--encoding=UTF8 --lc-collate=en_US.utf8 --lc-ctype=en_US.utf8"

COPY schema/01-init.sql /docker-entrypoint-initdb.d/01-init.sql
COPY schema/02-schema.sql /docker-entrypoint-initdb.d/02-schema.sql
COPY schema/03-change-password.sh /docker-entrypoint-initdb.d/03-change-password.sh

RUN chmod +x /docker-entrypoint-initdb.d/03-change-password.sh
