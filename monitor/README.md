# run.codes Compiler - Monitor

This program monitors the execution of another program. In particular, it
gathers information such as the running time, exit status and/or signal that
ended its execution.

**Disclaimer:** this solution, even though it works, might not be sufficient on preventing
miss-use from the users, might be a good idea to take a look of evolving this project and
including other solutions like firejail.

## Usage

```sh
monitor [options]
```

### Options

| Option | Description                                                          |
| ------ | -------------------------------------------------------------------- |
| `-f`   | Maximum file size (bytes) that the program can create (0: no limit)  |
| `-m`   | Maximum memory size (bytes) the program can allocate (0: no limit)   |
| `-s`   | Maximum stack size (bytes) the program can use (0: no limit)         |
| `-i`   | File path to where the monitored program will read from              |
| `-o`   | File path to where the monitored program will output to              |
| `-e`   | File path to where the monitored program will output errors to       |
| `-r`   | File path to write the report to (default: standard output)          |
| `-c`   | Command to be executed (single string with the command and its args) |

### Example

```sh
monitor -i input.txt -o output.txt -e error.txt 'program arg1 arg2'
```

## Report

The report is three lines:

```
exit_status=0
signal=
time=0.042
```

`signal` is empty when the program exited on its own, and `time` is the wall
clock time from the fork to the exit.

The report is written **after** the command's process group has been killed, and
with `-r` it is written to a temporary file first and renamed into place. Both
matter: the program under test runs in the same filesystem as the report, so
otherwise it could write the report itself — claiming a status, a signal or a
running time of its choosing — or point the path at another file with a symlink.
A process that puts itself in a new session still escapes the process-group kill;
the harness (`judge-runners/base/base-script.sh`) kills whatever is left as soon
as the monitor returns.

Where a submission does not control the monitor's arguments, the report path and
the limits come from the judge through the container's `container.config`.

## Image

The monitor is shipped as `ghcr.io/runcodes-icmc/runcodes-monitor`, built from
this directory by `.github/workflows/monitor.yml` on a `v*.*.*` tag.
`judge-runners/base/Dockerfile` is the only place that copies the binary out of
that reference, and every language image gets it from the base — 12 of them
`FROM` it, the other 11 copy it into a third-party base. That makes the `:latest`
tag the workflow pushes the one in use: bumping only the version tags would leave
the graded containers on the previous monitor.

The monitor is the only part of a graded run that lives in the same container as
the submission, so a behaviour change here (the report's format, the
`-r`/`-s` options) is read by the judge and cannot be rolled out on its own —
see the contract in `../judge/DESIGN.md`.

## Tests

```sh
./test/monitor.test.sh
```

The tests build the monitor with the local `gcc` and check the report, the
process-group kill and the three limits against real child processes.

## License

For information on the license of this project, please see our [license file](LICENSE.md).

## Contributors

For information of the contributors to this project, please see our [contributors file](CONTRIBUTORS.md).

## Contributing

For information on contributing to this project, please see our [contribution guidelines](CONTRIBUTING.md).
