package services

import "errors"

var (
	ErrEmailExists        = errors.New("email is already in use")
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
)
