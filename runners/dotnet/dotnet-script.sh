if ! compgen -G "src/?akefile" >/dev/null; then
    pre_compilation_command="dotnet new console -o . --force &>/dev/null && rm Program.cs"
    compilation_command="dotnet build -c Release"
    post_compilation_command="grep -q \"FAILED\" ../${compilation_output} && cp ../${compilation_output} ../${compilation_error}"
    run_command="./bin/Release/net10.0/src"
fi

# Every limit (compilation memory, per-case memory and file size, workspace
# quota) comes from container.config, which the judge writes. The image sets no
# defaults of its own.

compile "${compilation_command}" "${pre_compilation_command}" "${post_compilation_command}"

# Bail if there are any compilation errors
if [ -s "${compilation_error}" ]; then
    exit 1
fi

run_tests "${run_command}"
