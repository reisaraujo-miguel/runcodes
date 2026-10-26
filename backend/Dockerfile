FROM golang:1.27.3-alpine3.24 AS builder

WORKDIR /app/

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o backend .

FROM alpine:3.24 AS runner

RUN addgroup -S runcodes && adduser -S runcodes -G runcodes

USER runcodes

WORKDIR /app/

COPY --from=builder /app/backend .

EXPOSE 8443

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null "http://127.0.0.1:${RUNCODES_API_PORT:-8443}/healthz" || exit 1

CMD ["./backend"]
