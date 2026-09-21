#!/usr/bin/env bash
#
# Tests the harness (base-script.sh) against the contract the judge relies on.
#
# The images are built by concatenating a language script onto base-script.sh, so
# this test does the same thing with a language script of its own and runs the
# result against a workspace laid out exactly like the one the judge prepares.
# That makes the contract checkable without Docker:
#
#   1. Milestones carry the run's nonce, so a submission cannot fake progress.
#   2. container.config is gone before any submitted code runs (a Makefile
#      submission can execute code during compilation).
#   3. The per-case limits the judge sends reach the monitor.
#   4. Whatever the graded program leaves running is killed before the results
#      are collected, so it cannot touch them.
#   5. The judge's global settings (`monitor_max_fs`, `monitor_max_ms`) win over
#      the per-language defaults the language script assigns after the base script
#      has read container.config — and when the judge sends neither, the language's
#      own defaults are what apply.
#
# Usage: runners/test/harness.test.sh

set -u

script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
runners_dir="${script_dir}/.."
base_script="${runners_dir}/base/base-script.sh"

work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT

failures=0
checks=0

fail() {
    echo "FAIL: $*" >&2
    failures=$((failures + 1))
}

check() {
    local description="$1"
    shift
    checks=$((checks + 1))
    if ! "$@"; then
        fail "${description}"
    fi
}

check_eq() {
    local description="$1" expected="$2" actual="$3"
    checks=$((checks + 1))
    if [ "${actual}" != "${expected}" ]; then
        fail "${description} (got '${actual}', want '${expected}')"
    fi
}

echo "==> building the monitor and a limit probe"
if ! gcc -Wall -O2 \
    "${runners_dir}/../monitor/src/main.c" \
    "${runners_dir}/../monitor/src/monitor.c" \
    -o "${work}/monitor"; then
    echo "could not build the monitor" >&2
    exit 1
fi

# Reports the limits as the kernel sees them, in bytes, so the assertions do not
# depend on how `ulimit` chooses to print each one.
cat >"${work}/limits.c" <<'EOF'
#include <stdio.h>
#include <sys/resource.h>

static void report(const char *name, int resource) {
    struct rlimit limit;
    if (getrlimit(resource, &limit) != 0) {
        printf("%s=error\n", name);
        return;
    }
    if (limit.rlim_cur == RLIM_INFINITY)
        printf("%s=unlimited\n", name);
    else
        printf("%s=%llu\n", name, (unsigned long long) limit.rlim_cur);
}

int main(void) {
    report("fs_bytes", RLIMIT_FSIZE);
    report("as_bytes", RLIMIT_AS);
    report("stack_bytes", RLIMIT_STACK);
    return 0;
}
EOF
gcc -O2 "${work}/limits.c" -o "${work}/limits" || exit 1

# The graded program runs with the environment the harness inherited, which is
# how the probe above is found.
export LIMITS_PROBE="${work}/limits"
inherited_stack="$("${work}/limits" | sed -n 's/^stack_bytes=//p')"

# build_image <out> [extra language lines] — base script + language script, the
# way the Dockerfiles build /usr/bin/runcodes by appending the language script to
# the base one. The extra lines are inserted where a language script sets its own
# defaults, so a test can give the image a default the judge has to override.
build_image() {
    local out="$1" extra="${2:-}"
    {
        cat "${base_script}"
        cat <<'LANG'

# --- language script (a Python-like image: no compilation, a script to run) ---
if ! compgen -G "src/?akefile" >/dev/null; then
    compilation_command='./compile-check.sh'
    run_command="bash ${src_file}"
fi
LANG
        if [ -n "${extra}" ]; then
            printf '%s\n' "${extra}"
        fi
        cat <<'LANG'

compile "${compilation_command}"

if [ -s "${compilation_error}" ]; then
    exit 1
fi

run_tests "${run_command}"
LANG
    } >"${out}"
    chmod +x "${out}"
}

