FROM golang:1.26-trixie AS builder

WORKDIR /app/

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# The podman bindings need these opt-out tags: without them the local storage
# drivers require gpgme/btrfs development headers.
ARG TAGS="remote containers_image_openpgp exclude_graphdriver_btrfs exclude_graphdriver_overlay exclude_graphdriver_devicemapper exclude_graphdriver_zfs btrfs_noversion"
RUN CGO_ENABLED=0 go build -tags "$TAGS" -o judge .

FROM alpine:3.23 AS runner

RUN apk add --no-cache ca-certificates && \
    addgroup -S runcodes && adduser -S runcodes -G runcodes

# Runs as the host UID/GID by default (see docker-compose) so it can reach the
# rootless podman socket and the shared exec directory.
USER runcodes

WORKDIR /app/

COPY --from=builder /app/judge .

EXPOSE 9000

CMD ["./judge"]
