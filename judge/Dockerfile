FROM golang:1.27.3-alpine3.24 AS builder

WORKDIR /app/

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# The podman bindings need these opt-out tags: without them the local storage
# drivers require gpgme/btrfs development headers.
ARG TAGS="remote containers_image_openpgp exclude_graphdriver_btrfs exclude_graphdriver_overlay exclude_graphdriver_devicemapper exclude_graphdriver_zfs btrfs_noversion"

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -tags "$TAGS" -o judge .

FROM scratch AS runner

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /app/judge /app/judge

# Runs as the host UID/GID by default (see docker-compose) so it can reach the
# rootless podman socket and the shared exec directory.
USER 10001:10001

WORKDIR /app/

EXPOSE 9000

ENTRYPOINT ["/app/judge"]
