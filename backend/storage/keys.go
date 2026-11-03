package storage

import (
	"crypto/rand"
	"fmt"
	"path"
	"strings"
	"time"
)

/*
FileBasename normalises an uploaded file name to a safe basename. Directory
components are stripped and path traversal is rejected; otherwise the name is
preserved (spaces and dots included) because test-case programs look files up by
their original name at runtime. The judge materialises S3 objects with this
exact basename.
*/
func FileBasename(name string) string {
	base := strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	base = path.Base(base)
	if base == "" || base == "." || base == ".." || base == "/" {
		return "file"
	}
	return base
}

/*
backupKeyFor derives a temporary key in the same namespace as key. Before an
in-place object replacement the previous object is copied here, so it can be
restored if the surrounding database transaction rolls back.
*/
func backupKeyFor(key string) string {
	var token [8]byte
	if _, err := rand.Read(token[:]); err != nil {
		return fmt.Sprintf("%s.backup-%d", key, time.Now().UnixNano())
	}

	return fmt.Sprintf("%s.backup-%x", key, token)
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
	return fmt.Sprintf("%d/files/%s", caseID, FileBasename(filename))
}

/*
CompilationFileKey is the S3 key of an exercise compilation file in the files
bucket (`compilationfiles/<exercise_id>/<basename>`).
*/
func CompilationFileKey(exerciseID int64, filename string) string {
	return fmt.Sprintf("compilationfiles/%d/%s", exerciseID, FileBasename(filename))
}

/*
AttachmentKey is the S3 key of an exercise attachment in the files bucket
(`attachments/<exercise_id>/<basename>`).
*/
func AttachmentKey(exerciseID int64, filename string) string {
	return fmt.Sprintf("attachments/%d/%s", exerciseID, FileBasename(filename))
}

/*
OutputFileKey is the S3 key of a file the submission generated, in the
outputfiles bucket (`<commit_id>/<path>`). The path keeps the workspace layout
the judge collected from: `test_<case_id>/<relative>` for a file a test case
produced, `outputfiles/<relative>` for one written directly to the runner's
output directory.
*/
func OutputFileKey(commitID int64, filePath string) string {
	return fmt.Sprintf("%d/%s", commitID, filePath)
}

/*
OutputPrefix is the key prefix shared by every generated file of a commit, for
listing and for deleting them together.
*/
func OutputPrefix(commitID int64) string {
	return fmt.Sprintf("%d/", commitID)
}

/*
OutputPathValid reports whether rel names a file inside a commit's output
namespace. The path arrives from the browser, so it must be slash-separated,
relative and already clean: `a/../b`, `/etc/passwd` and `a//b` are all rejected
instead of being resolved into a key outside the commit's prefix.
*/
func OutputPathValid(rel string) bool {
	if rel == "" || rel == "." || strings.HasPrefix(rel, "/") {
		return false
	}

	clean := path.Clean(rel)
	if clean != rel {
		return false
	}

	return clean != ".." && !strings.HasPrefix(clean, "../")
}
