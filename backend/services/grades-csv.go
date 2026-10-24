package services

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/runcodes-icmc/runcodes/models"
)

// csvFormulaPrefixes are the leading characters a spreadsheet treats as the
// start of a formula. A student name or exercise title is user input, so a cell
// starting with one is prefixed with an apostrophe to keep it text.
const csvFormulaPrefixes = "=+-@\t\r"

/*
WriteOfferingGradebookCSV renders the grade table as RFC 4180 CSV.

Every exercise is a column, followed by the student's offering grade, the number
of submissions and the last submission date. An exercise without a submission is
left empty instead of being written as zero, so a spreadsheet can tell "not
submitted" from "submitted, scored zero". The offering-grade column still counts
missing exercises as zero, matching the on-screen table.
*/
func WriteOfferingGradebookCSV(
	w io.Writer, gradebook *models.OfferingGradebook,
) error {
	writer := csv.NewWriter(w)
	writer.UseCRLF = true

	header := make([]string, 0, len(gradebook.Exercises)+5)
	header = append(header, "Aluno", "Email")
	for _, exercise := range gradebook.Exercises {
		header = append(header, csvSafe(exercise.Title))
	}
	header = append(header, "Média", "Submissões", "Última submissão")

	if err := writer.Write(header); err != nil {
		return err
	}

	for _, student := range gradebook.Students {
		row := make([]string, 0, len(gradebook.Exercises)+5)
		row = append(row, csvSafe(student.Name), csvSafe(student.Email))
		for _, cell := range student.Scores {
			if cell.Score == nil {
				row = append(row, "")
				continue
			}
			row = append(row, strconv.FormatFloat(*cell.Score, 'f', 2, 64))
		}
		row = append(row,
			strconv.FormatFloat(student.Average, 'f', 2, 64),
			strconv.FormatInt(student.SubmissionCount, 10),
			csvSafe(formatCSVTime(student.LastSubmissionAt)),
		)
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	writer.Flush()
	return writer.Error()
}

func csvSafe(value string) string {
	value = strings.ReplaceAll(value, "\r\n", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	if value == "" {
		return value
	}
	if strings.ContainsRune(csvFormulaPrefixes, rune(value[0])) {
		return "'" + value
	}
	return value
}

func formatCSVTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.In(time.Local).Format("2006-01-02 15:04:05")
}
