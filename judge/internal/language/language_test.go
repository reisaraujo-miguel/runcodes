package language

import "testing"

func TestFromFilename(t *testing.T) {
	cases := []struct {
		fname string
		want  string // expected language name, "" for nil
	}{
		{"main.c", "C"},
		{"main.cpp", "C++"},
		{"main.py", "Python"},
		{"main.omp.c", "C (OpenMP)"},
		{"main.mpi.cpp", "C++ (OpenMP + MPI)"},
		{"Main.PY", "Python"},
		{"main.xyz", ""},
	}
	for _, tc := range cases {
		got := FromFilename(tc.fname)
		name := ""
		if got != nil {
			name = got.Name
		}
		if name != tc.want {
			t.Errorf("FromFilename(%q) = %q, want %q", tc.fname, name, tc.want)
		}
	}
}

func TestIsCompilable(t *testing.T) {
	if !IsCompilable("c") {
		t.Error("C should be compilable")
	}
	if IsCompilable("py") {
		t.Error("Python should not be compilable")
	}
}

func TestDeduceFromArchive(t *testing.T) {
	got, err := DeduceFromArchive([]string{"src/main.py", "src/util.py", "README.md"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "py" {
		t.Fatalf("DeduceFromArchive = %q, want py", got)
	}
	if _, err := DeduceFromArchive([]string{"README.md"}); err == nil {
		t.Fatal("expected an error when no known extensions are present")
	}
}

func TestImageFormat(t *testing.T) {
	lang := FromFilename("main.cpp")
	if lang == nil {
		t.Fatal("cpp not found")
	}
	if got := lang.Image(DefaultImageFormat); got != "ghcr.io/runcodes-icmc/compiler-images-cpp:latest" {
		t.Fatalf("unexpected image: %s", got)
	}
}
