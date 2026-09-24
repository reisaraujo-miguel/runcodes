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
