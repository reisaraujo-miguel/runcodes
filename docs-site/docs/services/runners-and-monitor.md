---
title: Runner images & monitor
description: The language image chain, the in-container harness, the process monitor and image publishing.
---

# Runner images & monitor

The judge executes every submission inside a language-specific container. That
container is the **runner image**, and it carries the harness and the monitor
that make grading possible. The judge, the monitor and the language images are
one contract: a change to `container.config` or to a milestone must be released
together.

## Image families

| Image | Built from | Purpose |
| --- | --- | --- |
| `ghcr.io/runcodes-icmc/runcodes-monitor` | `monitor/` | the process monitor binary |
| `ghcr.io/runcodes-icmc/runcodes-runner-base` | `runners/base/` | Debian base with the harness and monitor |
| `ghcr.io/runcodes-icmc/runcodes-runner-<language>` | `runners/<language>/` | one toolchain per language |

These are the upstream names. The publish workflows derive the namespace from
the repository that runs them, so a fork produces
`ghcr.io/<owner>/<repo>-monitor` and `ghcr.io/<owner>/<repo>-runner-*`. The judge
can pull the same images from a mirror or another namespace with
`JUDGE_IMAGE_REGISTRY`, `JUDGE_RUNNER_IMAGE_PREFIX` and
`JUDGE_RUNNER_IMAGE_TAG`; a mirror must keep the `-<language>` and `-base` naming.
See [Configuration: judge](../reference/configuration.md#podman-and-execution).

There are **22 language images**. Twelve inherit `FROM` the base image and
inherit `/usr/bin/runcodes` and `/usr/bin/monitor`. The other ten are built on a
third-party base (`python:3.14-slim-trixie`, `eclipse-temurin`, `golang`,
`rust:slim`, …) and copy the two binaries out of the base image.

Images are published for **`linux/amd64` only** — the architecture the judge
runs on. Most toolchains do not build an arm64 variant of every package, and
graded runs happen on the judge host, not a developer machine.

## The harness

Every language image is built by appending its own script to the base script and
installing the result as `/usr/bin/runcodes`:

```dockerfile
COPY ./c-script.sh /tmp/script.sh
RUN cat /tmp/script.sh >> /usr/bin/runcodes && rm /tmp/script.sh
CMD [ "/usr/bin/runcodes" ]
```

A language script only sets `compilation_command` / `run_command` and calls
`compile` / `run_tests`. The base script owns:

- reading `container.config` (written by the judge) and **deleting it before any
  submitted code runs**, so the milestone nonce cannot be read by the code being
  graded;
- printing progress milestones with that nonce, which stops a submission from
  faking them;
- running the compiler and each test case through the monitor, passing the
  per-case limits (`ms_<id>`, `fs_<id>`, `stack_<id>`) and timeout (`t_<id>`);
- preferring the global limits the judge sends (`monitor_max_fs`,
  `monitor_max_ms`, `compilation_timeout`) over the defaults each language
  script assigns;
- killing whatever the submission left running after each case and after
  compilation.

## The user a run executes as

Every image ends with a non-root `USER runcodes` and a home directory of
`/root`. The harness runs as the same user, so the submission is compiled and
executed with exactly the permissions the harness has and cannot change the
image it is graded in.

The uid is deliberately **not part of the contract**. The user is a *system*
user, so its number comes from the range the distribution keeps free; a fixed uid
would collide with Ubuntu's `ubuntu` user at 1000. The judge mounts a 0777
workspace and starts each run with a zero umask precisely so the numeric uid does
not matter.

The shape that must be preserved:

- a non-root `USER runcodes` as the last `USER` in the file;
- a home directory of `/root`, where the judge mounts the workspace and where
  toolchains place caches.

Images built on the base inherit the user; the twelve that do must switch to
`USER root` for installation steps and back before `CMD`. The ten third-party
bases create the user themselves with `groupadd`/`useradd` (Fedora has neither,
so the Zig image installs `shadow-utils`).

## Language images

| Language | Directory | Base | Compiles |
| --- | --- | --- | :-: |
| C | `c` | runner-base | ✓ |
| C++ | `cpp` | runner-base | ✓ |
| C (OpenMP) | `c-omp` | runner-base | ✓ |
| C++ (OpenMP) | `cpp-omp` | runner-base | ✓ |
| C (OpenMP + MPI) | `c-omp-mpi` | runner-base | ✓ |
| C++ (OpenMP + MPI) | `cpp-omp-mpi` | runner-base | ✓ |
| C# | `dotnet` | `mcr.microsoft.com/dotnet/sdk` | ✓ |
| Fortran | `fortran` | runner-base | ✓ |
| Go | `go` | `golang` | ✓ |
| Haskell | `haskell` | runner-base | ✓ |
| Java | `java` | `eclipse-temurin` | ✓ |
| Julia | `julia` | `julia` | — |
| Lua | `lua` | runner-base | ✓ |
| Octave | `octave` | `gnuoctave/octave` | — |
| Pascal | `pascal` | runner-base | ✓ |
| Portugol | `portugol` | `eclipse-temurin` | ✓ |
| Prolog | `prolog` | runner-base | ✓ |
| Python | `python` | `python:3.14-slim-trixie` | — |
| R | `r` | runner-base | — |
| Rust | `rust` | `rust:slim` | ✓ |
| Verilog | `verilog` | `hdlc/iverilog` | — |
| Zig | `zig` | `fedora` | ✓ |

The judge maps a submission's extension to an image name; the mapping (including
compound extensions such as `main.omp.c`) lives in `judge/internal/language`.
See [Languages & images](../reference/languages.md) for the extension matrix.

