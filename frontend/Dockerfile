FROM oven/bun:slim AS builder

WORKDIR /app

COPY . .

RUN bun install && bun run build

FROM builder AS dist

EXPOSE 4173

CMD ["bun", "run", "preview", "--host"]
