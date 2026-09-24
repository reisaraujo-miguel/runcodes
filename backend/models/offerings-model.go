package models

import "time"

type CreateOfferingRequest struct {
	Name        string `json:"name"`
	EndDate     string `json:"end_date"`
	Description string `json:"description"`
}

// UpdateOfferingRequest is the JSON body of PUT /api/v1/offerings/{id}. Every
// field is optional: absent fields are left untouched (partial update).
type UpdateOfferingRequest struct {
	Name            *string `json:"name"`
	EndDate         *string `json:"end_date"`
	Description     *string `json:"description"`
	VisibleToEnroll *bool   `json:"visible_to_enroll"`
}

// Offering is a class offering as returned by the API.
type Offering struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	EndDate         string `json:"end_date"`
	Description     string `json:"description"`
	EnrollmentCode  string `json:"enrollment_code"`
	VisibleToEnroll bool   `json:"visible_to_enroll"`
}

// OwnedOffering is a class the requesting user owns or teaches, as listed by
// GET /api/v1/offerings.
type OwnedOffering struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	EndDate         time.Time `json:"end_date"`
	EnrollmentCode  string    `json:"enrollment_code"`
	VisibleToEnroll bool      `json:"visible_to_enroll"`
	OwnerID         int64     `json:"owner_id"`
	IsOwner         bool      `json:"is_owner"`
	MemberCount     int64     `json:"member_count"`
	ExerciseCount   int64     `json:"exercise_count"`
	EnrollmentOpen  bool      `json:"enrollment_open"`
}

// EnrollRequest is the JSON body of POST /api/v1/offerings/enroll: the code the
// professor shares with the class.
type EnrollRequest struct {
	EnrollmentCode string `json:"enrollment_code"`
}

// Enrollment is one of the caller's own class memberships, with the class it
// points at, as listed by GET /api/v1/user/offerings.
type Enrollment struct {
	OfferingID  int64     `json:"offering_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	EndDate     time.Time `json:"end_date"`
	Role        string    `json:"role"`
	OwnerName   string    `json:"owner_name"`
	IsOwner     bool      `json:"is_owner"`
}

// OpenExercise is an exercise of one of the caller's classes that is currently
// open, as listed by GET /api/v1/user/exercises.
type OpenExercise struct {
	ID           int64     `json:"id"`
	OfferingID   int64     `json:"offering_id"`
	OfferingName string    `json:"offering_name"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Deadline     time.Time `json:"deadline"`
	OpenDate     time.Time `json:"open_date"`
}

// OfferingMember is a user enrolled in an offering (student, monitor or
// professor) as returned by the member endpoints.
type OfferingMember struct {
	UserID    int64     `json:"user_id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Banned    bool      `json:"banned"`
	CreatedAt time.Time `json:"created_at"`
}

// AddMemberRequest is the JSON body of POST
// /api/v1/offerings/{id}/members.
type AddMemberRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// UpdateMemberRequest is the JSON body of PUT
// /api/v1/offerings/{id}/members/{userId}. Every field is optional.
type UpdateMemberRequest struct {
	Role   *string `json:"role"`
	Banned *bool   `json:"banned"`
}
