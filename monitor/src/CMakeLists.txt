cmake_minimum_required(VERSION 3.31)
project(monitor C)

set(CMAKE_C_STANDARD 23)
ADD_COMPILE_OPTIONS(-Wall -O2 -static)

set(SOURCES monitor.c monitor.h)

add_executable(monitor main.c ${SOURCES})
