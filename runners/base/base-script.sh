#!/bin/bash

# Configuration variables shared by all containers
container_config_file=container.config
compilation_monitor_error=compilation.monitor_err
compilation_monitor_output=compilation.monitor_out
compilation_error=compilation.err
compilation_output=compilation.out
compilation_warnings=compilation.warnings
outputfiles_dir=outputfiles
monitor_bin=/usr/bin/monitor
user_bin=executable

# Variables we expect to be inside ${container_config_file}:
# monitor_max_fs, monitor_max_ms, compilation_timeout, src_file, run_nonce and the
# per-case t_<id> (seconds), ms_<id> (bytes), fs_<id> (bytes) and stack_<id>
# (bytes).
#
# They are read into the same names and snapshotted immediately: the values are
# the judge's, resolved from the platform, the exercise and the test case, and
# the images carry no defaults of their own to fall back to.
monitor_max_fs=''
monitor_max_ms=''
compilation_timeout=''
src_file=''
run_nonce=''
[[ -e "${container_config_file}" ]] && source "${container_config_file}"
judge_max_fs="${monitor_max_fs}"
judge_max_ms="${monitor_max_ms}"
judge_compilation_seconds="${compilation_timeout}"

# The nonce authenticates the progress milestones, so nothing submitted may be
# able to read it: a Makefile submission (and anything it runs) can execute code
# during compilation. Everything above is already in shell variables, so the file
# goes away before a single line of the submission is compiled or run.
[[ -e "${container_config_file}" ]] && rm -f "${container_config_file}"

# A value the judge could not quote safely would be a shell injection into this
# script: it is sourced, so the injected text would run as the image's user
# (see the Dockerfile) with everything the harness itself may do.
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

# The limits are the judge's. The images used to carry a default for their own
# language, which made the same exercise behave differently depending on the
# image; that policy now lives in the platform and the judge sends every value
# on every run. A missing global is a broken invocation, not a default to guess.
for required in monitor_max_fs monitor_max_ms compilation_timeout; do
    if [ -z "${!required}" ]; then
        echo "runcodes: container.config is missing ${required}" >&2
        exit 2
    fi
done

limit_fs() {
    printf '%s' "${judge_max_fs}"
}

limit_ms() {
    printf '%s' "${judge_max_ms}"
}

limit_compilation_seconds() {
    printf '%s' "${judge_compilation_seconds}"
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
    local compilation_command=$1
    local pre_compilation_command=$2
    local post_compilation_command=$3
    local compile_status compile_signal

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
    # phase is contained exactly like a test case. Stop any leftover process
    # before reading or copying the reports it could still modify.
    kill_stragglers

    # The monitor's exit status is the compiler's exit status. A non-zero status
    # is a compilation failure even when the compiler wrote nothing to stderr
    # (a killed compiler or `timeout` are the usual cases), so make sure the
    # error file is non-empty for the language scripts' existing check. On
    # success, stderr is only diagnostics: preserve it as warnings so a compiler
    # that warns about otherwise valid code does not fail the submission.
    compile_status="$(sed -n 's/^exit_status=//p' "${compilation_monitor_output}" 2>/dev/null | tail -n 1)"
    compile_signal="$(sed -n 's/^signal=//p' "${compilation_monitor_output}" 2>/dev/null | tail -n 1)"
    if [ -z "${compile_status}" ]; then
        if [ ! -s "${compilation_error}" ]; then
            printf 'compilation failed: the monitor did not produce a report\n' >"${compilation_error}"
        fi
    elif [ "${compile_status}" != "0" ]; then
        if [ ! -s "${compilation_error}" ]; then
            if [ -n "${compile_signal}" ]; then
                printf 'compilation failed: killed by signal %s\n' "${compile_signal}" >"${compilation_error}"
            else
                printf 'compilation failed: exit status %s\n' "${compile_status}" >"${compilation_error}"
            fi
        fi
    elif [ -s "${compilation_error}" ]; then
        cp "${compilation_error}" "${compilation_warnings}"
        : >"${compilation_error}"
    fi

    cp "${compilation_monitor_output}" "${outputfiles_dir}"
    cp "${compilation_monitor_error}" "${outputfiles_dir}"
    cp "${compilation_output}" "${outputfiles_dir}"
    cp "${compilation_error}" "${outputfiles_dir}"
    [ ! -e "${compilation_warnings}" ] || cp "${compilation_warnings}" "${outputfiles_dir}"

    milestone "${compilation_done_msg}"
}

