package services

import "testing"

func TestNormalizeFileTypeExtension(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{".PY", "py", false},
		{" py ", "py", false},
		{"omp.cpp", "omp.cpp", false},
		{"c++", "c++", false},
		{"", "", true},
		{"..", "", true},
		{"a/b", "", true},
		{"a b", "", true},
		{"a..b", "", true},
		{"-py", "", true},
	}

	for _, test := range tests {
		got, err := normalizeFileTypeExtension(test.input)
		if test.wantErr {
			if err == nil {
				t.Errorf("normalizeFileTypeExtension(%q) = %q, want error",
					test.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("normalizeFileTypeExtension(%q): %v", test.input, err)
			continue
		}
		if got != test.want {
			t.Errorf("normalizeFileTypeExtension(%q) = %q, want %q",
				test.input, got, test.want)
		}
	}
}