## Image-side defaults and precedence

Language scripts set their own defaults for some limits. For example,
`monitor_max_ms` is 1 GiB for Python, most JVM images set none, Go and C# compile
with a 60 s timeout, and Zig uses 20 s. These are **defaults**: a value the judge
writes wins.

Two ordering constraints apply:

- keep every image memory default **below** `JUDGE_CONTAINER_MEMORY_BYTES`, or a
  run that exceeds it is killed by the cgroup and reported as a signal instead of
  as a memory limit;
- keep `JUDGE_COMPILATION_WAIT` **above** the largest `compilation_timeout`
  (60 s), or the judge gives up on a compilation the container may still finish.

## The monitor

`monitor/` is a small C program that runs another program and reports what
happened. It is the only part of a graded run that lives in the same container as
the submission, so its report format and options are read by the judge.

```text
monitor [options] 'command arg1 arg2'
```

| Option | Meaning |
| --- | --- |
| `-f` | maximum file size (bytes) the program may create; 0 = no limit |
| `-m` | maximum memory size (bytes); 0 = no limit |
| `-s` | maximum stack size (bytes); 0 = no limit |
| `-i` | file the program reads from |
| `-o` | file the program writes to |
| `-e` | file the program writes errors to |
| `-r` | file to write the report to (default: stdout) |
| `-c` | command to execute (a single string with its arguments) |

The report is three lines:

```text
exit_status=0
signal=
time=0.042
cpu_time=0.031
```

`signal` is empty when the program exited on its own. `time` is wall-clock time
from fork to exit; `cpu_time` is aggregate CPU charged to the container cgroup,
falling back to the direct child's rusage when cgroup accounting is unavailable.
Memory usage is **not** reported: the monitor is not the
program's parent (the harness runs it through `timeout`), so the judge records
`mem_usage = -1` instead of a number it cannot stand behind.

The report is written **after** the command's process group is killed, and with
`-r` it is written to a temporary file first and renamed into place. Both matter:
the program runs in the same filesystem as the report, so otherwise it could
write the report itself — claiming a status, a signal or a running time of its
choosing — or point the path at another file with a symlink. A process that
starts a new session still escapes the process-group kill; the harness kills
whatever is left as soon as the monitor returns.

