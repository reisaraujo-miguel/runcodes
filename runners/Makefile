IMAGE_PREFIX ?= ghcr.io/runcodes-icmc/runcodes-runner
MONITOR_IMAGE ?= ghcr.io/runcodes-icmc/runcodes-monitor

BUILD ?= podman build
RUNNER_BASE_ARG = --build-arg RUNNER_BASE_IMAGE=$(IMAGE_PREFIX)-base:latest
MONITOR_ARG = --build-arg MONITOR_IMAGE=$(MONITOR_IMAGE):latest

build-base:
	$(BUILD) $(MONITOR_ARG) -t $(IMAGE_PREFIX)-base:latest -f ./base/Dockerfile ./base --no-cache

build-%:
	$(BUILD) $(RUNNER_BASE_ARG) -t $(IMAGE_PREFIX)-$*:latest -f ./$*/Dockerfile ./$* --no-cache

build-cached-%:
	$(BUILD) $(RUNNER_BASE_ARG) -t $(IMAGE_PREFIX)-$*:latest -f ./$*/Dockerfile ./$*

build-cached-octave:
	$(BUILD) $(RUNNER_BASE_ARG) -t $(IMAGE_PREFIX)-octave:latest -f ./octave/Dockerfile ./octave --build-arg="MAKE_ARGS=-j24"


all: build-base build-c build-cpp build-fortran build-go build-haskell build-java build-pascal build-portugol build-python build-r build-rust build-octave build-dotnet build-lua build-julia build-c-omp build-c-omp-mpi build-cpp-omp build-cpp-omp-mpi build-prolog build-verilog build-zig

.PHONY: all
