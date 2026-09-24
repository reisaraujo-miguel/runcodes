package models

import "time"

// AdminUser is a user as listed and edited from the admin panel.
type AdminUser struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	OrgID     string    `json:"org_id"`
	Role      string    `json:"role"`
	Confirmed bool      `json:"confirmed"`
	CreatedAt time.Time `json:"created_at"`
}

// UpdateUserRequest is the JSON body of PUT /api/v1/admin/users/{id}. Every
// field is optional: absent fields are left untouched (partial update).
type UpdateUserRequest struct {
	Name      *string `json:"name"`
	Email     *string `json:"email"`
	OrgID     *string `json:"org_id"`
	Role      *string `json:"role"`
	Confirmed *bool   `json:"confirmed"`
}

// AdminOffering is a class as listed from the admin panel, with the ownership
// and size information the professor-facing endpoints do not need.
type AdminOffering struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	EndDate         time.Time `json:"end_date"`
	EnrollmentCode  string    `json:"enrollment_code"`
	VisibleToEnroll bool      `json:"visible_to_enroll"`
	OwnerID         *int64    `json:"owner_id"`
	OwnerName       string    `json:"owner_name"`
	OwnerEmail      string    `json:"owner_email"`
	MemberCount     int64     `json:"member_count"`
	ExerciseCount   int64     `json:"exercise_count"`
	CreatedAt       time.Time `json:"created_at"`
}

// AdminUpdateOfferingRequest is the JSON body of PUT
// /api/v1/admin/offerings/{id}. Every field is optional.
type AdminUpdateOfferingRequest struct {
	Name            *string    `json:"name"`
	Description     *string    `json:"description"`
	EndDate         *time.Time `json:"end_date"`
	VisibleToEnroll *bool      `json:"visible_to_enroll"`
	OwnerID         *int64     `json:"owner_id"`
}
