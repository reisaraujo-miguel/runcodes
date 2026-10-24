package services

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func validOfferingManifest() *offeringArchiveManifest {
	return &offeringArchiveManifest{
		Format:     offeringArchiveFormat,
		Version:    offeringArchiveVersion,
		ExportedAt: time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC),
		Offering: offeringArchiveOffering{
			Name:            "Algoritmos",
			Description:     "Turma de teste",
			EndDate:         "2026-12-20T23:59:59-03:00",
			VisibleToEnroll: true,
		},
		Exercises: []offeringArchiveExercise{
			{
				Title:                     "Soma",
				Description:               "Some dois números",
				Deadline:                  time.Date(2026, 12, 1, 23, 59, 0, 0, time.UTC),
				OpenDate:                  time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
				AllowedFileTypeExtensions: []string{"py"},
				TestCases: []offeringArchiveTestCase{
					{
						Input:               "1 2",
						InputType:           IOTypeText,
						ShowInput:           true,
						ExpectedOutput:      "3",
						ExpectedOutputType:  IOTypeText,
						ShowExpectedOutput:  true,
						ShowUserOutput:      true,
						CPUTimeLimitSeconds: 2,
						Files:               []string{"extra.txt"},
					},
				},
				CompilationFiles: []string{"main.c"},
				AttachedFiles:    []string{"enunciado.pdf"},
			},
		},
	}
}

func offeringArchiveFiles(t *testing.T, manifest *offeringArchiveManifest) map[string]string {
	t.Helper()

	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	return map[string]string{
		"manifest.json":                               string(encoded),
		"exercises/001/testcases/001/in":              "1 2",
		"exercises/001/testcases/001/out":             "3",
		"exercises/001/testcases/001/files/extra.txt": "extra",
		"exercises/001/compilation/main.c":            "int main(){}",
		"exercises/001/attached/enunciado.pdf":        "%PDF",
	}
}

func TestParseOfferingArchiveAcceptsExport(t *testing.T) {
	archive, err := ParseOfferingArchive(
		zipWith(t, offeringArchiveFiles(t, validOfferingManifest())),
	)
	if err != nil {
		t.Fatalf("ParseOfferingArchive: %v", err)
	}
	if archive.manifest.Offering.Name != "Algoritmos" {
		t.Fatalf("offering = %+v", archive.manifest.Offering)
	}
	if len(archive.manifest.Exercises) != 1 ||
		len(archive.manifest.Exercises[0].TestCases) != 1 {
		t.Fatalf("manifest = %+v", archive.manifest)
	}
}

func TestParseOfferingArchiveRejectsBrokenArchives(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*offeringArchiveManifest, map[string]string)
		wantErr error
	}{
		{
			name: "missing manifest",
			mutate: func(_ *offeringArchiveManifest, files map[string]string) {
				delete(files, "manifest.json")
			},
			wantErr: ErrInvalidTransfer,
		},
		{
			name: "unknown format",
			mutate: func(manifest *offeringArchiveManifest, _ map[string]string) {
				manifest.Format = "something-else"
			},
			wantErr: ErrInvalidTransfer,
		},
		{
			name: "unsupported version",
			mutate: func(manifest *offeringArchiveManifest, _ map[string]string) {
				manifest.Version = 99
			},
			wantErr: ErrInvalidTransfer,
		},
		{
			name: "missing case output",
			mutate: func(_ *offeringArchiveManifest, files map[string]string) {
				delete(files, "exercises/001/testcases/001/out")
			},
			wantErr: ErrInvalidTransfer,
		},
		{
			name: "missing compilation file",
			mutate: func(_ *offeringArchiveManifest, files map[string]string) {
				delete(files, "exercises/001/compilation/main.c")
			},
			wantErr: ErrInvalidTransfer,
		},
		{
			name: "invalid input type",
			mutate: func(manifest *offeringArchiveManifest, _ map[string]string) {
				manifest.Exercises[0].TestCases[0].InputType = "binary"
			},
			wantErr: ErrInvalidTransfer,
		},
		{
			name: "exercise closes before opening",
			mutate: func(manifest *offeringArchiveManifest, _ map[string]string) {
				manifest.Exercises[0].Deadline = manifest.Exercises[0].OpenDate.Add(-time.Hour)
			},
			wantErr: ErrInvalidTransfer,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := validOfferingManifest()
			files := offeringArchiveFiles(t, manifest)
			test.mutate(manifest, files)
			if test.name != "missing manifest" {
				encoded, err := json.Marshal(manifest)
				if err != nil {
					t.Fatal(err)
				}
				files["manifest.json"] = string(encoded)
			}

			_, err := ParseOfferingArchive(zipWith(t, files))
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("err = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestParseOfferingArchiveRejectsUnsafeFileNames(t *testing.T) {
	manifest := validOfferingManifest()
	manifest.Exercises[0].AttachedFiles = []string{"../segredo.pdf"}
	files := offeringArchiveFiles(t, manifest)
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	files["manifest.json"] = string(encoded)

	_, err = ParseOfferingArchive(zipWith(t, files))
	if !errors.Is(err, ErrInvalidTransfer) {
		t.Fatalf("err = %v, want ErrInvalidTransfer", err)
	}
	if !strings.Contains(err.Error(), "plain file name") {
		t.Fatalf("err = %v", err)
	}
}
