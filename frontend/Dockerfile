FROM oven/bun:slim AS builder

WORKDIR /app

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

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null "http://127.0.0.1:80/" || exit 1
