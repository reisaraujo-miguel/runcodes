FROM oven/bun:slim AS builder

WORKDIR /app

# Caddy serves the API same-origin, so the SPA calls the relative /api path by
# default. Set VITE_API_ENDPOINT to an absolute URL only for split deployments.
ARG VITE_API_ENDPOINT=""
ARG VITE_CONTACT_INFO_HTML=""
ENV VITE_API_ENDPOINT=$VITE_API_ENDPOINT
ENV VITE_CONTACT_INFO_HTML=$VITE_CONTACT_INFO_HTML

COPY . .
RUN bun install --frozen-lockfile && bun run build

FROM caddy:alpine AS dist

# Upstream caddy images no longer ship a "caddy" user (they run as root).
# Create it ourselves so the container can keep running unprivileged.
RUN addgroup -S caddy && adduser -S caddy -G caddy

USER caddy

COPY --from=builder /app/dist /usr/share/caddy

COPY Caddyfile /etc/caddy/Caddyfile

EXPOSE 80
EXPOSE 443

# Probe Caddy's admin API rather than the site itself: it answers regardless of
# the configured site address (HTTP locally, HTTPS with a real domain).
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null "http://127.0.0.1:2019/config/" || exit 1