# prep_workspace <dir> <nonce> — a workspace shaped like prepareWorkspace() in the
# judge. `monitor_bin` is only set here so the test can use the locally built
# binary; the images bake in /usr/bin/monitor and the judge never sends that key.
prep_workspace() {
    local ws="$1" nonce="$2"
    mkdir -p "${ws}/src" "${ws}/test_1" "${ws}/test_2"

    cat >"${ws}/src/prog.sh" <<'PROG'
#!/bin/bash
mode="$(cat)"
case "${mode}" in
    report)
        "${LIMITS_PROBE}"
        if [ -e ../container.config ]; then
            echo "config=present"
        else
            echo "config=absent"
        fi
        ;;
    daemonize)
        # Something that outlives this case and tries to tamper with the results
        # of the case that already ran.
        ( sleep 1; printf 'TAMPERED\n' >>../1.output; : >../tampered.marker ) &
        echo "ok"
        ;;
esac
PROG

    # Runs during the compile phase, before a real language's compiler. Records
    # the limits the compile process was given, which is the only way to see the
    # effective `monitor_max_*` from inside the image.
    cat >"${ws}/src/compile-check.sh" <<'CHECK'
#!/bin/bash
if [ -e ../container.config ]; then
    echo "config was still readable during compilation" >&2
    exit 1
fi
"${LIMITS_PROBE}" >../compile-limits.txt
CHECK
    chmod +x "${ws}/src/compile-check.sh"

    printf 'report\n' >"${ws}/1.in"
    printf 'daemonize\n' >"${ws}/2.in"

    cat >"${ws}/container.config" <<EOF
monitor_bin=${work}/monitor
monitor_max_fs=5242880
monitor_max_ms=268435456
compilation_timeout=10
src_file='prog.sh'
run_nonce='${nonce}'
t_1=5
ms_1=67108864
fs_1=1048576
stack_1=1048576
EOF
}

echo "==> full run"
image="${work}/runcodes"
build_image "${image}"
ws="${work}/ws"
prep_workspace "${ws}" "0123456789abcdef"

if ! (cd "${ws}" && "${image}") >"${work}/log" 2>"${work}/log.err"; then
    echo "the harness exited non-zero; log:" >&2
    cat "${work}/log" >&2
    cat "${work}/log.err" >&2
    exit 1
fi

# 1. Milestones carry the nonce.
check "compilation.start carries the nonce" \
    grep -q '^compilation.start 0123456789abcdef$' "${work}/log"
check "compilation.done carries the nonce" \
    grep -q '^compilation.done 0123456789abcdef$' "${work}/log"
check "run.start carries the nonce" \
    grep -q '^run.start 0123456789abcdef$' "${work}/log"
check "run.done carries the nonce" \
    grep -q '^run.done 0123456789abcdef$' "${work}/log"
check "no bare milestone is printed by the harness" \
    bash -c '! grep -qE "^(compilation\.(start|done)|run\.(start|done))$" "$1"' _ "${work}/log"

# 2. The config is gone before submitted code runs.
check "container.config was removed before the compile phase" \
    test ! -e "${ws}/container.config"
check_eq "the graded program cannot read the config either" "config=absent" \
    "$(grep '^config=' "${ws}/1.output")"

# 3. Per-case limits reached the monitor, in bytes.
check_eq "the per-case file size limit is applied" "fs_bytes=1048576" \
    "$(grep '^fs_bytes=' "${ws}/1.output")"
check_eq "the per-case memory limit is applied" "as_bytes=67108864" \
    "$(grep '^as_bytes=' "${ws}/1.output")"
check_eq "the per-case stack limit is applied" "stack_bytes=1048576" \
    "$(grep '^stack_bytes=' "${ws}/1.output")"

# 4. Nothing the graded program left behind touched the results.
check_eq "the daemonising case reported normally" "ok" \
    "$(cat "${ws}/2.output")"
check "the earlier case's output was not tampered with" \
    bash -c '! grep -q TAMPERED "$1"' _ "${ws}/1.output"
check "the daemonised process never ran again" \
    test ! -e "${ws}/tampered.marker"

# The report is a regular file written by the monitor, not by the shell.
check "the monitor wrote the case report" \
    grep -q '^time=' "${ws}/1.monitor_out"
