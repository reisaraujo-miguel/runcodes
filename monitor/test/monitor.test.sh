#!/usr/bin/env bash
#
# Tests the monitor's contract with the judge.
#
# The monitor is the only component inside a graded container that produces data
# the judge trusts, so its two guarantees are checked here directly:
#
#   1. The report describes the command and nothing else: whatever the command
#      left running is killed before the report is written, and a report path the
#      command pre-created (a symlink, say) is replaced rather than followed.
#   2. The resource limits the judge asks for (-m, -f, -s) are actually applied
#      to the command.
#
# Usage: monitor/test/monitor.test.sh   (builds the monitor into a temp dir)

set -u

script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
src_dir="${script_dir}/../src"

work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT

failures=0
checks=0

fail() {
    echo "FAIL: $*" >&2
    failures=$((failures + 1))
}

check() {
    # check <description> <command...>
    local description="$1"
    shift
    checks=$((checks + 1))
    if ! "$@"; then
        fail "${description}"
    fi
}

echo "==> building the monitor"
if ! { cmake -S "$src_dir" -B "${work}" && cmake --build "${work}"; }; then
    echo "could not build the monitor" >&2
    exit 1
fi

monitor="${work}/monitor"

# A helper the limit tests can run: it tries to allocate and touch 64 MiB.
cat >"${work}/allocate.c" <<'EOF'
#include <stdlib.h>
#include <string.h>

int main(void) {
    size_t n = 64u << 20;
    char *p = malloc(n);
    if (!p)
        return 3;
    memset(p, 1, n);
    return 0;
}
EOF
gcc -O2 "${work}/allocate.c" -o "${work}/allocate"

echo "==> report"
# Without -r the report goes to stdout, which is what the images did before.
stdout_report="$("${monitor}" -o "${work}/out" -e "${work}/err" -c /bin/true)"
check "the report is printed to stdout when -r is absent" \
    grep -q 'exit_status=0' <<<"${stdout_report}"
check "the exit status is reported" \
    grep -q '^exit_status=0$' <<<"${stdout_report}"
check "the time is reported" \
    grep -q '^time=' <<<"${stdout_report}"

# A non-zero exit status is reported as such.
"${monitor}" -o "${work}/out" -e "${work}/err" -r "${work}/report_fail" -c /bin/false
check "a failing command is reported with its status" \
    grep -q '^exit_status=1$' "${work}/report_fail"

echo "==> report file"
"${monitor}" -o "${work}/out" -e "${work}/err" -r "${work}/report" -c /bin/true
check "the report is written to the requested path" \
    grep -q '^exit_status=0$' "${work}/report"
check "no temporary report file is left behind" \
    test -z "$(find "${work}" -maxdepth 1 -name 'report.tmp.*')"

# A report path the command pre-created as a symlink must be replaced, not
# followed: following it would let a submission redirect the judge's input.
echo "sentinel" >"${work}/target"
ln -s "${work}/target" "${work}/linked"
"${monitor}" -o "${work}/out" -e "${work}/err" -r "${work}/linked" -c /bin/true
check "a symlinked report path is replaced by a regular file" \
    test ! -L "${work}/linked"
check "the replaced report has the report content" \
    grep -q '^exit_status=0$' "${work}/linked"
check "the symlink target is left untouched" \
    test "$(cat "${work}/target")" = "sentinel"

echo "==> process containment"
# The command forks a process that outlives it and would rewrite the report a
# second later. It must be killed with the rest of the command's process group,
# before the report is written.
cat >"${work}/tamper.sh" <<EOF
#!/bin/bash
report="\${1}"
(
    sleep 1
    printf 'signal=SIGKILL\\n' >"\${report}"
    : >"\${report}.tampered"
) &
exit 0
EOF
chmod +x "${work}/tamper.sh"
"${monitor}" -o "${work}/out" -e "${work}/err" -r "${work}/contained" \
    -c "${work}/tamper.sh ${work}/contained"
sleep 2 # long enough that the surviving process would have rewritten the report
check "the report is not overwritten by a process the command left behind" \
    grep -q '^signal=$' "${work}/contained"
check "the surviving process never ran again" \
    test ! -e "${work}/contained.tampered"

echo "==> limits"
# -s: the stack limit is inherited by the command.
cat >"${work}/stack.sh" <<'EOF'
#!/bin/bash
ulimit -s
EOF
chmod +x "${work}/stack.sh"
"${monitor}" -o "${work}/stack_out" -e "${work}/err" -r "${work}/stack_report" \
    -s 1048576 -c "${work}/stack.sh"
check "the stack limit is applied to the command" \
    grep -q '^1024$' "${work}/stack_out"

# -m: the address space limit stops an oversized allocation.
"${monitor}" -o "${work}/out" -e "${work}/err" -r "${work}/mem_report" \
    -m 16777216 -c "${work}/allocate"
check "an allocation over the memory limit fails" \
    grep -qv '^exit_status=0$' "${work}/mem_report"

# ...and the same command succeeds without the limit.
"${monitor}" -o "${work}/out" -e "${work}/err" -r "${work}/mem_report_free" \
    -c "${work}/allocate"
check "the same allocation succeeds without a memory limit" \
    grep -q '^exit_status=0$' "${work}/mem_report_free"

# -f: the file size limit bounds what the command can write.
"${monitor}" -o "${work}/big_out" -e "${work}/err" -r "${work}/fs_report" \
    -f 4096 -c "/bin/dd if=/dev/zero bs=1024 count=100"
size="$(stat -c %s "${work}/big_out" 2>/dev/null || echo 0)"
check "the command cannot write past the file size limit" \
    test "${size}" -le 4096

echo "==> input/output redirection"
printf 'hello\n' >"${work}/input"
"${monitor}" -i "${work}/input" -o "${work}/out" -e "${work}/err" \
    -r "${work}/io_report" -c "/bin/cat"
check "the input file is fed to the command" \
    grep -q '^hello$' "${work}/out"

echo
if [ "${failures}" -eq 0 ]; then
    echo "all ${checks} checks passed"
    exit 0
fi

echo "${failures} of ${checks} checks failed" >&2
exit 1
