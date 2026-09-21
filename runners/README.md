# RunCodes Runner Images

This repository houses the Dockerfiles for building each language's compiling and runtime
containers. It might need some small adjustments, like language udpates and firejail insertion.

## Building

There are lots of containers, so the simpler way of executing the build process is to call the
`make all` command from the provided makefile.

## Dependencies

There is the following dependency chain over the docker images: every language depends on the `base` image (available at `base/`) and the `base` image depends on the `runcodes-monitor` image (built from `../monitor`).

## Published images

Images are named after the judge's expectations, not after this repository, and
the names are repeated verbatim in three places, so a rename is a coordinated
change:

| Consumer                                                                                           | Reference                                                 |
| -------------------------------------------------------------------------------------------------- | --------------------------------------------------------- |
| `base/Dockerfile`                                                                                  | `ghcr.io/runcodes-icmc/runcodes-monitor:latest`           |
| every language image (22 of them)                                                                  | `ghcr.io/runcodes-icmc/runcodes-runner-base:latest`       |
| The judge, as a run's image (`language.DefaultImageFormat`, overridable with `JUDGE_IMAGE_FORMAT`) | `ghcr.io/runcodes-icmc/runcodes-runner-<language>:latest` |

The base image is pulled by all 22 language images: 12 derive `FROM` it and
inherit `/usr/bin/runcodes` and `/usr/bin/monitor`, and the other 10 copy those
two binaries into a third-party base image (`python:3.14-slim-trixie`,
`eclipse-temurin`, ...) that carries the toolchain the language needs.

`.github/workflows/monitor.yml` and `.github/workflows/runners.yml` publish
exactly those names on a `v*.*.*` tag, including the `:latest` tag every
consumer pulls — the version tags are for pinning a deployment, and nothing
reads them by default. `runners.yml` builds `base` first and then one job per
language, so a release always pairs the language images with the base they were
built from.

Images are published for **`linux/amd64`** only, the architecture the judge runs
on: most of the toolchains they install come from distributions that do not
build an arm64 variant of every package, and graded runs happen on the judge
host rather than on a developer machine. Building for another architecture means
adding it to `platforms:` in both workflows (and a QEMU setup step for the
foreign one).

The per-language limits a language script sets (`monitor_max_ms` is 1 GiB for
Python, `compilation_timeout` is 60 s for Go and C#) are the language's own
defaults, and they apply unless the judge overrides them — see the precedence
rules in `../judge/DESIGN.md`. Two ordering constraints come with that:

- keep each memory default **below** the judge's `JUDGE_CONTAINER_MEMORY_BYTES`,
  or a run that exceeds it is killed by the cgroup and reported as a signal
  instead of as a memory limit;
- the judge's `JUDGE_COMPILATION_WAIT` must stay **above** the largest
  `compilation_timeout` here (60 s), or the judge gives up on a compilation the
  container is still allowed to run.

Because the judge is half of the contract described in `../judge/DESIGN.md`,
a judge change that alters `container.config` must be released together with a
rebuild of these images (and the monitor, if it changed):

```sh
make all                 # from this directory, or run the runners.yml workflow
```

## The user a run executes as

Every image runs as an unprivileged `runcodes` user, never as root, and the
harness runs as that same user: a submission is compiled and executed with
exactly the permissions the harness has, and neither can change the image they
are graded in — `/usr/bin/runcodes` and `/usr/bin/monitor` included. What a run
may write is what the workspace lets it write.

The uid is deliberately **not** part of the contract. The user is a _system_
user, so its number comes from the range the distribution keeps free; a fixed
one cannot be used, because the images built on Ubuntu inherit that
distribution's `ubuntu` user at 1000. Nothing reads the number: the judge mounts
a 0777 workspace and starts each run with a zero umask precisely so that the uid
writing there does not matter (`internal/podman` in the judge has the details).

What the images have to keep is the shape:

- a non-root `USER runcodes` as the last `USER` in the file, and
- a home directory of `/root`, because that is where the judge mounts the
  workspace and where the toolchains look for `$HOME` to put their caches.

The images built on `base` inherit the user. The 12 that do must `USER root` for
the steps that install a toolchain or append to `/usr/bin/runcodes` — the base
image now ends as `runcodes` — and switch back before their `CMD`. The 10 built
on a third-party base create the user themselves with `groupadd`/`useradd`;
Fedora has neither, so the Zig image installs `shadow-utils`.

## The harness (`base/base-script.sh`)

Every language image is built by appending its own script to the base script and
installing the result as `/usr/bin/runcodes`:

```dockerfile
COPY ./c-script.sh /tmp/script.sh
RUN cat /tmp/script.sh >> /usr/bin/runcodes && rm /tmp/script.sh
CMD [ "/usr/bin/runcodes" ]
```

So the language script only has to set `compilation_command` / `run_command` and
call `compile` / `run_tests`. The base script own:

- reading `container.config` (written by the judge) and **deleting it before any
  submitted code runs**, so the milestone nonce in it cannot be read by the code
  being graded;
- printing the progress milestones with that nonce, which is what stops a
  submission from faking them (see `judge/DESIGN.md`);
- running the compiler and each test case through the monitor, passing the
  per-case limits (`ms_<id>`, `fs_<id>`, `stack_<id>`) and timeout (`t_<id>`);
  the values the judge sends for the global limits (`monitor_max_fs`,
  `monitor_max_ms`, `compilation_timeout`) win over the defaults each language
  script assigns, which are only defaults — see `../judge/DESIGN.md`;
- killing whatever the submission left running after each case and after
  compilation, so nothing can rewrite the results the judge reads next.

## Tests

The harness contract is checked against the real script, a real monitor build and
real child processes — no Docker required:

```sh
./test/harness.test.sh
```

The same script runs in CI (the `monitor` job of
`.github/workflows/tests.yml`), together with the monitor's own suite, on every
push and pull request.

It asserts the nonce on every milestone, that `container.config` is gone before
submitted code runs, that the per-case limits reach the monitor, that a program
that daemonises a helper cannot touch another case's results, and that the
globals the judge sends are not overridden by the language script's defaults.

## License

For information on the license of this project, please see our [license file](LICENSE.md).

## Contributors

For information of the contributors of this project, please see our [contributors file](CONTRIBUTORS.md).

## Contributing

For information on contributing to this project, please see our [contribution guidelines](CONTRIBUTING.md).