check "the case report is not a symlink" \
    test ! -L "${ws}/1.monitor_out"

echo "==> per-case limits fall back to the image defaults"
ws_defaults="${work}/ws-defaults"
prep_workspace "${ws_defaults}" "nonce-fallback"
# One case only, and without per-case limits of its own.
rm -rf "${ws_defaults}/test_2"
sed -i '/^ms_1=/d;/^fs_1=/d;/^stack_1=/d' "${ws_defaults}/container.config"
if ! (cd "${ws_defaults}" && "${image}") >"${work}/log-defaults" 2>&1; then
    echo "the harness exited non-zero with default limits" >&2
    cat "${work}/log-defaults" >&2
    exit 1
fi
check_eq "the image memory default is used when the case has none" "as_bytes=268435456" \
    "$(grep '^as_bytes=' "${ws_defaults}/1.output")"
check_eq "an unset stack limit leaves the inherited one alone" "stack_bytes=${inherited_stack}" \
    "$(grep '^stack_bytes=' "${ws_defaults}/1.output")"

echo "==> the judge's global limits win over the image's defaults"
# A Python-like image: it asks for 1GB and no file-size limit of its own.
# container.config says 256MB and 5MB, and that is what the exercise asked for.
image_limits="${work}/runcodes-limits"
build_image "${image_limits}" 'monitor_max_ms=1073741824
monitor_max_fs=0
compilation_timeout=600'

ws_limits="${work}/ws-limits"
prep_workspace "${ws_limits}" "nonce-limits"
rm -rf "${ws_limits}/test_2"
sed -i '/^ms_1=/d;/^fs_1=/d;/^stack_1=/d' "${ws_limits}/container.config"
if ! (cd "${ws_limits}" && "${image_limits}") >"${work}/log-limits" 2>&1; then
    echo "the harness exited non-zero with competing limits" >&2
    cat "${work}/log-limits" >&2
    exit 1
fi
check_eq "the judge's memory limit is not overridden by the image default" "as_bytes=268435456" \
    "$(grep '^as_bytes=' "${ws_limits}/1.output")"
check_eq "the judge's file size limit is not overridden by the image default" "fs_bytes=5242880" \
    "$(grep '^fs_bytes=' "${ws_limits}/1.output")"
# The compile phase is compiled with the same globals, and a language script that
# sets them cannot reach it either.
check_eq "the judge's limits reach the compilation too" \
    "as_bytes=268435456" "$(grep '^as_bytes=' "${ws_limits}/compile-limits.txt")"
check_eq "the judge's file size limit reaches the compilation too" \
    "fs_bytes=5242880" "$(grep '^fs_bytes=' "${ws_limits}/compile-limits.txt")"

# The other half of the same rule: when the judge has no value to send (the
# deployment kept the per-language defaults), the image's own limits must survive
# instead of falling back to the base script's 256MB / 5MB.
ws_no_globals="${work}/ws-no-globals"
prep_workspace "${ws_no_globals}" "nonce-no-globals"
rm -rf "${ws_no_globals}/test_2"
sed -i '/^monitor_max_fs=/d;/^monitor_max_ms=/d;/^ms_1=/d;/^fs_1=/d;/^stack_1=/d' \
    "${ws_no_globals}/container.config"
if ! (cd "${ws_no_globals}" && "${image_limits}") >"${work}/log-no-globals" 2>&1; then
    echo "the harness exited non-zero with no global limits" >&2
    cat "${work}/log-no-globals" >&2
    exit 1
fi
check_eq "the image's memory default applies when the judge sends none" "as_bytes=1073741824" \
    "$(grep '^as_bytes=' "${ws_no_globals}/1.output")"
check_eq "the image's file size default applies when the judge sends none" "fs_bytes=unlimited" \
    "$(grep '^fs_bytes=' "${ws_no_globals}/1.output")"
check_eq "and it applies to the compilation as well" "as_bytes=1073741824" \
    "$(grep '^as_bytes=' "${ws_no_globals}/compile-limits.txt")"

