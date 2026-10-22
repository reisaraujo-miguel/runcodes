---
title: Languages & images
description: Supported languages, file extensions, image names and compile behaviour.
---

# Languages & images

The judge derives a submission's language from the filename extension and runs
the matching container image. The mapping lives in `judge/internal/language`;
the images live in `runners/`.

## Extension matrix

| Language | Recognised extensions | Image suffix | Compiles |
| --- | --- | --- | :-: |
| C | `c`, `h`* | `c` | ✓ |
| C++ | `cpp`, `cc`, `cxx`, `c++`, `hpp`, `h`* | `cpp` | ✓ |
| C# | `cs` | `dotnet` | ✓ |
| Fortran | `f`, `f90`, `f95`, `f15`, `f03` | `fortran` | ✓ |
| Go | `go` | `go` | ✓ |
| Haskell | `hs`, `lhs` | `haskell` | ✓ |
| Java | `java`, `jar`, `class` | `java` | ✓ |
| Julia | `jl` | `julia` | — |
| Lua | `lua`, `lol`, `lu`, `luac` | `lua` | ✓ |
| Octave | `m` | `octave` | — |
| Pascal | `pas`, `pp`, `pascal` | `pascal` | ✓ |
| Portugol | `por` | `portugol` | ✓ |
| Prolog | `pl`, `pro`, `prolog` | `prolog` | ✓ |
| Python | `py`, `py3`, `pyc` | `python` | — |
| R | `r` | `r` | — |
| Rust | `rs` | `rust` | ✓ |
| Verilog | `v`, `vh` | `verilog` | — |
| Zig | `zig` | `zig` | ✓ |
| C (OpenMP) | `omp.c`, `omp.h` | `c-omp` | ✓ |
| C++ (OpenMP) | `omp.cpp`, `omp.cc`, `omp.cxx`, `omp.c++`, `omp.hpp`, `omp.h` | `cpp-omp` | ✓ |
| C (OpenMP + MPI) | `mpi.c`, `mpi.h` | `c-omp-mpi` | ✓ |
| C++ (OpenMP + MPI) | `mpi.cpp`, `mpi.cc`, `mpi.cxx`, `mpi.c++`, `mpi.hpp`, `mpi.h` | `cpp-omp-mpi` | ✓ |

\* `h` is deliberately ambiguous between C and C++ in the extension index. It
is not resolved to either language; a `.h` submission takes the same fallback
path as an unknown extension.

## Image names

The default template is:

```text
ghcr.io/runcodes-icmc/runcodes-runner-%s:latest
```

`JUDGE_IMAGE_FORMAT` overrides it. The `%s` is replaced with the image suffix
from the table, for example:

```text
ghcr.io/runcodes-icmc/runcodes-runner-cpp-omp:latest
```

The runner images inherit from `ghcr.io/runcodes-icmc/runcodes-runner-base`,
which contains `/usr/bin/runcodes` and `/usr/bin/monitor`. See
[Runner images & monitor](../services/runners-and-monitor.md).

## How a language is selected

1. The filename is lowercased.
2. The last extension is extracted (`main.omp.c` → `c`).
3. If the extension is one of the compound-capable ones (`c`, `h`, `cpp`, `cc`,
   `cxx`, `c++`, `hpp`) and the previous extension is `omp` or `mpi`, the
   compound extension (`omp.c`, `mpi.cpp`, …) is used.
4. The extension index is consulted. If it maps to a language, that language's
   image is used. If it is unknown or ambiguous (for example `h`), the run uses
   the fallback path: the image receives the file as-is and the language script's
   behaviour applies.

For zip submissions, the judge inspects the archive and standardises extensions
to pick a language; it then passes a matching file from the archive to the image
as `src_file` (a top-level file first, then the shortest path, then the
lexicographically smallest name). An archive with a top-level Makefile still
uses the harness's `make` path. `zip` remains its own case.

## Compilable vs interpreted

`Compilable` tells the harness whether to emit `compilation.start` /
`compilation.done` milestones. Interpreted languages (Python, R, Julia, Octave,
Verilog) skip the compilation phase. Lua, Prolog and Portugol are marked
compilable because their images run a compile/validation step.

This flag does not mean the language is unavailable without a compiler; it only
controls the milestone sequence the judge expects.

## Adding a language

A new language needs:

1. a directory in `runners/<name>/` with a Dockerfile and script;
2. an entry in the judge's `KnownLanguages` (name, extensions, compilable flag,
   image suffix);
3. an entry in `runners.yml`'s matrix and in
   `config_runcodes_runner_languages` for pre-pulling;
4. if it has an extension allowlist entry, a seed row in
   `database/schema/03-seed-data.sql`;
5. a rebuild/release of the base and all language images (the harness contract
   is shared) and a judge release if the mapping changed.

See [Runner images & monitor § Changing the contract](../services/runners-and-monitor.md#changing-the-contract).

## Related pages

- [Runner images & monitor](../services/runners-and-monitor.md)
- [Judge § Container contract](../services/judge.md#container-contract)
- [CI/CD & releases](../ci-cd.md)
