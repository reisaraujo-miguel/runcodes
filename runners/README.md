# Runner images

One container image per supported language, plus the shared base and the monitor.
The judge runs a submission in the image that matches its extension.

**Full documentation:** [Runner images & monitor](https://reisaraujo-miguel.github.io/runcodes/services/runners-and-monitor.html)
· [Languages & images](https://reisaraujo-miguel.github.io/runcodes/reference/languages.html)

## Images

| Image | Built from |
| --- | --- |
| `ghcr.io/runcodes-icmc/runcodes-monitor` | `../monitor/` |
| `ghcr.io/runcodes-icmc/runcodes-runner-base` | `base/` |
| `ghcr.io/runcodes-icmc/runcodes-runner-<language>` | one directory per language |

There are 22 language images. Twelve inherit `FROM` the base image; the other ten
copy `/usr/bin/runcodes` and `/usr/bin/monitor` into a third-party base that
provides their toolchain. Images are published for `linux/amd64` only.

The names above are the upstream output. The publish workflows derive the
namespace from the repository, so a fork publishes
`ghcr.io/<owner>/<repo>-monitor` and `ghcr.io/<owner>/<repo>-runner-*`. The
language Dockerfiles accept `RUNNER_BASE_IMAGE`, the base accepts
`MONITOR_IMAGE`, and `make` forwards both, so a local build can point at the
same fork namespace. The judge can pull a mirror or another namespace through
`JUDGE_IMAGE_REGISTRY`, `JUDGE_RUNNER_IMAGE_PREFIX` and
`JUDGE_RUNNER_IMAGE_TAG`; the mirror must keep the
`runcodes-runner-<language>` naming.

## Build

```bash
make all
```

Build the base first (the `Makefile` does). Every language depends on the base,
and the base copies the monitor binary out of `runcodes-monitor:latest`.
`.github/workflows/runners.yml` publishes the same images on a `v*.*.*` tag and
calls the monitor workflow before it builds the base.

For a fork namespace:

```bash
make all \
  IMAGE_PREFIX=ghcr.io/<owner>/<repo>-runner \
  MONITOR_IMAGE=ghcr.io/<owner>/<repo>-monitor
```

## Test

```bash
./test/harness.test.sh
```

The harness test needs no container engine: it checks the real script, a local
monitor build and real child processes. It covers nonce-protected milestones, deletion of
`container.config` before submitted code runs, per-case limits, daemon isolation
and judge-global precedence.

## The judge contract

The judge writes `container.config`, the harness prints nonce-bearing
milestones, and the monitor writes the report. These are one contract across
`judge/`, `runners/` and `monitor/`: change them together and rebuild the base
and every language image. The full contract, per-image defaults and release
rules are in the [Judge documentation](https://reisaraujo-miguel.github.io/runcodes/services/judge.html#container-contract)
and [Runner images & monitor](https://reisaraujo-miguel.github.io/runcodes/services/runners-and-monitor.html).

## Layout

Each language directory has a `Dockerfile` and a `<language>-script.sh` appended
to `base/base-script.sh` at build time. `base/base-script.sh` is the harness that
reads the judge's config, runs the compiler and cases through the monitor, and
kills leftover processes.

## Related

- [Judge](https://reisaraujo-miguel.github.io/runcodes/services/judge.html)
- [Monitor](../monitor/README.md)
- [CI/CD & releases](https://reisaraujo-miguel.github.io/runcodes/ci-cd.html)