The monitor is shipped as `ghcr.io/<owner>/<repo>-monitor` (upstream:
`ghcr.io/runcodes-icmc/runcodes-monitor`); the base Dockerfile is the only place
that copies the binary out of it. The `:latest` tag is therefore the one in use;
bumping only the version tags would leave graded containers on the previous
monitor.

## Building and publishing

```bash
cd runners
make all          # builds base first, then every language
```

The build order matters: every language depends on `base`, and `base` depends on
the monitor image. `.github/workflows/runners.yml` calls `monitor.yml` first,
then builds `base`, then one job per language, so a release always pairs the
language images with the monitor and base they were built from. `monitor.yml`
remains manually dispatchable.

All three workflows publish `:latest` plus semver tags. `app-images.yml` and
`runners.yml` run on a `v*.*.*` tag and on manual dispatch; `monitor.yml` is
called by `runners.yml` and can also be dispatched on its own. The deployment
defaults to `:latest`; the version tags exist for pinning.

## Tests

The harness contract is checked against the real script, a real monitor build and
real child processes — no container engine required:

```sh
./runners/test/harness.test.sh
./monitor/test/monitor.test.sh
```

The harness suite asserts the nonce on every milestone, that
`container.config` is gone before submitted code runs, that per-case limits reach
the monitor, that a daemonised helper cannot touch another case's results, and
that the judge's globals are not overridden by language defaults. CI runs both
suites on every push and pull request.

## Read-only rootfs smoke test

The judge runs every submission with a read-only image filesystem and
size-capped tmpfs at `/tmp` and `/run`; only the workspace bind mount is
persistent. The Go tests assert the generated spec, but the compiler, harness and
monitor still have to work under those mounts on a real runtime.

Run the smoke test on a host with rootless Podman and user namespaces enabled:

```sh
make rootfs-smoke
# equivalent:
./judge/test/rootfs-smoke.sh
```

It defaults to `ghcr.io/runcodes-icmc/runcodes-runner-c:latest`. To test an image
from another repository or tag, pass the full reference or set
`RUNCODES_SMOKE_IMAGE`:

```sh
podman build -t ghcr.io/runcodes-icmc/runcodes-monitor:latest monitor
make -C runners build-base build-c
./judge/test/rootfs-smoke.sh ghcr.io/runcodes-icmc/runcodes-runner-c:latest
```

The script checks that `/` is mounted `ro`, writes to `/tmp` and `/root`, verifies
that an 80 MiB write to a 64 MiB tmpfs fails, then compiles and runs a C case
through `/usr/bin/runcodes` and checks the monitor reports and output. It uses
no network. `KEEP_WORKDIR=1` preserves the temporary workspace on failure;
`SMOKE_TMPFS_SIZE`, `SMOKE_MEMORY`, `RUNCODES_SMOKE_IMAGE` and `PODMAN` override
the defaults.

If the guard fails with "root filesystem is not read-only", the runtime ignored
`--read-only`; if `dd` succeeds, the tmpfs size option was not applied. A compile
failure usually means a toolchain writes outside `/root` and `/tmp`; add that
path to the writable set deliberately, not by dropping the read-only mount.

## Changing the contract

`container.config`, the milestones and the monitor report are shared between the
judge, `runners/base` and the language images. A change to any of them requires:

1. a judge change;
2. a rebuild of the base and every language image;
3. a release of the monitor if it changed;
4. a coordinated deployment, because a new judge against old images logs
   nonce-less milestones as warnings and the run times out.

See [Judge § Container contract](./judge.md#container-contract) for the exact
keys and milestones.

## Related pages

- [Judge](./judge.md)
- [Languages & images](../reference/languages.md)
- [CI/CD & releases](../ci-cd.md)
