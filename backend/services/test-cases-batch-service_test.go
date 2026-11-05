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

	cases, ignored, err := ParseBatchTestCaseArchive(archive)
	if err != nil {
		t.Fatalf("ParseBatchTestCaseArchive: %v", err)
	}
	if len(ignored) != 0 {
		t.Fatalf("ignored = %v, want none", ignored)
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

func TestParseBatchTestCaseArchiveUnwrapsSingleRootFolder(t *testing.T) {
	archive := zipWith(t, map[string]string{
		"casos/1.in":      "um",
		"casos/1.out":     "dois",
		"casos/2.in":      "três",
		"casos/2.out":     "quatro",
		"casos/notas.txt": "rascunho",
		"LEIAME.md":       "instruções",
	})

	cases, ignored, err := ParseBatchTestCaseArchive(archive)
	if err != nil {
		t.Fatalf("ParseBatchTestCaseArchive: %v", err)
	}
	if len(cases) != 2 {
		t.Fatalf("got %d cases, want 2", len(cases))
	}
	if cases[0].Folder != "1" || cases[0].InputName != "1.in" ||
		string(cases[0].InputData) != "um" {
		t.Fatalf("unexpected first case: %+v", cases[0])
	}
	if cases[1].Folder != "2" || cases[1].OutputName != "2.out" {
		t.Fatalf("unexpected second case: %+v", cases[1])
	}
	want := []string{"LEIAME.md", "notas.txt"}
	if len(ignored) != len(want) || ignored[0] != want[0] || ignored[1] != want[1] {
		t.Fatalf("ignored = %v, want %v", ignored, want)
	}
}

func TestParseBatchTestCaseArchiveUnwrapsSingleFolderOfCases(t *testing.T) {
	archive := zipWith(t, map[string]string{
		"turma/caso_001/input.in":   "1",
		"turma/caso_001/output.out": "2",
		"turma/caso_001/extra.txt":  "material",
		"turma/caso_002/input.in":   "3",
		"turma/caso_002/output.out": "4",
		"turma/caso_002/files/x.in": "extra",
	})

	cases, ignored, err := ParseBatchTestCaseArchive(archive)
	if err != nil {
		t.Fatalf("ParseBatchTestCaseArchive: %v", err)
	}
	if len(ignored) != 0 {
		t.Fatalf("ignored = %v, want none", ignored)
	}
	if len(cases) != 2 {
		t.Fatalf("got %d cases, want 2", len(cases))
	}
	if cases[0].Folder != "caso_001" || len(cases[0].ExtraFiles) != 1 {
		t.Fatalf("unexpected first case: %+v", cases[0])
	}
	if cases[1].Folder != "caso_002" || len(cases[1].ExtraFiles) != 1 ||
		cases[1].ExtraFiles[0].Name != "x.in" {
		t.Fatalf("unexpected second case: %+v", cases[1])
	}
}

func TestParseBatchTestCaseArchiveUnwrapsMixedFolderLayout(t *testing.T) {
	archive := zipWith(t, map[string]string{
		"turma/1.in":        "um",
		"turma/1.out":       "dois",
		"turma/2/2.in":      "três",
		"turma/2/2.out":     "quatro",
		"turma/2/extra.txt": "material",
	})

	cases, _, err := ParseBatchTestCaseArchive(archive)
	if err != nil {
		t.Fatalf("ParseBatchTestCaseArchive: %v", err)
	}
	if len(cases) != 2 {
		t.Fatalf("got %d cases, want 2", len(cases))
	}
	if cases[0].Folder != "1" {
		t.Fatalf("unexpected root case: %+v", cases[0])
	}
	if cases[1].Folder != "2" || len(cases[1].ExtraFiles) != 1 {
		t.Fatalf("unexpected folder case: %+v", cases[1])
	}
}

func TestParseBatchTestCaseArchiveUnwrapsFolderWithOneCase(t *testing.T) {
	archive := zipWith(t, map[string]string{
		"turma/caso_001/input.in":   "1",
		"turma/caso_001/output.out": "2",
		"turma/caso_001/extra.txt":  "material",
	})

	cases, _, err := ParseBatchTestCaseArchive(archive)
	if err != nil {
		t.Fatalf("ParseBatchTestCaseArchive: %v", err)
	}
	if len(cases) != 1 {
		t.Fatalf("got %d cases, want 1", len(cases))
	}
	if cases[0].Folder != "caso_001" || len(cases[0].ExtraFiles) != 1 {
		t.Fatalf("unexpected case: %+v", cases[0])
	}
}

func TestParseBatchTestCaseArchiveKeepsSingleCaseFolderIntact(t *testing.T) {
	archive := zipWith(t, map[string]string{
		"caso_001/input.in":   "1",
		"caso_001/output.out": "2",
		"caso_001/extra.txt":  "material",
		"caso_001/files/x.in": "extra",
	})

	cases, ignored, err := ParseBatchTestCaseArchive(archive)
	if err != nil {
		t.Fatalf("ParseBatchTestCaseArchive: %v", err)
	}
	if len(ignored) != 0 {
		t.Fatalf("ignored = %v, want none", ignored)
	}
	if len(cases) != 1 {
		t.Fatalf("got %d cases, want 1", len(cases))
	}
	if cases[0].Folder != "caso_001" || len(cases[0].ExtraFiles) != 2 {
		t.Fatalf("single case folder lost its extras: %+v", cases[0])
	}
}

func TestParseBatchTestCaseArchivePairsRootFiles(t *testing.T) {
	archive := zipWith(t, map[string]string{
		"1.in":      "um",
		"1.out":     "dois",
		"2.in":      "três",
		"2.out":     "quatro",
		"README.md": "instruções",
		"notas.txt": "rascunho",
	})

	cases, ignored, err := ParseBatchTestCaseArchive(archive)
	if err != nil {
		t.Fatalf("ParseBatchTestCaseArchive: %v", err)
	}
	if len(cases) != 2 {
		t.Fatalf("got %d cases, want 2", len(cases))
	}
	if cases[0].Folder != "1" || cases[0].InputName != "1.in" ||
		string(cases[0].InputData) != "um" || string(cases[0].OutputData) != "dois" {
		t.Fatalf("unexpected first case: %+v", cases[0])
	}
	if cases[1].Folder != "2" || cases[1].InputName != "2.in" {
		t.Fatalf("unexpected second case: %+v", cases[1])
	}
	if len(cases[0].ExtraFiles) != 0 {
		t.Fatalf("root case should have no extras: %+v", cases[0].ExtraFiles)
	}
	want := []string{"README.md", "notas.txt"}
	if len(ignored) != len(want) || ignored[0] != want[0] || ignored[1] != want[1] {
		t.Fatalf("ignored = %v, want %v", ignored, want)
	}
}

func TestParseBatchTestCaseArchiveMixesRootPairsAndFolders(t *testing.T) {
	archive := zipWith(t, map[string]string{
		"1.in":         "um",
		"1.out":        "dois",
		"2/2.in":       "três",
		"2/2.out":      "quatro",
		"2/extra.txt":  "material",
		"3/entrada.in": "cinco",
		"3/saida.out":  "seis",
	})

	cases, ignored, err := ParseBatchTestCaseArchive(archive)
	if err != nil {
		t.Fatalf("ParseBatchTestCaseArchive: %v", err)
	}
	if len(ignored) != 0 {
		t.Fatalf("ignored = %v, want none", ignored)
	}
	if len(cases) != 3 {
		t.Fatalf("got %d cases, want 3", len(cases))
	}
	if cases[0].Folder != "1" || len(cases[0].ExtraFiles) != 0 {
		t.Fatalf("unexpected root case: %+v", cases[0])
	}
	if cases[1].Folder != "2" || len(cases[1].ExtraFiles) != 1 {
		t.Fatalf("unexpected folder case with extras: %+v", cases[1])
	}
	if cases[2].Folder != "3" || cases[2].InputName != "entrada.in" {
		t.Fatalf("unexpected folder case: %+v", cases[2])
	}
}

func TestParseBatchTestCaseArchiveReportsTheBrokenCase(t *testing.T) {
	tests := []struct {
		name       string
		files      map[string]string
		wantCase   string
		wantReason string
	}{
		{
			name: "folder missing output",
			files: map[string]string{
				"bom/input.in": "ok",
			},
			wantCase:   "bom",
			wantReason: ".out",
		},
		{
			name: "two inputs in a folder",
			files: map[string]string{
				"bom/a.in":    "1",
				"bom/b.in":    "2",
				"bom/out.out": "3",
			},
			wantCase:   "bom",
			wantReason: "mais de um arquivo .in",
		},
		{
			name: "root pair missing the output",
			files: map[string]string{
				"1.in": "1",
			},
			wantCase:   "1",
			wantReason: `falta o arquivo "1.out"`,
		},
		{
			name: "root pair missing the input",
			files: map[string]string{
				"1.out": "1",
			},
			wantCase:   "1",
			wantReason: `falta o arquivo "1.in"`,
		},
		{
			name: "two root inputs for the same case",
			files: map[string]string{
				"1.in":  "1",
				"1.IN":  "1",
				"1.out": "2",
			},
			wantCase:   "1",
			wantReason: "mais de um arquivo .in",
		},
		{
			name: "root pair and folder with the same name",
			files: map[string]string{
				"1.in":        "1",
				"1.out":       "2",
				"1/extra.txt": "material",
			},
			wantCase:   "1",
			wantReason: "duas vezes",
		},
		{
			name: "nested folder",
			files: map[string]string{
				"bom/sub/input.in": "1",
			},
			wantCase:   "bom",
			wantReason: "estrutura",
		},
		{
			name: "files/ without the case folder",
			files: map[string]string{
				"1.in":         "1",
				"1.out":        "2",
				"1/files/x.in": "extra",
			},
			wantCase:   "1",
			wantReason: "files/",
		},
		{
			name: "ignored root files only",
			files: map[string]string{
				"README.md": "instruções",
				".DS_Store": "metadata",
			},
			wantCase:   "(raiz do arquivo)",
			wantReason: "arquivos ignorados na raiz: README.md",
		},
		{
			name: "nothing at all",
			files: map[string]string{
				".DS_Store": "metadata",
			},
			wantCase:   "(raiz do arquivo)",
			wantReason: "nenhum caso",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := ParseBatchTestCaseArchive(zipWith(t, test.files))
			if err == nil {
				t.Fatal("expected an error")
			}

			var batchErr *BatchImportError
			if !errors.As(err, &batchErr) {
				t.Fatalf("err = %v, want *BatchImportError", err)
			}
			found := false
			for _, issue := range batchErr.Issues {
				if issue.Folder == test.wantCase &&
					strings.Contains(issue.Reason, test.wantReason) {
					found = true
				}
			}
			if !found {
				t.Fatalf("issues = %+v, want case %q with reason %q",
					batchErr.Issues, test.wantCase, test.wantReason)
			}
		})
	}
}

