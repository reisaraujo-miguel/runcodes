FROM golang:1.27.4-alpine3.24 AS builder

WORKDIR /app/

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o backend .

# An empty world-writable directory for the runtime image: the multipart reader
# spills file parts past its in-memory budget to os.TempDir(), and scratch has no
# /tmp. Without it an over-limit upload fails with ENOENT instead of the
# intended 413. The mode (sticky, world-writable) is preserved by COPY.
RUN mkdir -p /empty-tmp && chmod 1777 /empty-tmp

FROM scratch AS runner

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /empty-tmp /tmp
COPY --from=builder /app/backend /app/backend

ENV TMPDIR=/tmp

USER 10001:10001

WORKDIR /app/

EXPOSE 8443

ENTRYPOINT ["/app/backend"]