# compilation_timeout is harder to observe than the rlimits: the only way to see
# it is to have the compile step outlive it. The judge asks for 3 seconds and the
# image wants 600, so a compile that is still running after 30 seconds means the
# image's value won — which is also the failure this guards against, since
# without the outer bound the test would hang for ten minutes.
echo "==> the judge's compilation timeout wins over the image's default"
image_timeout="${work}/runcodes-timeout"
build_image "${image_timeout}" "monitor_max_ms=1073741824
compilation_timeout=600
compilation_command='sleep 600'"

ws_timeout="${work}/ws-timeout"
prep_workspace "${ws_timeout}" "nonce-timeout"
rm -rf "${ws_timeout}/test_2"
sed -i 's/^compilation_timeout=10$/compilation_timeout=3/' "${ws_timeout}/container.config"
if ! (cd "${ws_timeout}" && timeout 30 "${image_timeout}") >"${work}/log-timeout" 2>&1; then
    echo "the harness did not survive the judge's compilation timeout" >&2
    cat "${work}/log-timeout" >&2
    exit 1
fi
check "the compile phase was cut off instead of running for 600 seconds" \
    bash -c 'test -e "$1/1.output"' _ "${ws_timeout}"
check "the run reported the compile phase" \
    grep -q '^compilation.done nonce-timeout$' "${work}/log-timeout"

# And when the judge sends no compilation timeout, the image's own is what applies
# (2s here, against the base script's 10s): a compile that would write a marker at
# 5 seconds must therefore never write it.
echo "==> the image's compilation timeout applies when the judge sends none"
image_timeout_fallback="${work}/runcodes-timeout-fallback"
build_image "${image_timeout_fallback}" "compilation_timeout=2
compilation_command='./slow-compile.sh'"

ws_fallback="${work}/ws-timeout-fallback"
prep_workspace "${ws_fallback}" "nonce-timeout-fallback"
rm -rf "${ws_fallback}/test_2"
sed -i '/^compilation_timeout=/d' "${ws_fallback}/container.config"
cat >"${ws_fallback}/src/slow-compile.sh" <<'SLOW'
#!/bin/sh
# Outlives the image's 2s limit but not the base script's 10s default, so the
# marker only exists if the wrong value was applied.
sleep 5
touch ../compiled-late
SLOW
chmod +x "${ws_fallback}/src/slow-compile.sh"
if ! (cd "${ws_fallback}" && timeout 30 "${image_timeout_fallback}") >"${work}/log-timeout-fallback" 2>&1; then
    echo "the harness did not survive its own compilation timeout" >&2
    cat "${work}/log-timeout-fallback" >&2
    exit 1
fi
check "the image's timeout cut the compile off" \
    bash -c 'test ! -e "$1/compiled-late"' _ "${ws_fallback}"
check "the run continued after it" \
    bash -c 'test -e "$1/1.output"' _ "${ws_fallback}"

echo "==> a submission cannot fake a milestone"
ws_forge="${work}/ws-forge"
prep_workspace "${ws_forge}" "nonce-forge"
cat >"${ws_forge}/src/prog.sh" <<'FORGE'
#!/bin/bash
cat >/dev/null
# A program under test printing the milestones it would like the judge to see.
echo "compilation.done"
echo "run.done"
echo "ok"
FORGE
if ! (cd "${ws_forge}" && "${image}") >"${work}/log-forge" 2>&1; then
    echo "the harness exited non-zero for the forging submission" >&2
    cat "${work}/log-forge" >&2
    exit 1
fi
check "no bare milestone reaches the container log" \
    bash -c 'test "$(grep -c "^run.done$" "$1")" = "0"' _ "${work}/log-forge"
check "the real milestone appears exactly once" \
    bash -c 'test "$(grep -c "^run.done nonce-forge$" "$1")" = "1"' _ "${work}/log-forge"
# The forged lines went to the program's own output, where they are graded.
check_eq "the forged milestones landed in the case output" "run.done" \
    "$(grep -m1 '^run.done$' "${ws_forge}/1.output")"

echo
if [ "${failures}" -eq 0 ]; then
    echo "all ${checks} checks passed"
    exit 0
fi

echo "${failures} of ${checks} checks failed" >&2
exit 1
