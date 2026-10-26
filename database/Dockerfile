# The postgres image ships gosu 1.19 compiled with Go 1.24, which Trivy flags
# for standard-library CVEs. Build gosu 1.20 from source with the current Go
# toolchain instead of accepting those findings.
FROM golang:1.27.3-alpine3.24 AS gosu-builder

RUN apk add --no-cache git && \
  git clone --depth 1 --branch 1.20 https://github.com/tianon/gosu.git /gosu

WORKDIR /gosu

RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /gosu/gosu .

FROM postgres:18.6-alpine3.24

ENV GOSU_VERSION=1.20

# The tag still points at an Alpine 3.24.2 image whose curl/brotli packages
# have published fixes. Apply them when building the database image.
RUN apk upgrade --no-cache

COPY --from=gosu-builder /gosu/gosu /usr/local/bin/gosu

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
