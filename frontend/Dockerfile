FROM oven/bun:1.4.3-alpine AS builder

WORKDIR /app

# Caddy serves the API same-origin, so the SPA calls the relative /api path by
# default. Set VITE_API_ENDPOINT to an absolute URL only for split deployments.
ARG VITE_API_ENDPOINT=""
ARG VITE_CONTACT_DISCLAIMER_HTML=""
ARG VITE_CONTACT_EMAIL=""
# Browser telemetry is opt-in at build time. The Compose observability overlay
# sets these args; a plain `docker compose up` leaves tracing disabled.
ARG VITE_OTEL_ENABLED="false"
ARG VITE_OTEL_EXPORTER_OTLP_ENDPOINT=""
ARG VITE_OTEL_SERVICE_NAME="runcodes-frontend"
ARG VITE_OTEL_SERVICE_VERSION="dev"
ARG VITE_DEPLOYMENT_ENVIRONMENT="development"
ARG VITE_OTEL_TRACES_SAMPLER_ARG="0.1"
ENV VITE_API_ENDPOINT=$VITE_API_ENDPOINT
ENV VITE_CONTACT_DISCLAIMER_HTML=$VITE_CONTACT_DISCLAIMER_HTML
ENV VITE_CONTACT_EMAIL=$VITE_CONTACT_EMAIL
ENV VITE_OTEL_ENABLED=$VITE_OTEL_ENABLED
ENV VITE_OTEL_EXPORTER_OTLP_ENDPOINT=$VITE_OTEL_EXPORTER_OTLP_ENDPOINT
ENV VITE_OTEL_SERVICE_NAME=$VITE_OTEL_SERVICE_NAME
ENV VITE_OTEL_SERVICE_VERSION=$VITE_OTEL_SERVICE_VERSION
ENV VITE_DEPLOYMENT_ENVIRONMENT=$VITE_DEPLOYMENT_ENVIRONMENT
ENV VITE_OTEL_TRACES_SAMPLER_ARG=$VITE_OTEL_TRACES_SAMPLER_ARG

COPY bun.lock package.json ./
RUN bun install --frozen-lockfile

COPY . .
RUN bun -b run build

# The published caddy:2.11.7-alpine binary was compiled with an older Go
# toolchain and an x/net version with a known HTTP/2 DoS. Rebuild the same
# Caddy release with the current Go toolchain and the fixed x/net.
FROM caddy:2.11.7-builder-alpine AS caddy-builder

RUN CGO_ENABLED=0 xcaddy build v2.11.7 \
  --replace golang.org/x/net=golang.org/x/net@v0.60.0 \
  --output /usr/bin/caddy

FROM alpine:3.24 AS dist

RUN apk add --no-cache ca-certificates libcap && \
  addgroup -S caddy && adduser -S caddy -G caddy && \
  mkdir -p /data /config && chown caddy:caddy /data /config

COPY --from=caddy-builder /usr/bin/caddy /usr/bin/caddy

# The unprivileged user needs CAP_NET_BIND_SERVICE to bind Caddy's privileged
# HTTP/HTTPS ports (80/443). Grant it on the binary: capabilities added to the
# container are dropped when switching from root to the caddy user.
RUN setcap cap_net_bind_service=+ep /usr/bin/caddy

USER caddy

COPY --from=builder /app/dist /usr/share/caddy

COPY Caddyfile /etc/caddy/Caddyfile

ENV CADDY_VERSION=v2.11.7 \
  XDG_CONFIG_HOME=/config \
  XDG_DATA_HOME=/data

EXPOSE 80
EXPOSE 443
EXPOSE 2019

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null "http://127.0.0.1:2019/config/" || exit 1

ENTRYPOINT ["caddy"]
CMD ["run", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"]
