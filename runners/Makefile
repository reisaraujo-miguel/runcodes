IMAGE_PREFIX ?= ghcr.io/runcodes-icmc/runcodes-runner

build-base:
	docker build -t ${IMAGE_PREFIX}-base:latest -f ./base/Dockerfile ./base --no-cache

build-%:
	docker build -t ${IMAGE_PREFIX}-$*:latest -f ./$*/Dockerfile ./$* --no-cache

build-cached-%:
	docker build -t ${IMAGE_PREFIX}-$*:latest -f ./$*/Dockerfile ./$*

build-cached-octave:
	docker build -t ${IMAGE_PREFIX}-octave:latest -f ./octave/Dockerfile ./octave --build-arg="MAKE_ARGS=-j24"

podman-build-base:
	podman build -t ${IMAGE_PREFIX}-base:latest -f ./base/Dockerfile ./base --no-cache

podman-build-%:
	podman build -t ${IMAGE_PREFIX}-$*:latest -f ./$*/Dockerfile ./$* --no-cache

podman-build-cached-%:
	podman build -t ${IMAGE_PREFIX}-$*:latest -f ./$*/Dockerfile ./$*

podman-build-cached-octave:
	podman build -t ${IMAGE_PREFIX}-octave:latest -f ./octave/Dockerfile ./octave --build-arg="MAKE_ARGS=-j24"


all: build-base build-c build-cpp build-fortran build-go build-haskell build-java build-pascal build-portugol build-python build-r build-rust build-octave build-dotnet build-lua build-julia build-c-omp build-c-omp-mpi build-cpp-omp build-cpp-omp-mpi build-prolog build-verilog build-zig

podman-all: podman-build-base podman-build-c podman-build-cpp podman-build-fortran podman-build-go podman-build-haskell podman-build-java podman-build-pascal podman-build-portugol podman-build-python podman-build-r podman-build-rust podman-build-octave podman-build-dotnet podman-build-lua podman-build-julia podman-build-c-omp podman-build-c-omp-mpi podman-build-cpp-omp podman-build-cpp-omp-mpi podman-build-prolog podman-build-verilog podman-build-zig

.PHONY: all
