# Runner monitor

A small C program that executes another program under resource limits and
reports what happened. It is the only part of a graded run that lives inside the
same container as the submission.

**Full documentation:** [Runner images & monitor](https://reisaraujo-miguel.github.io/runcodes/services/runners-and-monitor.html)

## Usage

```text
monitor [options] 'command arg1 arg2'
```

| Option | Meaning |
| --- | --- |
| `-f` | maximum file size in bytes; 0 = no limit |
| `-m` | maximum memory size in bytes; 0 = no limit |
| `-s` | maximum stack size in bytes; 0 = no limit |
| `-i` | input file |
| `-o` | output file |
| `-e` | error file |
| `-r` | report file (default: stdout) |
| `-c` | command to execute |

The report is four lines:

```text
exit_status=0
signal=
time=0.042
cpu_time=0.031
```

`signal` is empty when the command exited on its own. A command killed by a
signal always reports it, even if a core dump was requested; core dumps are
disabled for every run. `time` is wall-clock elapsed time. `cpu_time` is
aggregate CPU time charged to the container cgroup when cgroup accounting is
available; otherwise it falls back to the direct child's rusage.

The report is written after the command's process group is killed and, with
`-r`, renamed into place so the program cannot forge it. The harness kills any
process that escapes the process group.

The binary is linked statically: the monitor image copies it into runner images
built on other distributions, so it must not depend on the builder's libc.

## Build and test

```bash
cd monitor
./test/monitor.test.sh
```

The test build links the monitor statically, so it needs the static libc of your
distribution (`glibc-static` on Fedora, included in `build-essential` on
Debian/Ubuntu). A build failing with `cannot find -lc` is that missing package,
not a source error.

The image is published as `ghcr.io/runcodes-icmc/runcodes-monitor`; the base
runner image copies the binary from `:latest`. The report format and options are
part of the judge contract, so release them together.

## Related

- [Judge](https://reisaraujo-miguel.github.io/runcodes/services/judge.html)
- [Runners](https://reisaraujo-miguel.github.io/runcodes/services/runners-and-monitor.html)
