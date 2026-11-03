if ! compgen -G "src/?akefile" >/dev/null; then
    compilation_command=''
    run_command="julia ${src_file}"
fi

run_tests "$run_command"
