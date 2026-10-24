package services

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/runcodes-icmc/runcodes/models"
)

func TestWriteOfferingGradebookCSV(t *testing.T) {
	score := 75.5
	submittedAt := time.Date(2026, 10, 20, 18, 0, 0, 0, time.UTC)
	gradebook := &models.OfferingGradebook{
		Exercises: []models.GradebookExercise{
			{ID: 1, Title: "=soma"},
			{ID: 2, Title: "Exercício 2"},
		},
		Students: []models.GradebookStudent{
			{
				Name:  "Maria",
				Email: "maria@example.com",
				Scores: []models.GradebookCell{
					{ExerciseID: 1, Score: &score, SubmissionCount: 2, LastSubmissionAt: &submittedAt},
					{ExerciseID: 2},
				},
				Average:          37.75,
				SubmissionCount:  2,
				LastSubmissionAt: &submittedAt,
			},
		},
	}

	var buffer bytes.Buffer
	if err := WriteOfferingGradebookCSV(&buffer, gradebook); err != nil {
		t.Fatalf("WriteOfferingGradebookCSV: %v", err)
	}

	reader := csv.NewReader(strings.NewReader(buffer.String()))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2: %#v", len(records), records)
	}

	header := records[0]
	wantHeader := []string{
		"Aluno", "Email", "'=soma", "Exercício 2",
		"Média", "Submissões", "Última submissão",
	}
	if strings.Join(header, "|") != strings.Join(wantHeader, "|") {
		t.Fatalf("header = %#v, want %#v", header, wantHeader)
	}

	row := records[1]
	wantRow := []string{
		"Maria", "maria@example.com", "75.50", "",
		"37.75", "2", submittedAt.In(time.Local).Format("2006-01-02 15:04:05"),
	}
	if strings.Join(row, "|") != strings.Join(wantRow, "|") {
		t.Fatalf("row = %#v, want %#v", row, wantRow)
	}
}

func TestCSVSafeNeutralizesFormulasAndLineBreaks(t *testing.T) {
	tests := map[string]string{
		"=1+1":            "'=1+1",
		"+1":              "'+1",
		"-1":              "'-1",
		"@cmd":            "'@cmd",
		"linha\nquebrada": "linha quebrada",
		"normal":          "normal",
	}
	for input, want := range tests {
		if got := csvSafe(input); got != want {
			t.Errorf("csvSafe(%q) = %q, want %q", input, got, want)
		}
	}
}
