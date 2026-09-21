#!/bin/bash

# Configuration variables shared by all containers
container_config_file=container.config
compilation_monitor_error=compilation.monitor_err
compilation_monitor_output=compilation.monitor_out
compilation_error=compilation.err
compilation_output=compilation.out
outputfiles_dir=outputfiles
monitor_bin=/usr/bin/monitor
user_bin=executable

# Variables we expect to be inside ${container_config_file}:
# monitor_max_fs, monitor_max_ms, compilation_timeout, src_file, run_nonce and the
# per-case t_<id> (seconds), ms_<id> (bytes), fs_<id> (bytes) and stack_<id>
# (bytes).
#
# They are read into the same names the image uses for its own defaults, so the
# values are snapshotted as soon as the file has been sourced. The language
# script is appended to this one and assigns those defaults afterwards (the
# Python image asks for 1GB), with no way of knowing whether the judge sent a
# value; the judge's value is the exercise's, so it wins — see limit_fs().
monitor_max_fs=''
monitor_max_ms=''
compilation_timeout=''
src_file=''
run_nonce=''
make_file=false
[[ -e "${container_config_file}" ]] && source "${container_config_file}"
judge_max_fs="${monitor_max_fs}"
judge_max_ms="${monitor_max_ms}"
judge_compilation_seconds="${compilation_timeout}"

# The image's own defaults, used when the judge sent nothing (an image can also
# be run by hand). The language script may override them.
monitor_max_fs=5242880   # 5MB
monitor_max_ms=268435456 #256MB
compilation_timeout=10

# The nonce authenticates the progress milestones, so nothing submitted may be
# able to read it: a Makefile submission (and anything it runs) can execute code
# during compilation. Everything above is already in shell variables, so the file
# goes away before a single line of the submission is compiled or run.
[[ -e "${container_config_file}" ]] && rm -f "${container_config_file}"

# A value the judge could not quote safely would be a shell injection into this
# script, which runs as the container's root.
case "${src_file}" in
    *$'\n'* | *$'\r'*)
        echo "runcodes: src_file in container.config contains a line break"
        exit 2
        ;;
esac

# Messages sent for notifying of current status. Each is printed with the run's
# nonce appended when the judge supplied one, and the judge only accepts a
# milestone that carries it, so a submission cannot fake progress by printing the
# bare message from the program under test.
compilation_start_msg='compilation.start'
compilation_done_msg='compilation.done'
run_start_msg='run.start'
run_done_msg='run.done'

milestone() {
    if [ -n "${run_nonce}" ]; then
        echo "$1 ${run_nonce}"
    else
        echo "$1"
    fi
}

# Effective global limits: the judge's value when container.config carried one —
# an explicit 0 included, which means "no limit" — and the image's own default
# otherwise. They are resolved when the run happens, not when this script is
# read, because by then the language script's defaults are in place.
limit_fs() {
    printf '%s' "${judge_max_fs:-${monitor_max_fs}}"
}

limit_ms() {
    printf '%s' "${judge_max_ms:-${monitor_max_ms}}"
}

limit_compilation_seconds() {
    printf '%s' "${judge_compilation_seconds:-${compilation_timeout}}"
}

# Kills every process this script started that is still alive.
#
# The graded program runs once per test case and may leave processes behind —
# directly, or daemonised with a double fork. Anything of ours that survives could
# rewrite the results the judge reads next, so the sweep runs after each case (and
# after a Makefile has had its chance to run arbitrary code while "compiling").
#
# Only descendants of this script are killed. Inside the container that is every
# process except the container's init, but the sweep stays harmless if the image
# is ever run without a PID namespace of its own.
kill_stragglers() {
    local self=$$ pid ppid stat_line
    local -A parent=()
    local -A doomed=()
    local -a pids=()

    for pid_dir in /proc/[0-9]*; do
        pid="${pid_dir#/proc/}"
        # /proc/<pid>/stat is "pid (comm) state ppid ...", and comm may itself
        # contain spaces and parentheses, so the rest is read after the last ')'.
        stat_line="$(<"${pid_dir}/stat")" || continue
        stat_line="${stat_line##*)}"
        # shellcheck disable=SC2086
        set -- ${stat_line}
        ppid="${2:-}"
        [ -z "${ppid}" ] && continue
        parent["${pid}"]="${ppid}"
        pids+=("${pid}")
    done

    doomed["${self}"]=1

    # Walk down the tree, marking children of anything already doomed.
    local changed=1
    while [ "${changed}" = 1 ]; do
        changed=0
        for pid in "${pids[@]}"; do
            [ -n "${doomed[${pid}]:-}" ] && continue
            if [ -n "${doomed[${parent[${pid}]}]:-}" ]; then
                doomed["${pid}"]=1
                changed=1
            fi
        done
    done

    for pid in "${pids[@]}"; do
        [ "${pid}" = "${self}" ] && continue
        [ -n "${doomed[${pid}]:-}" ] && kill -KILL "${pid}" 2>/dev/null
    done

    return 0
}

