IMAGE_PREFIX ?= ghcr.io/runcodes-icmc/runcodes-runner

BUILD ?= podman build

build-base:
	$(BUILD) -t ${IMAGE_PREFIX}-base:latest -f ./base/Dockerfile ./base --no-cache

build-%:
	$(BUILD) -t ${IMAGE_PREFIX}-$*:latest -f ./$*/Dockerfile ./$* --no-cache

build-cached-%:
	$(BUILD) -t ${IMAGE_PREFIX}-$*:latest -f ./$*/Dockerfile ./$*

build-cached-octave:
	$(BUILD) -t ${IMAGE_PREFIX}-octave:latest -f ./octave/Dockerfile ./octave --build-arg="MAKE_ARGS=-j24"


all: build-base build-c build-cpp build-fortran build-go build-haskell build-java build-pascal build-portugol build-python build-r build-rust build-octave build-dotnet build-lua build-julia build-c-omp build-c-omp-mpi build-cpp-omp build-cpp-omp-mpi build-prolog build-verilog build-zig

.PHONY: all
