package services

import (
	"fmt"
	"path"
	"strings"
)

/*
fileBasename normalises an uploaded file name to a safe basename. Directory
components are stripped and path traversal is rejected; otherwise the name is
preserved (spaces and dots included) because test-case programs look files up by
their original name at runtime. The judge materialises S3 objects with this
exact basename.
*/
func fileBasename(name string) string {
	base := strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	base = path.Base(base)
	if base == "" || base == "." || base == ".." || base == "/" {
		return "file"
	}
	return base
}

/*
CaseInputKey is the S3 key of a test case's main input object in the cases
bucket (`<case_id>/in`).
*/
func CaseInputKey(caseID int64) string {
	return fmt.Sprintf("%d/in", caseID)
}

/*
CaseOutputKey is the S3 key of a test case's expected output object in the cases
bucket (`<case_id>/out`).
*/
func CaseOutputKey(caseID int64) string {
	return fmt.Sprintf("%d/out", caseID)
}

/*
CaseFileKey is the S3 key of an extra file attached to a test case in the cases
bucket (`<case_id>/files/<basename>`).
*/
func CaseFileKey(caseID int64, filename string) string {
	return fmt.Sprintf("%d/files/%s", caseID, fileBasename(filename))
}

/*
CompilationFileKey is the S3 key of an exercise compilation file in the files
bucket (`compilationfiles/<exercise_id>/<basename>`).
*/
func CompilationFileKey(exerciseID int64, filename string) string {
	return fmt.Sprintf("compilationfiles/%d/%s", exerciseID, fileBasename(filename))
}

/*
AttachmentKey is the S3 key of an exercise attachment in the files bucket
(`attachments/<exercise_id>/<basename>`).
*/
func AttachmentKey(exerciseID int64, filename string) string {
	return fmt.Sprintf("attachments/%d/%s", exerciseID, fileBasename(filename))
}
