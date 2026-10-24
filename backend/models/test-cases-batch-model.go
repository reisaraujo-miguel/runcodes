package models

// BatchTestCaseIssue names one folder of a test-case archive that could not be
// interpreted, and why. The folder is the path inside the zip.
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
	Created   int        `json:"created"`
	TestCases []TestCase `json:"test_cases"`
}
