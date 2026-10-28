# Judge integration test

`integration/run.sh` verifies the judge end to end: it starts ephemeral
PostgreSQL and SeaweedFS containers, loads the schema, seeds a commit and a test
case, uploads the source and test I/O, and runs the real engine against a real
language image through rootless Podman.

**Documentation:** [Judge § Integration test](https://reisaraujo-miguel.github.io/runcodes/services/judge.html#integration-test)

## Requirements

- Docker (the harness starts PostgreSQL and SeaweedFS with Docker directly).
- `curl`.
- A rootless Podman API socket, by default
  `unix:///run/user/$UID/podman/podman.sock`.
- The language images used by `JUDGE_TEST_LANGUAGES` (default `c`) or a
  `JUDGE_TEST_IMAGE_FORMAT` pointing at them.

## Run

```bash
make integration                    # starts containers, runs the C test, tears down
JUDGE_TEST_LANGUAGES="c r verilog" ./integration/run.sh  # language matrix
./integration/run.sh --keep         # keep the containers for inspection
```

The Go test skips itself unless `JUDGE_TEST_DB_DSN` is set; `run.sh` sets it.
The harness connects as the database **owner**, because it creates and drops its
own scratch database. A deployment points the judge at `runcodes_judge` instead.

Useful overrides: `ITEST_DB_PORT`, `ITEST_S3_PORT`, `RUNCODES_S3_ENDPOINT`,
`JUDGE_PODMAN_URI`, `JUDGE_TEST_IMAGE_FORMAT`, `JUDGE_TEST_LANGUAGES`,
`JUDGE_TEST_REQUIRE`, `JUDGE_TEST_DISABLE_CGROUPS`, `JUDGE_TEST_KEEP`.
