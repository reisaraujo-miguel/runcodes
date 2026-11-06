FROM golang:1.27.3-alpine3.24 AS builder

WORKDIR /app/

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o backend .

FROM scratch AS runner

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /app/backend /app/backend

USER 10001:10001

WORKDIR /app/

EXPOSE 8443

ENTRYPOINT ["/app/backend"]
