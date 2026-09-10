package services

import (
	"errors"
	"testing"
	"time"

	"github.com/runcodes-icmc/runcodes/models"
)

func ptrInt(v int) *int          { return &v }
func ptrInt64(v int64) *int64    { return &v }
func ptrBool(v bool) *bool       { return &v }
func ptrString(v string) *string { return &v }

func validTextCreate() *TestCaseInput {
	return &TestCaseInput{
		InputType:           ptrString(IOTypeText),
		ExpectedOutputType:  ptrString(IOTypeText),
		Input:               ptrString("1 2"),
		ExpectedOutput:      ptrString("3"),
		CPUTimeLimitSeconds: ptrInt(5),
		MemUsageLimitBytes:  ptrInt64(1024),
		StackLimitBytes:     ptrInt64(2048),
		FileSizeLimitBytes:  ptrInt64(4096),
	}
}

func TestResolveTestCaseInputTextCreate(t *testing.T) {
	res, err := ResolveTestCaseInput(validTextCreate(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.InputType != IOTypeText || res.ExpectedOutputType != IOTypeText {
		t.Fatalf("unexpected types: %+v", res)
	}
	if res.Input != "1 2" || res.ExpectedOutput != "3" {
		t.Fatalf("unexpected content: %+v", res)
	}
	if !res.ShowInput || !res.ShowExpectedOutput || !res.ShowUserOutput {
		t.Fatalf("show_* must default to true: %+v", res)
	}
	if res.CPUTimeLimitSeconds != 5 || res.MemUsageLimitBytes != 1024 {
		t.Fatalf("unexpected limits: %+v", res)
	}
	if res.InputObject == nil || string(res.InputObject.Data) != "1 2" {
		t.Fatalf("expected input object to upload: %+v", res.InputObject)
	}
	if res.OutputObject == nil || string(res.OutputObject.Data) != "3" {
		t.Fatalf("expected output object to upload: %+v", res.OutputObject)
	}
}

func TestResolveTestCaseInputFileCreate(t *testing.T) {
	in := validTextCreate()
	in.InputType = ptrString(IOTypeFile)
	in.Input = nil
	in.InputFile = &UploadedFile{Filename: "/tmp/input.txt", Data: []byte("data")}
	in.ExpectedOutputType = ptrString(IOTypeFile)
	in.ExpectedOutput = nil
	in.ExpectedOutputFile = &UploadedFile{Filename: "out.txt", Data: []byte("expected")}

	res, err := ResolveTestCaseInput(in, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Input != "input.txt" {
		t.Fatalf("expected basename as input column, got %q", res.Input)
	}
	if res.ExpectedOutput != "out.txt" {
		t.Fatalf("expected basename as output column, got %q", res.ExpectedOutput)
	}
	if res.InputObject == nil || res.OutputObject == nil {
		t.Fatal("expected both objects to upload")
	}
}

func TestResolveTestCaseInputValidation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*TestCaseInput)
		wantErr bool
	}{
		{"missing input type", func(in *TestCaseInput) { in.InputType = nil }, true},
		{"invalid input type", func(in *TestCaseInput) { in.InputType = ptrString("binary") }, true},
		{"missing input text", func(in *TestCaseInput) { in.Input = nil }, true},
		{"missing output text", func(in *TestCaseInput) { in.ExpectedOutput = nil }, true},
		{"negative cpu limit", func(in *TestCaseInput) { in.CPUTimeLimitSeconds = ptrInt(-1) }, true},
		{"negative mem limit", func(in *TestCaseInput) { in.MemUsageLimitBytes = ptrInt64(-1) }, true},
		{"negative size limit", func(in *TestCaseInput) { in.FileSizeLimitBytes = ptrInt64(-1) }, true},
		{"file input without file", func(in *TestCaseInput) {
			in.InputType = ptrString(IOTypeFile)
			in.Input = nil
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validTextCreate()
			tt.mutate(in)
			_, err := ResolveTestCaseInput(in, nil)
			if tt.wantErr && err == nil {
				t.Fatal("expected an error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestResolveTestCaseInputLimitsDefaultToZero(t *testing.T) {
	in := &TestCaseInput{
		InputType:          ptrString(IOTypeText),
		ExpectedOutputType: ptrString(IOTypeText),
		Input:              ptrString(""),
		ExpectedOutput:     ptrString("ok"),
	}

	res, err := ResolveTestCaseInput(in, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.CPUTimeLimitSeconds != 0 || res.MemUsageLimitBytes != 0 ||
		res.StackLimitBytes != 0 || res.FileSizeLimitBytes != 0 {
		t.Fatalf("expected zero-value limits, got %+v", res)
	}
}

func TestResolveTestCaseInputPartialUpdate(t *testing.T) {
	existing := &models.TestCase{
		ID:                  1,
		ExerciseID:          2,
		Input:               "old input",
		InputType:           IOTypeText,
		ShowInput:           true,
		ExpectedOutput:      "old output",
		ExpectedOutputType:  IOTypeText,
		ShowExpectedOutput:  true,
		ShowUserOutput:      true,
		CPUTimeLimitSeconds: 3,
		MemUsageLimitBytes:  100,
		StackLimitBytes:     200,
		FileSizeLimitBytes:  300,
	}

	in := &TestCaseInput{ShowInput: ptrBool(false)}
	res, err := ResolveTestCaseInput(in, existing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.ShowInput {
		t.Fatal("show_input should have been updated to false")
	}
	if res.Input != "old input" || res.ExpectedOutput != "old output" {
		t.Fatalf("untouched fields changed: %+v", res)
	}
	if res.CPUTimeLimitSeconds != 3 || res.MemUsageLimitBytes != 100 {
		t.Fatalf("limits should be preserved: %+v", res)
	}
	if res.InputObject != nil || res.OutputObject != nil {
		t.Fatal("no object should be re-uploaded for an unchanged text case")
	}
}

func TestResolveTestCaseInputPartialUpdateFileKeepsObject(t *testing.T) {
	existing := &models.TestCase{
		Input:               "input.txt",
		InputType:           IOTypeFile,
		ExpectedOutput:      "out.txt",
		ExpectedOutputType:  IOTypeFile,
		CPUTimeLimitSeconds: 2,
	}

	res, err := ResolveTestCaseInput(&TestCaseInput{ShowInput: ptrBool(true)}, existing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.InputObject != nil || res.OutputObject != nil {
		t.Fatal("file objects must not be re-uploaded without new file parts")
	}
	if res.Input != "input.txt" {
		t.Fatalf("expected existing filename, got %q", res.Input)
	}
}

func TestResolveTestCaseInputDedupesFiles(t *testing.T) {
	in := validTextCreate()
	in.FilesProvided = true
	in.Files = []UploadedFile{
		{Filename: "a.txt", Data: []byte("1")},
		{Filename: "dir/a.txt", Data: []byte("2")},
		{Filename: "b.txt", Data: []byte("3")},
	}

	res, err := ResolveTestCaseInput(in, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.FilesChanged {
		t.Fatal("expected FilesChanged")
	}
	if len(res.Files) != 2 {
		t.Fatalf("expected 2 deduped files, got %d", len(res.Files))
	}
	if res.Files[0].Name != "a.txt" || res.Files[1].Name != "b.txt" {
		t.Fatalf("unexpected file names: %+v", res.Files)
	}
}

func TestResolveTestCaseInputFileRequiresPart(t *testing.T) {
	in := validTextCreate()
	in.InputType = ptrString(IOTypeFile)
	in.Input = nil
	_, err := ResolveTestCaseInput(in, nil)
	if !errors.Is(err, ErrInvalidTestCase) {
		t.Fatalf("expected ErrInvalidTestCase, got %v", err)
	}
}

func TestValidateExerciseFields(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)

	tests := []struct {
		name     string
		title    string
		deadline time.Time
		open     time.Time
		wantErr  bool
	}{
		{"valid", "Lista 1", later, now, false},
		{"empty title", "  ", later, now, true},
		{"zero deadline", "Lista 1", time.Time{}, now, true},
		{"zero open date", "Lista 1", later, time.Time{}, true},
		{"deadline before open", "Lista 1", now, later, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExerciseFields(tt.title, tt.deadline, tt.open)
			if tt.wantErr && err == nil {
				t.Fatal("expected an error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestExerciseVisible(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		exercise models.Exercise
		expected bool
	}{
		{"removed", models.Exercise{OpenDate: now.Add(-time.Hour), Removed: true}, false},
		{"open in the past", models.Exercise{OpenDate: now.Add(-time.Hour)}, true},
		{"open in the future", models.Exercise{OpenDate: now.Add(time.Hour)}, false},
		{"future but shown", models.Exercise{OpenDate: now.Add(time.Hour), ShowBeforeOpenDate: true}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exerciseVisible(tt.exercise, now); got != tt.expected {
				t.Fatalf("exerciseVisible = %v, want %v", got, tt.expected)
			}
		})
	}
}
