# The podman bindings pull in the local storage drivers unless these tags opt
# out; those drivers need gpgme/btrfs development headers a judge host does not
# have. `remote` also trims the local (non-client) code paths.
TAGS ?= remote containers_image_openpgp exclude_graphdriver_btrfs exclude_graphdriver_overlay exclude_graphdriver_devicemapper exclude_graphdriver_zfs btrfs_noversion

.PHONY: build test vet fmt tidy run clean integration

build:
	@mkdir -p bin
	go build -tags "$(TAGS)" -o bin/judge .

test:
	go test -tags "$(TAGS)" ./...

vet:
	go vet -tags "$(TAGS)" ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy

run:
	go run -tags "$(TAGS)" .

clean:
	rm -rf bin

# End-to-end test against real PostgreSQL, SeaweedFS and rootless podman.
integration:
	./integration/run.sh
