package handlers

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/runcodes-icmc/runcodes/services"
)

type filePart struct {
	field   string
	name    string
	content string
}

func parseMultipartForm(
	t *testing.T, fields map[string][]string, files []filePart,
) *multipart.Form {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	for key, values := range fields {
		for _, value := range values {
			if err := writer.WriteField(key, value); err != nil {
				t.Fatalf("write field: %v", err)
			}
		}
	}
	for _, f := range files {
		part, err := writer.CreateFormFile(f.field, f.name)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := part.Write([]byte(f.content)); err != nil {
			t.Fatalf("write form file: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest("POST", "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(1 << 20); err != nil {
		t.Fatalf("parse multipart: %v", err)
	}
	t.Cleanup(func() { req.MultipartForm.RemoveAll() })

	return req.MultipartForm
}

func TestParseCreateExerciseRequest(t *testing.T) {
	body := `{
		"title": "  Lista 1  ",
		"description": "intro",
		"deadline": "2026-02-01T10:00:00Z",
		"open_date": "2026-01-01T10:00:00Z",
		"show_before_open_date": true,
		"allowed_file_type_ids": [1, 2]
	}`

	req, err := parseCreateExerciseRequest(strings.NewReader(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Title != "Lista 1" {
		t.Fatalf("title not trimmed: %q", req.Title)
	}
	if req.Deadline == nil || req.OpenDate == nil {
		t.Fatal("expected both dates to be parsed")
	}
	if req.ShowBeforeOpenDate == nil || !*req.ShowBeforeOpenDate {
		t.Fatal("expected show_before_open_date to be true")
	}
	if len(req.AllowedFileTypeIDs) != 2 {
		t.Fatalf("unexpected allowed file type ids: %v", req.AllowedFileTypeIDs)
	}
}

func TestParseCreateExerciseRequestErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"malformed json", `{`},
		{"empty title", `{"title":"  ","deadline":"2026-02-01T10:00:00Z","open_date":"2026-01-01T10:00:00Z"}`},
		{"missing deadline", `{"title":"x","open_date":"2026-01-01T10:00:00Z"}`},
		{"missing open date", `{"title":"x","deadline":"2026-02-01T10:00:00Z"}`},
		{"deadline before open", `{"title":"x","deadline":"2026-01-01T10:00:00Z","open_date":"2026-02-01T10:00:00Z"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseCreateExerciseRequest(strings.NewReader(tt.body)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestParseUpdateExerciseRequest(t *testing.T) {
	req, err := parseUpdateExerciseRequest(strings.NewReader(`{"title":"  Novo  "}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Title == nil || *req.Title != "Novo" {
		t.Fatalf("unexpected title: %v", req.Title)
	}
	if req.Description != nil || req.Deadline != nil {
		t.Fatal("absent fields must stay nil")
	}

	if _, err := parseUpdateExerciseRequest(strings.NewReader(
		`{"deadline":"2026-01-01T10:00:00Z","open_date":"2026-02-01T10:00:00Z"}`,
	)); err == nil {
		t.Fatal("expected deadline-before-open-date error")
	}
}

func TestParseTestCaseInputText(t *testing.T) {
	form := parseMultipartForm(t, map[string][]string{
		"input_type":             {"text"},
		"expected_output_type":   {"text"},
		"input":                  {"1 2"},
		"expected_output":        {"3"},
		"show_input":             {"false"},
		"show_expected_output":   {"true"},
		"show_user_output":       {"false"},
		"cpu_time_limit_seconds": {"5"},
		"mem_usage_limit_bytes":  {"1024"},
		"stack_limit_bytes":      {"2048"},
		"file_size_limit_bytes":  {"4096"},
	}, nil)

	in, err := parseTestCaseInput(form, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if in.InputType == nil || *in.InputType != "text" {
		t.Fatalf("unexpected input_type: %v", in.InputType)
	}
	if in.Input == nil || *in.Input != "1 2" {
		t.Fatalf("unexpected input: %v", in.Input)
	}
	if in.ShowInput == nil || *in.ShowInput {
		t.Fatal("expected show_input false")
	}
	if in.ShowExpectedOutput == nil || !*in.ShowExpectedOutput {
		t.Fatal("expected show_expected_output true")
	}
	if in.CPUTimeLimitSeconds == nil || *in.CPUTimeLimitSeconds != 5 {
		t.Fatalf("unexpected cpu limit: %v", in.CPUTimeLimitSeconds)
	}
	if in.MemUsageLimitBytes == nil || *in.MemUsageLimitBytes != 1024 {
		t.Fatalf("unexpected mem limit: %v", in.MemUsageLimitBytes)
	}
	if in.FilesProvided {
		t.Fatal("no files should be provided")
	}
}

func TestParseTestCaseInputFiles(t *testing.T) {
	form := parseMultipartForm(t, map[string][]string{
		"input_type":             {"file"},
		"expected_output_type":   {"text"},
		"expected_output":        {"ok"},
		"cpu_time_limit_seconds": {"1"},
		"mem_usage_limit_bytes":  {"0"},
		"stack_limit_bytes":      {"0"},
		"file_size_limit_bytes":  {"0"},
	}, []filePart{
		{field: "input_file", name: "in.txt", content: "payload"},
		{field: "files", name: "a.txt", content: "a"},
		{field: "files", name: "b.txt", content: "b"},
	})

	in, err := parseTestCaseInput(form, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if in.InputFile == nil || in.InputFile.Filename != "in.txt" {
		t.Fatalf("unexpected input_file: %+v", in.InputFile)
	}
	if string(in.InputFile.Data) != "payload" {
		t.Fatalf("unexpected input_file data: %q", in.InputFile.Data)
	}
	if !in.FilesProvided || len(in.Files) != 2 {
		t.Fatalf("expected 2 extra files, got %+v", in.Files)
	}
	if in.Files[0].Filename != "a.txt" || in.Files[1].Filename != "b.txt" {
		t.Fatalf("unexpected file names: %+v", in.Files)
	}
}

func TestParseTestCaseInputErrors(t *testing.T) {
	tests := []struct {
		name   string
		fields map[string][]string
	}{
		{"invalid int", map[string][]string{"cpu_time_limit_seconds": {"abc"}}},
		{"invalid bool", map[string][]string{"show_input": {"yes"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := parseMultipartForm(t, tt.fields, nil)
			if _, err := parseTestCaseInput(form, false); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestParseTestCaseInputAbsentFields(t *testing.T) {
	form := parseMultipartForm(t, map[string][]string{}, nil)

	in, err := parseTestCaseInput(form, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if in.InputType != nil || in.Input != nil || in.ShowInput != nil {
		t.Fatal("absent fields must stay nil for partial updates")
	}
}

func TestParseTestCaseInputTolerantNumbers(t *testing.T) {
	form := parseMultipartForm(t, map[string][]string{
		"cpu_time_limit_seconds": {"1.5"},
		"mem_usage_limit_bytes":  {""},
	}, nil)

	in, err := parseTestCaseInput(form, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if in.CPUTimeLimitSeconds == nil || *in.CPUTimeLimitSeconds != 2 {
		t.Fatalf("expected decimal seconds to round to 2, got %v", in.CPUTimeLimitSeconds)
	}
	if in.MemUsageLimitBytes != nil {
		t.Fatalf("empty numeric fields must be treated as absent, got %v", *in.MemUsageLimitBytes)
	}
}

func TestIsClientInputError(t *testing.T) {
	if !isClientInputError(services.ErrInvalidExercise) {
		t.Fatal("ErrInvalidExercise should be a client input error")
	}
	if isClientInputError(services.ErrServer) {
		t.Fatal("ErrServer should not be a client input error")
	}
}