run_tests() {
    run_command=$1
    pre_run_command=$2
    post_run_command=$3

    milestone "${run_start_msg}"

    # Save working dir to return to it later
    wd="$(pwd)"

    local test_dir test_id
    # The glob is expanded before the loop starts, so the straggler sweep below
    # cannot kill a find that is still feeding the loop. Unlike `find`/`mapfile`,
    # a glob works in the older third-party bases (the Verilog image, for one).
    for test_dir in ./test_*; do
        [ -d "${test_dir}" ] || continue
        test_id="${test_dir#./test_}"
        # `-n` is the portable no-clobber form; the newer `--update=none`
        # spelling is not available in every third-party runner base (the
        # Verilog image, for example, ships an older coreutils).
        cp -r -n ./src/* "${test_dir}"

        cd "${test_dir}" || exit

        # The judge sends all four limits for every case; a missing byte limit
        # means "no limit", and a missing CPU budget is an error rather than a
        # guessed timeout.
        local timeout ms_limit fs_limit stack_limit var
        var="t_${test_id}"
        timeout="${!var}"
        if [ -z "${timeout}" ]; then
            echo "runcodes: container.config is missing t_${test_id}" >&2
            exit 2
        fi
        var="ms_${test_id}"
        ms_limit="${!var:-0}"
        var="fs_${test_id}"
        fs_limit="${!var:-0}"
        var="stack_${test_id}"
        stack_limit="${!var:-0}"

        # Files the program creates or changes are collected into the artifact
        # zip after the case: the timestamp marker tells them apart from the
        # copy of the source the harness placed here.
        : > "../${test_id}.start"

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

        # Collect what the program wrote, preserving its relative paths, so the
        # student can download the files the submission produced. The files are
        # moved, not copied (the scratch directory is discarded next, and a
        # second copy would double the workspace charge).
        #
        # There is no size filter here: the case's file-size limit is already a
        # kernel limit (RLIMIT_FSIZE, set by the monitor), so a program cannot
        # write a file past it, and a case with no limit is bounded by the
        # workspace quota instead. The judge uploads this directory as soon as
        # the case reports `case.done` and deletes the local copy, so it is only
        # ever on disk for one case.
        #
        # The directory is created even when the program wrote nothing: the judge
        # reads its existence as "this image collects generated files". An image
        # from before that was introduced leaves no directory at all, and the
        # judge can then say so instead of silently omitting the downloads.
        case_files_dir="${outputfiles_dir}/case_${test_id}"
        mkdir -p "${case_files_dir}"
        # The walk is NUL-delimited: the program picks its own filenames, and a
        # newline in one would otherwise split it into two paths and drop the
        # file.
        while IFS= read -r -d '' produced; do
            case_relative="${produced#"${test_dir}"/}"
            mkdir -p "${case_files_dir}/$(dirname "${case_relative}")"
            mv -f "${produced}" "${case_files_dir}/${case_relative}"
        done < <(find "${test_dir}" -type f -newer "${test_id}.start" -print0)
        rm -f -- "${test_id}.start"

        cp "${test_id}.monitor_out" "${outputfiles_dir}"
        cp "${test_id}.monitor_err" "${outputfiles_dir}"
        cp "${test_id}.output" "${outputfiles_dir}"
        cp "${test_id}.error" "${outputfiles_dir}"

        # The case's artifacts are complete: the judge grades and publishes this
        # one now, instead of waiting for the whole suite to finish.
        case_done_msg="case.done ${test_id}"
        milestone "${case_done_msg}"

        # Free the case's scratch directory: the copy of the source and every
        # file the program wrote there. The artifacts the judge needs (output,
        # error and the monitor report) live at the workspace root, and the
        # outputfiles copies are already made, so nothing grading depends on is
        # removed. Without this a suite of cases that write files accumulates
        # them until the workspace quota kills the run.
        rm -rf -- "./test_${test_id}"
    done

    milestone "${run_done_msg}"
}

# Make sure we have our directory for output files
mkdir -p "${outputfiles_dir}"

# Standard commands for a Makefile-based submission. This can change depending
# on the particular needs of each language.
if compgen -G "src/?akefile" >/dev/null; then
    compilation_command='make all'
    run_command='make -s run'
fi
