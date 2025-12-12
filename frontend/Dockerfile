FROM oven/bun:slim AS builder

WORKDIR /app

COPY . .
RUN bun install --frozen-lockfile && bun run build

FROM caddy:alpine AS dist

COPY --from=builder /app/dist /usr/share/caddy

COPY Caddyfile /etc/caddy/Caddyfile

EXPOSE 80
