package services

import "errors"

var (
	ErrEmailExists        = errors.New("email is already in use")
	ErrOrgIDExists        = errors.New("that organization id is already in use")
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
	ErrInvalidFileType  = errors.New("file type is not allowed for this exercise")
	ErrJudgeUnavailable = errors.New("the judge is unavailable, try again later")
	ErrCommitNotFound   = errors.New("commit not found")
	ErrCommitForbidden  = errors.New("you are not allowed to view this commit")
	// ErrCommitSourceNotFound is reported when the source object is no longer
	// in storage (for example, removed by an object-store cleanup).
	ErrCommitSourceNotFound = errors.New("submitted source is not available")

	ErrNotOwner                = errors.New("you are not the owner of this offering")
	ErrTestCaseNotFound        = errors.New("test case not found")
	ErrCompilationFileNotFound = errors.New("compilation file not found")
	ErrAttachedFileNotFound    = errors.New("attached file not found")
	ErrInvalidTestCase         = errors.New("invalid test case")
	ErrInvalidExercise         = errors.New("invalid exercise")

	ErrInvalidEnrollmentCode = errors.New("invalid enrollment code")
	ErrEnrollmentClosed      = errors.New("enrollment is closed for this offering")
	ErrEnrollmentBanned      = errors.New("you are banned from this offering")
	ErrOfferingEnded         = errors.New("this offering has ended")
	ErrOfferingOwned         = errors.New("you own this offering")
	ErrOwnerCannotUnenroll   = errors.New("the owner cannot unenroll from their own offering")
	ErrMemberNotFound        = errors.New("member not found")
	ErrUserNotFound          = errors.New("user not found")
	ErrInvalidMemberRole     = errors.New("invalid member role")
	ErrInvalidRole           = errors.New("invalid role")
	ErrSelfManagement        = errors.New("an admin cannot change their own role or delete their own account")
	ErrSelfUnconfirm         = errors.New("an admin cannot unconfirm their own account")
	ErrNotAProfessor         = errors.New("only a professor or admin account can be assigned to teach a class")
	ErrCannotBanOwner        = errors.New("the owner of an offering cannot be banned or removed")
	ErrAlreadyMember         = errors.New("user is already a member of this offering")
)
