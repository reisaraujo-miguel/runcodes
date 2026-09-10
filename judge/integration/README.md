# Judge integration test

`integration/run.sh` verifies the judge end to end: it starts ephemeral
PostgreSQL and SeaweedFS containers, loads the platform schema, seeds a commit
and a test case, uploads the source and the test-case I/O to S3, and then runs
the real engine against a real language image through **rootless podman**.

The test itself (`../integration_test.go`) is behind the `integration` build tag
and asserts the emitted event stream: a successful compilation, a `correct`
case result and a `completed` run with the expected score.

## Requirements

- Docker (to run PostgreSQL and SeaweedFS).
- `curl`.
- A running **rootless podman** API socket, by default
  `unix:///run/user/$UID/podman/podman.sock`
  (`systemctl --user start podman.socket`).
- The language image. The default is the C image, which the test pulls on
  demand: `ghcr.io/runcodes-icmc/compiler-images-c:latest`.

## Run

```sh
make integration          # starts containers, runs the test, tears down
./integration/run.sh --keep   # keep the containers for inspection
```

## Configuration

| Variable                     | Default                                        | Purpose                              |
| ---------------------------- | ---------------------------------------------- | ------------------------------------ |
| `JUDGE_TEST_DB_DSN`          | set by the script                              | PostgreSQL DSN (test skips if unset) |
| `RUNCODES_S3_ENDPOINT`       | `http://127.0.0.1:8333`                        | S3 endpoint                          |
| `RUNCODES_S3_BUCKET_PREFIX`  | `runcodes-itest`                               | Bucket prefix                        |
| `JUDGE_PODMAN_URI`           | `unix:///run/user/$UID/podman/podman.sock`     | Podman API socket                    |
| `JUDGE_TEST_IMAGE_FORMAT`    | `ghcr.io/runcodes-icmc/compiler-images-%s:latest` | Image format                     |
| `JUDGE_TEST_KEEP`            | `false`                                        | Keep per-commit workspaces           |
| `ITEST_DB_PORT` / `ITEST_S3_PORT` | `55432` / `18333`                         | Host ports                           |

You can also point the test at an already-running PostgreSQL/SeaweedFS without
the harness:

```sh
JUDGE_TEST_DB_DSN="host=localhost port=5432 user=runcodes password=... dbname=runcodes sslmode=disable" \
RUNCODES_S3_ENDPOINT="http://localhost:8333" \
go test -tags "integration remote containers_image_openpgp exclude_graphdriver_btrfs exclude_graphdriver_overlay exclude_graphdriver_devicemapper exclude_graphdriver_zfs btrfs_noversion" \
  -run TestJudgeRunsLanguageImageEndToEnd -v .
```
