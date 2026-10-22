# Each case compiles the submission together with that case's testbench. The
# pre-run hook is called with the case id as its first argument, so the
# expansion happens inside the function rather than in the eval string.
compile_verilog() {
    iverilog -o "${user_bin}" "${src_file}" "../$1.in"
}

if ! compgen -G "src/?akefile" >/dev/null; then
    pre_run_command=compile_verilog
    run_command="./${user_bin}"
fi

run_tests "$run_command" "$pre_run_command"