func TestParseBatchTestCaseArchiveRejectsInvalidInput(t *testing.T) {
	if _, _, err := ParseBatchTestCaseArchive(nil); !errors.Is(err, ErrInvalidArchive) {
		t.Fatalf("empty archive err = %v, want ErrInvalidArchive", err)
	}
	if _, _, err := ParseBatchTestCaseArchive([]byte("not a zip")); !errors.Is(err, ErrInvalidArchive) {
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

	_, _, err = ParseBatchTestCaseArchive(buffer.Bytes())
	var batchErr *BatchImportError
	if !errors.As(err, &batchErr) {
		t.Fatalf("traversal err = %v, want *BatchImportError", err)
	}
}

func TestBatchLimitOverrides(t *testing.T) {
	empty := batchLimitOverrides(BatchTestCaseConfig{})
	if !empty.Empty() {
		t.Fatalf("an absent limit set must inherit, got %+v", empty)
	}

	cpu := 2
	zero := int64(0)
	overrides := batchLimitOverrides(BatchTestCaseConfig{
		CPUTimeLimitSeconds: &cpu,
		MemUsageLimitBytes:  &zero,
	})
	if overrides.CPUTimeLimitSeconds == nil || *overrides.CPUTimeLimitSeconds != 2 {
		t.Fatalf("cpu = %+v, want 2", overrides.CPUTimeLimitSeconds)
	}
	if overrides.StackLimitBytes != nil {
		t.Fatalf("stack = %+v, want nil (inherit)", overrides.StackLimitBytes)
	}
	if overrides.MemUsageLimitBytes == nil || *overrides.MemUsageLimitBytes != 0 {
		t.Fatalf("memory = %+v, want an explicit 0 (no limit)", overrides.MemUsageLimitBytes)
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

	negative := int64(-1)
	config = BatchTestCaseConfig{
		InputType:          IOTypeText,
		ExpectedOutputType: IOTypeFile,
		MemUsageLimitBytes: &negative,
	}
	if err := validateBatchConfig(&config); !errors.Is(err, ErrInvalidTestCase) {
		t.Fatalf("negative limit err = %v, want ErrInvalidTestCase", err)
	}

	// An explicit 0 is meaningful now: it means "no limit" for byte limits and
	// must be accepted, while a zero CPU budget has no meaning.
	zero := int64(0)
	positive := 3
	config = BatchTestCaseConfig{
		CPUTimeLimitSeconds: &positive,
		MemUsageLimitBytes:  &zero,
		StackLimitBytes:     &zero,
		FileSizeLimitBytes:  &zero,
	}
	if err := validateBatchConfig(&config); err != nil {
		t.Fatalf("explicit zero byte limits: %v", err)
	}

	zeroCPU := 0
	config = BatchTestCaseConfig{CPUTimeLimitSeconds: &zeroCPU}
	if err := validateBatchConfig(&config); !errors.Is(err, ErrInvalidTestCase) {
		t.Fatalf("zero cpu err = %v, want ErrInvalidTestCase", err)
	}
}
