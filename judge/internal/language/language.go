// Package language maps file extensions to the container image that runs them.
//
// This is a port of the legacy engine's `rcc/languages.py`. The new database
// schema dropped the per-language compile/run commands (the migration notes say
// "the new compiler engine configures itself"), so the mapping lives here.
package language

import (
	"fmt"
	"path"
	"strings"
)

// DefaultImageFormat is how a language's image name is expanded.
const DefaultImageFormat = "ghcr.io/runcodes-icmc/compiler-images-%s:latest"

// Language describes one supported language.
type Language struct {
	Name       string
	Extensions []string
	Compilable bool
	// ImageName is the image suffix (e.g. "cpp" for C++).
	ImageName string
}

// Image renders the language's container image with the given format.
func (l *Language) Image(format string) string {
	if format == "" {
		format = DefaultImageFormat
	}
	return fmt.Sprintf(format, l.ImageName)
}

// StandardExtension is the canonical extension of the language (first listed).
func (l *Language) StandardExtension() string {
	if len(l.Extensions) == 0 {
		return ""
	}
	return l.Extensions[0]
}

// KnownLanguages mirrors the legacy engine's list.
var KnownLanguages = []Language{
	{Name: "C", Extensions: []string{"c", "h"}, Compilable: true, ImageName: "c"},
	{Name: "C++", Extensions: []string{"cpp", "cc", "cxx", "c++", "hpp", "h"}, Compilable: true, ImageName: "cpp"},
	{Name: "C#", Extensions: []string{"cs"}, Compilable: true, ImageName: "dotnet"},
	{Name: "Fortran", Extensions: []string{"f", "f90", "f95", "f15", "f03"}, Compilable: true, ImageName: "fortran"},
	{Name: "Golang", Extensions: []string{"go"}, Compilable: true, ImageName: "go"},
	{Name: "Haskell", Extensions: []string{"hs", "lhs"}, Compilable: true, ImageName: "haskell"},
	{Name: "Java", Extensions: []string{"java", "jar", "class"}, Compilable: true, ImageName: "java"},
	{Name: "Octave", Extensions: []string{"m"}, Compilable: false, ImageName: "octave"},
	{Name: "Pascal", Extensions: []string{"pas", "pp", "pascal"}, Compilable: true, ImageName: "pascal"},
	{Name: "Portugol", Extensions: []string{"por"}, Compilable: true, ImageName: "portugol"},
	{Name: "Python", Extensions: []string{"py", "py3", "pyc"}, Compilable: false, ImageName: "python"},
	{Name: "R", Extensions: []string{"r"}, Compilable: false, ImageName: "r"},
	{Name: "Rust", Extensions: []string{"rs"}, Compilable: true, ImageName: "rust"},
	{Name: "Lua", Extensions: []string{"lua", "lol", "lu", "luac"}, Compilable: true, ImageName: "lua"},
	{Name: "Julia", Extensions: []string{"jl"}, Compilable: false, ImageName: "julia"},
	{Name: "Prolog", Extensions: []string{"pl", "pro", "prolog"}, Compilable: true, ImageName: "prolog"},
	{Name: "C (OpenMP)", Extensions: []string{"omp.c", "omp.h"}, Compilable: true, ImageName: "c-omp"},
	{Name: "C++ (OpenMP)", Extensions: []string{"omp.cpp", "omp.cc", "omp.cxx", "omp.c++", "omp.hpp", "omp.h"}, Compilable: true, ImageName: "cpp-omp"},
	{Name: "C (OpenMP + MPI)", Extensions: []string{"mpi.c", "mpi.h"}, Compilable: true, ImageName: "c-omp-mpi"},
	{Name: "C++ (OpenMP + MPI)", Extensions: []string{"mpi.cpp", "mpi.cc", "mpi.cxx", "mpi.c++", "mpi.hpp", "mpi.h"}, Compilable: true, ImageName: "cpp-omp-mpi"},
	{Name: "Verilog", Extensions: []string{"v", "vh"}, Compilable: false, ImageName: "verilog"},
	{Name: "Zig", Extensions: []string{"zig"}, Compilable: true, ImageName: "zig"},
}

// extensionIndex maps an extension to its language. Ambiguous extensions map to
// nil, exactly like the legacy engine.
var extensionIndex = buildIndex()

func buildIndex() map[string]*Language {
	idx := make(map[string]*Language)
	for i := range KnownLanguages {
		lang := &KnownLanguages[i]
		for _, ext := range lang.Extensions {
			if _, exists := idx[ext]; exists {
				idx[ext] = nil // ambiguous
			} else {
				idx[ext] = lang
			}
		}
	}
	return idx
}

// normalizeExtension extracts the "interesting" extension from a filename,
// handling the OpenMP/MPI compound extensions (`main.omp.c` -> `omp.c`).
func normalizeExtension(name string) string {
	ext := strings.ToLower(path.Ext(name))
	ext = strings.TrimPrefix(ext, ".")
	if ext == "" {
		return ""
	}
	base := strings.TrimSuffix(name, path.Ext(name))
	if isCompound(ext) && strings.Contains(base, ".") {
		pre := strings.ToLower(path.Ext(base))
		pre = strings.TrimPrefix(pre, ".")
		if pre == "omp" || pre == "mpi" {
			return pre + "." + ext
		}
	}
	return ext
}

func isCompound(ext string) bool {
	switch ext {
	case "c", "h", "cpp", "cc", "cxx", "c++", "hpp":
		return true
	}
	return false
}

// FromFilename returns the language for a file name, or nil if unknown or
// ambiguous. A nil language is not fatal: the run then uses the "zip"-style
// path (the images fall back to running the file as-is).
func FromFilename(fname string) *Language {
	return FromExtension(normalizeExtension(fname))
}

// FromExtension resolves a normalized extension.
func FromExtension(ext string) *Language {
	return extensionIndex[strings.ToLower(ext)]
}

// StandardizeExtension maps similar extensions onto one canonical name, as
// used when deducing the language inside a zip archive.
func StandardizeExtension(ext string) string {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	if ext == "zip" {
		return "zip"
	}
	if lang := FromExtension(ext); lang != nil {
		return lang.StandardExtension()
	}
	return ""
}

// IsCompilable reports whether the extension's language needs a compile step.
func IsCompilable(ext string) bool {
	lang := FromExtension(ext)
	return lang != nil && lang.Compilable
}

// DeduceFromArchive picks the most common recognizable extension in a set of
// file names (used for zip submissions).
func DeduceFromArchive(names []string) (string, error) {
	counts := make(map[string]int)
	for _, name := range names {
		ext := path.Ext(name)
		if ext == "" {
			continue
		}
		standard := StandardizeExtension(ext)
		if standard == "" {
			continue
		}
		counts[standard]++
	}
	if len(counts) == 0 {
		return "", fmt.Errorf("no files with known extensions found in archive")
	}
	best, bestCount := "", -1
	for ext, count := range counts {
		if count > bestCount || (count == bestCount && ext < best) {
			best, bestCount = ext, count
		}
	}
	return best, nil
}
