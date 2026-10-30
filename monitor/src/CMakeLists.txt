cmake_minimum_required(VERSION 3.31)
project(monitor C)

set(CMAKE_C_STANDARD 23)
ADD_COMPILE_OPTIONS(-Wall -O2)

set(SOURCES monitor.c monitor.h)

add_executable(monitor main.c ${SOURCES})

# -static only affects linking, so it has to be a link option: as a compile
# option it silently produced a dynamically linked binary. The monitor image and
# every runner image copy this binary between bases, so it must not depend on the
# builder's libc being the same as the runner's.
target_link_options(monitor PRIVATE -static)