compile() {
    compilation_command=$1
    pre_compilation_command=$2
    post_compilation_command=$3

    milestone "${compilation_start_msg}"

    # Save working dir to return to it later
    wd="$(pwd)"
    cd ./src || exit

    [ -z "${pre_compilation_command}" ] || eval "${pre_compilation_command}"

    "${monitor_bin}" -f "$(limit_fs)" \
        -m "$(limit_ms)" \
        -o "../${compilation_output}" \
        -e "../${compilation_error}" \
        -r "../${compilation_monitor_output}" \
        -c "timeout --signal=SIGKILL $(limit_compilation_seconds) ${compilation_command}" \
        2>"../${compilation_monitor_error}"

    [ -z "${post_compilation_command}" ] || eval "${post_compilation_command}"

    # Back to original working dir
    cd "${wd}" || exit

    # A Makefile can run arbitrary code while "compiling", so the compilation
    # phase is contained exactly like a test case.
    kill_stragglers

    cp "${compilation_monitor_output}" "${outputfiles_dir}"
    cp "${compilation_monitor_error}" "${outputfiles_dir}"
    cp "${compilation_output}" "${outputfiles_dir}"
    cp "${compilation_error}" "${outputfiles_dir}"

    milestone "${compilation_done_msg}"
}

run_tests() {
    run_command=$1
    pre_run_command=$2
    post_run_command=$3

    milestone "${run_start_msg}"

    # Save working dir to return to it later
    wd="$(pwd)"

    # The list of test directories is read up front: the straggler sweep below
    # kills every other process, and a `find` still feeding the loop would be one
    # of them.
    local test_dirs=()
    mapfile -t -d '' test_dirs < <(find . -maxdepth 1 -type d -name 'test_*' -print0)

    local test_dir test_id
    for test_dir in "${test_dirs[@]}"; do
        test_id="${test_dir#./test_}"
        cp -r --update=none ./src/* "${test_dir}"

        cd "${test_dir}" || exit

        # These should have a value set in ${container_config_file}; the judge
        # sends the exercise's per-case limits, and the image's globals are the
        # fallback.
        local timeout ms_limit fs_limit stack_limit var
        var="t_${test_id}"
        [ -z "${!var}" ] && timeout=3 || timeout="${!var}"
        var="ms_${test_id}"
        [ -z "${!var}" ] && ms_limit="$(limit_ms)" || ms_limit="${!var}"
        var="fs_${test_id}"
        [ -z "${!var}" ] && fs_limit="$(limit_fs)" || fs_limit="${!var}"
        var="stack_${test_id}"
        [ -z "${!var}" ] && stack_limit=0 || stack_limit="${!var}"

        [ -z "${pre_run_command}" ] || eval "${pre_run_command} ${test_id}"

        # Save output files outside of the test dir. The report is written by the
        # monitor itself (-r) so a file the program pre-created at that path is
        # replaced instead of being written through.
        "${monitor_bin}" -f "${fs_limit}" \
            -m "${ms_limit}" \
            -s "${stack_limit}" \
            -i "../${test_id}.in" \
            -o "../${test_id}.output" \
            -e "../${test_id}.error" \
            -r "../${test_id}.monitor_out" \
            -c "timeout --signal=SIGKILL ${timeout} ${run_command}" \
            2>"../${test_id}.monitor_err"

        [ -z "${post_run_command}" ] || eval "${post_run_command} ${test_id}"

        # Back to original working dir
        cd "${wd}" || exit

        # Nothing the graded program left behind may touch this case's results.
        kill_stragglers

        cp "${test_id}.monitor_out" "${outputfiles_dir}"
        cp "${test_id}.monitor_err" "${outputfiles_dir}"
        cp "${test_id}.output" "${outputfiles_dir}"
        cp "${test_id}.error" "${outputfiles_dir}"
    done

    milestone "${run_done_msg}"
}

# Make sure we have our directory for output files
mkdir -p "${outputfiles_dir}"

# Standard commands for a Makefile-based submission. This can change depending
# on the particular needs of each language.
if compgen -G "src/?akefile" >/dev/null; then
    make_file=true
    compilation_command='make all'
    run_command='make -s run'
fi
