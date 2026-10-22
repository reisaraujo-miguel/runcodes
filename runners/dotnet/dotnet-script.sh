if ! compgen -G "src/?akefile" >/dev/null; then
    pre_compilation_command="dotnet new console -o . --force &>/dev/null && rm Program.cs"
    compilation_command="dotnet build -c Release"
    post_compilation_command="grep -q \"FAILED\" ../${compilation_output} && cp ../${compilation_output} ../${compilation_error}"
    run_command="./bin/Release/net10.0/src"
fi

# Leave memory to the judge's cgroup backstop, but bound a single file. The
# workspace itself has no aggregate quota, so an unbounded file would let a
# submission fill the host disk.
monitor_max_fs=268435456 # 256 MiB
monitor_max_ms=0
compilation_timeout=60

compile "${compilation_command}" "${pre_compilation_command}" "${post_compilation_command}"

# Bail if there are any compilation errors
if [ -s "${compilation_error}" ]; then
    exit 1
fi

run_tests "${run_command}"
