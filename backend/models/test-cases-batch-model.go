package models

// BatchTestCaseIssue names one case of a test-case archive that could not be
// interpreted, and why. The folder is the case folder inside the zip, or the
// base name of a pair at the archive root.
type BatchTestCaseIssue struct {
	Folder string `json:"folder"`
	Reason string `json:"reason"`
}

// BatchTestCaseErrorResponse is the body of a rejected batch import. It is more
// specific than Error: the caller gets every offending folder at once instead of
// fixing one folder per upload.
type BatchTestCaseErrorResponse struct {
	Message        string               `json:"error_msg"`
	InvalidFolders []BatchTestCaseIssue `json:"invalid_folders,omitempty"`
}

// BatchTestCaseImportResponse reports a successful batch import.
type BatchTestCaseImportResponse struct {
	Created int `json:"created"`
	// Ignored lists the files at the archive root that were skipped because they
	// are neither `.in` nor `.out`, so the professor can tell what was left out.
	Ignored   []string   `json:"ignored,omitempty"`
	TestCases []TestCase `json:"test_cases"`
}
