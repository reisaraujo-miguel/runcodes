# The postgres image ships gosu 1.19 compiled with Go 1.24, which Trivy flags
# for standard-library CVEs. Build gosu 1.20 from source with the current Go
# toolchain instead of accepting those findings.
FROM golang:1.27.4-alpine3.24 AS gosu-builder

RUN apk add --no-cache git && \
    git clone --depth 1 --branch 1.20 https://github.com/tianon/gosu.git /gosu

WORKDIR /gosu

RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /gosu/gosu .

FROM postgres:18.7-alpine3.24

ENV GOSU_VERSION=1.20

# The tag's package repository keeps moving while its digest (and the cached
# layers built from it) stays the same, so a cached upgrade would keep an old
# package revision -- and its CVEs -- indefinitely. CI passes a per-run value so
# the upgrade runs again on every build; a local build leaves it empty and can
# reuse the cache.
ARG APK_CACHE_BUST=""
RUN echo "apk cache bust: ${APK_CACHE_BUST}" && apk upgrade --no-cache

COPY --from=gosu-builder /gosu/gosu /usr/local/bin/gosu

COPY schema/01-init.sql /docker-entrypoint-initdb.d/01-init.sql
COPY schema/02-schema.sql /docker-entrypoint-initdb.d/02-schema.sql
COPY schema/03-seed-data.sql /docker-entrypoint-initdb.d/03-seed-data.sql
COPY schema/04-change-password.sh /docker-entrypoint-initdb.d/04-change-password.sh
COPY schema/05-app-role.sh /docker-entrypoint-initdb.d/05-app-role.sh

RUN chmod +x /docker-entrypoint-initdb.d/04-change-password.sh \
    /docker-entrypoint-initdb.d/05-app-role.sh

USER postgres
