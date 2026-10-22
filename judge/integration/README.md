# Judge integration test

`integration/run.sh` verifies the judge end to end: it starts ephemeral
PostgreSQL and SeaweedFS containers, loads the schema, seeds a commit and a test
case, uploads the source and test I/O, and runs the real engine against a real
language image through rootless Podman.

**Documentation:** [Judge § Integration test](https://reisaraujo-miguel.github.io/runcodes/services/judge.html#integration-test)

## Requirements

- Docker (for PostgreSQL and SeaweedFS).
- `curl`.
- A rootless Podman API socket, by default
  `unix:///run/user/$UID/podman/podman.sock`.
- The language image; the default is
  `ghcr.io/runcodes-icmc/runcodes-runner-c:latest`.

## Run

```bash
make integration          # starts containers, runs the test, tears down
./integration/run.sh --keep   # keep the containers for inspection
```

The Go test skips itself unless `JUDGE_TEST_DB_DSN` is set; `run.sh` sets it.
The harness connects as the database **owner**, because it creates and drops its
own scratch database. A deployment points the judge at `runcodes_app` instead.

Useful overrides: `ITEST_DB_PORT`, `ITEST_S3_PORT`, `RUNCODES_S3_ENDPOINT`,
`JUDGE_PODMAN_URI`, `JUDGE_TEST_IMAGE_FORMAT`, `JUDGE_TEST_KEEP`.
