package services

import "testing"

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "main.c", "main.c"},
		{"directory is stripped", "../../etc/passwd", "passwd"},
		{"nested path", "a/b/c.py", "c.py"},
		{"windows separator", "a\\b\\c.py", "c.py"},
		{"unsafe characters", "weird name!.c", "weird_name_.c"},
		{"empty", "", "submission"},
		{"dots only", "..", "submission"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeFilename(tt.input); got != tt.expected {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFilenameMatchesExtension(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		extension string
		expected  bool
	}{
		{"simple match", "main.c", "c", true},
		{"case insensitive", "MAIN.C", "c", true},
		{"compound extension", "main.omp.c", "omp.c", true},
		{"prefix collision", "main.omp.c", "mp.c", false},
		{"wrong extension", "main.cpp", "c", false},
		{"no extension", "README", "zip", false},
		{"zip", "project.zip", "zip", true},
		{"empty extension", "main.c", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filenameMatchesExtension(tt.filename, tt.extension); got != tt.expected {
				t.Errorf("filenameMatchesExtension(%q, %q) = %v, want %v",
					tt.filename, tt.extension, got, tt.expected)
			}
		})
	}
}

func TestParseClientIP(t *testing.T) {
	tests := []struct {
		name         string
		forwardedFor string
		remoteAddr   string
		expected     string
	}{
		{"forwarded for first entry", "203.0.113.5, 10.0.0.1", "10.0.0.1:5555", "203.0.113.5"},
		{"remote addr with port", "", "192.168.0.2:1234", "192.168.0.2"},
		{"remote addr without port", "", "192.168.0.2", "192.168.0.2"},
		{"invalid", "not-an-ip", "also-bad", ""},
		{"empty", "", "", ""},
		{"ipv6", "", "[2001:db8::1]:9000", "2001:db8::1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseClientIP(tt.forwardedFor, tt.remoteAddr)
			if tt.expected == "" {
				if got != nil {
					t.Errorf("expected nil, got %q", *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected %q, got nil", tt.expected)
			}
			if *got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, *got)
			}
		})
	}
}
