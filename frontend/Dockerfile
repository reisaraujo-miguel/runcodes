FROM oven/bun:1.4.2-alpine AS builder

WORKDIR /app

# Caddy serves the API same-origin, so the SPA calls the relative /api path by
# default. Set VITE_API_ENDPOINT to an absolute URL only for split deployments.
ARG VITE_API_ENDPOINT=""
ARG VITE_CONTACT_DISCLAIMER_HTML=""
ARG VITE_CONTACT_EMAIL=""
ENV VITE_API_ENDPOINT=$VITE_API_ENDPOINT
ENV VITE_CONTACT_DISCLAIMER_HTML=$VITE_CONTACT_DISCLAIMER_HTML
ENV VITE_CONTACT_EMAIL=$VITE_CONTACT_EMAIL

COPY . .
RUN bun install --frozen-lockfile && bun run build

FROM caddy:2.11.4-alpine AS dist

RUN addgroup -S caddy && adduser -S caddy -G caddy

# The unprivileged user needs CAP_NET_BIND_SERVICE to bind Caddy's privileged
# HTTP/HTTPS ports (80/443). Grant it on the binary: capabilities added to the
# container are dropped when switching from root to the caddy user.
RUN apk add --no-cache libcap && setcap cap_net_bind_service=+ep "$(command -v caddy)"

USER caddy

COPY --from=builder /app/dist /usr/share/caddy

COPY Caddyfile /etc/caddy/Caddyfile

EXPOSE 80
EXPOSE 443

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null "http://127.0.0.1:2019/config/" || exit 1
