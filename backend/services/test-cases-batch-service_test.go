package services

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"
)

// zipWith builds an in-memory zip from a name→content map.
func zipWith(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create %q: %v", name, err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("write %q: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buffer.Bytes()
}

func TestParseBatchTestCaseArchiveAcceptsConventionalFolders(t *testing.T) {
	archive := zipWith(t, map[string]string{
		"caso1/input.in":            "1 2\n",
		"caso1/output.out":          "3\n",
		"caso1/extra.txt":           "material",
		"caso2/entrada.in":          "5\n",
		"caso2/saida.out":           "10\n",
		"caso2/files/x.in":          "extra com nome ambiguo",
		"__MACOSX/caso1/._input.in": "metadata",
	})

	cases, err := ParseBatchTestCaseArchive(archive)
	if err != nil {
		t.Fatalf("ParseBatchTestCaseArchive: %v", err)
	}
	if len(cases) != 2 {
		t.Fatalf("got %d cases, want 2", len(cases))
	}
	if cases[0].Folder != "caso1" || cases[0].InputName != "input.in" {
		t.Fatalf("unexpected first case: %+v", cases[0])
	}
	if string(cases[0].InputData) != "1 2\n" || string(cases[0].OutputData) != "3\n" {
		t.Fatalf("unexpected contents: %+v", cases[0])
	}
	if len(cases[0].ExtraFiles) != 1 || cases[0].ExtraFiles[0].Name != "extra.txt" {
		t.Fatalf("unexpected extras: %+v", cases[0].ExtraFiles)
	}
	if len(cases[1].ExtraFiles) != 1 || cases[1].ExtraFiles[0].Name != "x.in" {
		t.Fatalf("files/ extra not flattened: %+v", cases[1].ExtraFiles)
	}
}

func TestParseBatchTestCaseArchiveReportsTheBrokenFolder(t *testing.T) {
	tests := []struct {
		name       string
		files      map[string]string
		wantFolder string
		wantReason string
	}{
		{
			name: "missing output",
			files: map[string]string{
				"bom/input.in": "ok",
			},
			wantFolder: "bom",
			wantReason: ".out",
		},
		{
			name: "two inputs",
			files: map[string]string{
				"bom/a.in":    "1",
				"bom/b.in":    "2",
				"bom/out.out": "3",
			},
			wantFolder: "bom",
			wantReason: "mais de um arquivo .in",
		},
		{
			name: "file at root",
			files: map[string]string{
				"soltinho.txt": "x",
			},
			wantFolder: "soltinho.txt",
			wantReason: "raiz",
		},
		{
			name: "nested folder",
			files: map[string]string{
				"bom/sub/input.in": "1",
			},
			wantFolder: "bom",
			wantReason: "estrutura",
		},
		{
			name: "no case folders",
			files: map[string]string{
				".DS_Store": "metadata",
			},
			wantFolder: "(raiz do arquivo)",
			wantReason: "nenhuma pasta",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseBatchTestCaseArchive(zipWith(t, test.files))
			if err == nil {
				t.Fatal("expected an error")
			}

			var batchErr *BatchImportError
			if !errors.As(err, &batchErr) {
				t.Fatalf("err = %v, want *BatchImportError", err)
			}
			found := false
			for _, issue := range batchErr.Issues {
				if issue.Folder == test.wantFolder &&
					strings.Contains(issue.Reason, test.wantReason) {
					found = true
				}
			}
			if !found {
				t.Fatalf("issues = %+v, want folder %q with reason %q",
					batchErr.Issues, test.wantFolder, test.wantReason)
			}
		})
	}
}

func TestParseBatchTestCaseArchiveRejectsInvalidInput(t *testing.T) {
	if _, err := ParseBatchTestCaseArchive(nil); !errors.Is(err, ErrInvalidArchive) {
		t.Fatalf("empty archive err = %v, want ErrInvalidArchive", err)
	}
	if _, err := ParseBatchTestCaseArchive([]byte("not a zip")); !errors.Is(err, ErrInvalidArchive) {
		t.Fatalf("garbage archive err = %v, want ErrInvalidArchive", err)
	}

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.Create("../escape/input.in")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = ParseBatchTestCaseArchive(buffer.Bytes())
	var batchErr *BatchImportError
	if !errors.As(err, &batchErr) {
		t.Fatalf("traversal err = %v, want *BatchImportError", err)
	}
}

func TestValidateBatchConfig(t *testing.T) {
	config := BatchTestCaseConfig{}
	if err := validateBatchConfig(&config); err != nil {
		t.Fatalf("default config: %v", err)
	}
	if config.InputType != IOTypeText || config.ExpectedOutputType != IOTypeText {
		t.Fatalf("defaults = %+v", config)
	}

	config = BatchTestCaseConfig{
		InputType:          IOTypeText,
		ExpectedOutputType: IOTypeFile,
		MemUsageLimitBytes: -1,
	}
	if err := validateBatchConfig(&config); !errors.Is(err, ErrInvalidTestCase) {
		t.Fatalf("negative limit err = %v, want ErrInvalidTestCase", err)
	}
}
