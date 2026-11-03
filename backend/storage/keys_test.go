package storage

import "testing"

func TestAuthoringKeys(t *testing.T) {
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"case input", CaseInputKey(7), "7/in"},
		{"case output", CaseOutputKey(7), "7/out"},
		{"case file", CaseFileKey(7, "data.txt"), "7/files/data.txt"},
		{"case file strips directories", CaseFileKey(7, "../../etc/passwd"), "7/files/passwd"},
		{"case file windows path", CaseFileKey(7, `a\b\input.txt`), "7/files/input.txt"},
		{"case file keeps spaces", CaseFileKey(7, "my input.dat"), "7/files/my input.dat"},
		{"case file empty fallback", CaseFileKey(7, ""), "7/files/file"},
		{"compilation file", CompilationFileKey(3, "helper.c"), "compilationfiles/3/helper.c"},
		{"compilation file traversal", CompilationFileKey(3, "../evil.c"), "compilationfiles/3/evil.c"},
		{"attachment", AttachmentKey(3, "statement.pdf"), "attachments/3/statement.pdf"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Fatalf("got %q, want %q", tt.got, tt.expected)
			}
		})
	}
}

func TestFileBasename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"main.c", "main.c"},
		{"/tmp/a/main.c", "main.c"},
		{"..", "file"},
		{".", "file"},
		{"", "file"},
		{"  spaced.txt  ", "spaced.txt"},
	}

	for _, tt := range tests {
		if got := FileBasename(tt.input); got != tt.expected {
			t.Errorf("FileBasename(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestOutputFileKey(t *testing.T) {
	if got := OutputFileKey(7, "test_1/result.txt"); got != "7/test_1/result.txt" {
		t.Fatalf("OutputFileKey = %q", got)
	}
	if got := OutputPrefix(7); got != "7/" {
		t.Fatalf("OutputPrefix = %q", got)
	}
}

func TestOutputPathValid(t *testing.T) {
	valid := []string{
		"result.txt",
		"test_1/result.txt",
		"test_1/nested/dir/result.txt",
		"outputfiles/explicit.txt",
		"a file with spaces.txt",
		"acentuação.txt",
	}
	for _, rel := range valid {
		if !OutputPathValid(rel) {
			t.Errorf("OutputPathValid(%q) = false, want true", rel)
		}
	}

	invalid := []string{
		"",
		".",
		"..",
		"../other.txt",
		"a/../b",
		"/etc/passwd",
		"a//b",
		"./a",
		"a/",
	}
	for _, rel := range invalid {
		if OutputPathValid(rel) {
			t.Errorf("OutputPathValid(%q) = true, want false", rel)
		}
	}
}
