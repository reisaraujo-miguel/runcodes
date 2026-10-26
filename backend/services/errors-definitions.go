package services

import "errors"

var (
	ErrEmailExists        = errors.New("email is already in use")
	ErrOrgIDExists        = errors.New("that institutional id is already in use")
	ErrServer             = errors.New("internal server error, try again later")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailNotVerified   = errors.New("email is not verified, check your inbox for the verification link")
	// ErrCurrentPasswordRequired is returned when a profile update changes the
	// email address without proving knowledge of the current password.
	ErrCurrentPasswordRequired = errors.New("current password is required to change the email")
	// ErrInvalidVerificationToken covers a token that is unknown, already
	// consumed, or past its expiry; a caller sees one message, so the link's
	// state is not disclosed.
	ErrInvalidVerificationToken = errors.New("the verification link is invalid or has expired")
	// ErrInvalidPasswordResetToken covers an unknown, consumed or expired
	// recovery link. Like the verification message, it deliberately does not
	// disclose which of those states the link is in.
	ErrInvalidPasswordResetToken = errors.New("the password reset link is invalid or has expired")
	ErrOfferingNotFound          = errors.New("offering not found")

	ErrExerciseNotFound = errors.New("exercise not found")
	ErrNotEnrolled      = errors.New("you are not enrolled in this exercise's offering")
	ErrDeadlinePassed   = errors.New("the deadline for this exercise has passed")
	ErrNoTestCases      = errors.New("this exercise has no test cases; add at least one before submitting")
	ErrInvalidFileType  = errors.New("file type is not allowed for this exercise")
	ErrJudgeUnavailable = errors.New("the judge is unavailable, try again later")
	// ErrSubmissionLimit is per-user admission control: the caller already has
	// the maximum number of runs waiting for the judge.
	ErrSubmissionLimit = errors.New("you already have the maximum number of submissions waiting; wait for one to finish")
	// ErrQueueFull is global admission control: the judge queue is at its
	// configured depth, so accepting this submission would only delay everyone.
	ErrQueueFull       = errors.New("the judging queue is full; try again in a moment")
	ErrCommitNotFound  = errors.New("commit not found")
	ErrCommitForbidden = errors.New("you are not allowed to view this commit")
	// ErrCommitSourceNotFound is reported when the source object is no longer
	// in storage (for example, removed by an object-store cleanup).
	ErrCommitSourceNotFound = errors.New("submitted source is not available")

	ErrNotOwner                = errors.New("you are not the owner of this offering")
	ErrTestCaseNotFound        = errors.New("test case not found")
	ErrCompilationFileNotFound = errors.New("compilation file not found")
	ErrAttachedFileNotFound    = errors.New("attached file not found")
	ErrInvalidTestCase         = errors.New("invalid test case")
	ErrInvalidExercise         = errors.New("invalid exercise")
	ErrInvalidEmail            = errors.New("invalid email message")

	ErrInvalidEnrollmentCode  = errors.New("invalid enrollment code")
	ErrEnrollmentClosed       = errors.New("enrollment is closed for this offering")
	ErrEnrollmentBanned       = errors.New("you are banned from this offering")
	ErrOfferingEnded          = errors.New("this offering has ended")
	ErrOfferingOwned          = errors.New("you own this offering")
	ErrOwnerCannotUnenroll    = errors.New("the owner cannot unenroll from their own offering")
	ErrMemberNotFound         = errors.New("member not found")
	ErrUserNotFound           = errors.New("user not found")
	ErrOwnerReferenceConflict = errors.New("provide at most one of owner_email, owner_org_id or owner_id")
	ErrOwnerReferenceEmpty    = errors.New("the owner reference cannot be empty")
	ErrInvalidMemberRole      = errors.New("invalid member role")
	ErrInvalidRole            = errors.New("invalid role")
	ErrSelfManagement         = errors.New("an admin cannot change their own role, change their own email, or delete their own account")
	ErrSelfUnconfirm          = errors.New("an admin cannot unconfirm their own account")
	ErrNotAProfessor          = errors.New("only a professor or admin account can be assigned to teach a class")
	ErrCannotBanOwner         = errors.New("the owner of an offering cannot be banned or removed")
	ErrAlreadyMember          = errors.New("user is already a member of this offering")

	ErrFileTypeNotFound = errors.New("file type not found")
	ErrFileTypeExists   = errors.New("a file type with that name or extension already exists")
	// ErrFileTypeInUse refuses to delete a catalog entry that exercises still
	// reference; hiding it with is_available is the reversible alternative.
	ErrFileTypeInUse = errors.New("this file type is used by at least one exercise; disable it instead")
	// ErrCannotPreview refuses a preview for an account that is not an
	// administrator or for a role that cannot be previewed.
	ErrCannotPreview      = errors.New("only an administrator can preview a role")
	ErrInvalidPreviewRole = errors.New("preview role must be \"student\" or \"professor\"")
	ErrNotPreviewing      = errors.New("this session is not a role preview session")
	// ErrInvalidArchive reports an upload that is not a readable zip file.
	ErrInvalidArchive = errors.New("the uploaded file is not a valid zip archive")
	// ErrArchiveTooLarge is returned when a compressed or expanded archive
	// exceeds the platform's limits.
	ErrArchiveTooLarge = errors.New("the uploaded archive is too large")
	ErrInvalidTransfer = errors.New("the offering archive is not valid")
	ErrImportFailed    = errors.New("could not import the offering archive")
)
