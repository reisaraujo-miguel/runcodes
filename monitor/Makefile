BUILD ?= podman build

build:
	echo "Warning: This make file runs the container build, for building locally go inside the src directory!"
	$(BUILD) -t ghcr.io/runcodes-icmc/runcodes-monitor:latest -f ./Dockerfile .

all: build

.PHONY: all
