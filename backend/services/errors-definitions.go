package services

import "errors"

var (
	ErrEmailExists        = errors.New("email is already in use")
	ErrOrgIDExists        = errors.New("that organization id is already in use")
	ErrServer             = errors.New("internal server error, try again later")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrOfferingNotFound   = errors.New("offering not found")

	ErrExerciseNotFound = errors.New("exercise not found")
	ErrNotEnrolled      = errors.New("you are not enrolled in this exercise's offering")
	ErrDeadlinePassed   = errors.New("the deadline for this exercise has passed")
	ErrInvalidFileType  = errors.New("file type is not allowed for this exercise")
	ErrJudgeUnavailable = errors.New("the judge is unavailable, try again later")
	ErrCommitNotFound   = errors.New("commit not found")
	ErrCommitForbidden  = errors.New("you are not allowed to view this commit")

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
	ErrNotAProfessor         = errors.New("only a professor or admin account can be assigned to teach a class")
	ErrCannotBanOwner        = errors.New("the owner of an offering cannot be banned or removed")
	ErrAlreadyMember         = errors.New("user is already a member of this offering")
)
