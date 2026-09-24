package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/services"
	"github.com/runcodes-icmc/runcodes/validation"

	"github.com/go-chi/jwtauth/v5"
)

/*
CreateOffering handles new offering creations.
*/
func CreateOffering(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.CreateOfferingRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		msg := "Invalid offering creation request"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: msg})
		return
	}

	var claims map[string]any
	var err error
	if _, claims, err = jwtauth.FromContext(ctx); err != nil {
		slog.ErrorContext(ctx,
			"error retrieving claims from context",
			slog.String("error", err.Error()))
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.EndDate = strings.TrimSpace(req.EndDate)

	if err := validation.ValidateRequiredString(
		req.Name, services.MaxOfferingNameLength,
	); err != nil {
		slog.InfoContext(ctx,
			"user tried to create an offering with an invalid name",
			slog.Any("user_id", claims["id"]),
		)
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: err.Error()})
		return
	}

	if date, err := validation.ValidateDate(ctx, req.EndDate); err != nil {
		slog.InfoContext(ctx,
			"user tried to create an offering with an invalid end date",
			slog.Any("user_id", claims["id"]),
		)
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: err.Error()})
		return
	} else if date.Before(time.Now()) {
		slog.InfoContext(ctx,
			"user tried to create an offering with an invalid end date",
			slog.Any("user_id", claims["id"]),
		)
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "end date cannot be in the past"},
		)
		return
	}

	var offering *models.Offering
	if offering, err = services.CreateOffering(ctx, &req, claims); err != nil {
		slog.ErrorContext(ctx,
			"Failed to create offering",
			slog.String("error", err.Error()),
			slog.Any("user_id", claims["id"]),
		)
		WriteResponse(w, http.StatusInternalServerError,
			models.Error{Message: services.ErrServer.Error()},
		)
		return
	}

	WriteResponse(w, http.StatusCreated, offering)
}

/*
GetOffering handles fetching an offering owned by the requesting user.
*/
func GetOffering(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var claims map[string]any
	var err error
	if _, claims, err = jwtauth.FromContext(ctx); err != nil {
		slog.ErrorContext(ctx,
			"error retrieving claims from context",
			slog.String("error", err.Error()))
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	offeringID, err := pathID(r, "id")
	if err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid offering id"},
		)
		return
	}

	var offering *models.Offering
	if offering, err = services.GetOffering(ctx, offeringID, claims); err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, offering)
}

/*
ListOfferings lists the classes the requesting professor manages: the ones they
own and the ones they were assigned to teach.
*/
func ListOfferings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	offerings, err := services.ListOwnedOfferings(ctx, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, offerings)
}

/*
UpdateOffering edits a class owned by the requesting professor.
*/
func UpdateOffering(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	offeringID, err := pathID(r, "id")
	if err != nil || offeringID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid offering id"},
		)
		return
	}

	var req models.UpdateOfferingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "Invalid offering update request"},
		)
		return
	}

	offering, err := services.UpdateOffering(ctx, offeringID, &req, claims)
	if err != nil {
		slog.ErrorContext(ctx, "error updating offering",
			slog.String("error", err.Error()),
			slog.Int64("offering_id", offeringID),
			slog.Any("user_id", claims["id"]),
		)
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, offering)
}

/*
DeleteOffering removes a class owned by the requesting professor.
*/
func DeleteOffering(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	offeringID, err := pathID(r, "id")
	if err != nil || offeringID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid offering id"},
		)
		return
	}

	if err := services.DeleteOffering(ctx, offeringID, claims); err != nil {
		slog.ErrorContext(ctx, "error deleting offering",
			slog.String("error", err.Error()),
			slog.Int64("offering_id", offeringID),
			slog.Any("user_id", claims["id"]),
		)
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusNoContent, nil)
}

/*
Enroll adds the requesting user to the class the enrollment code belongs to.
*/
func Enroll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	var req models.EnrollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "Invalid enrollment request"},
		)
		return
	}

	enrollment, err := services.Enroll(ctx, &req, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusCreated, enrollment)
}

/*
Unenroll removes the requesting user from a class they joined.
*/
func Unenroll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	offeringID, err := pathID(r, "id")
	if err != nil || offeringID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid offering id"},
		)
		return
	}

	if err := services.Unenroll(ctx, offeringID, claims); err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusNoContent, nil)
}

/*
ListMyOfferings lists the classes the requesting user belongs to, for the home
page.
*/
func ListMyOfferings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	offerings, err := services.ListUserOfferings(ctx, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, offerings)
}

/*
ListMyOpenExercises lists the exercises that are open right now in the classes
the requesting user belongs to, for the home page.
*/
func ListMyOpenExercises(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	exercises, err := services.ListUserOpenExercises(ctx, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, exercises)
}

/*
ListOfferingMembers lists the members of a class owned by the requesting user.
*/
func ListOfferingMembers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	offeringID, err := pathID(r, "id")
	if err != nil || offeringID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid offering id"},
		)
		return
	}

	members, err := services.ListOfferingMembers(ctx, offeringID, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, members)
}

/*
AddOfferingMember assigns a monitor or a co-professor to a class owned by the
requesting user.
*/
func AddOfferingMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	offeringID, err := pathID(r, "id")
	if err != nil || offeringID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid offering id"},
		)
		return
	}

	var req models.AddMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "Invalid member request"},
		)
		return
	}

	member, err := services.AddOfferingMember(ctx, offeringID, &req, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusCreated, member)
}

/*
UpdateOfferingMember changes the role of a member of a class owned by the
requesting user, or bans and unbans them.
*/
func UpdateOfferingMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	offeringID, err := pathID(r, "id")
	if err != nil || offeringID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid offering id"},
		)
		return
	}

	memberID, err := pathID(r, "userId")
	if err != nil || memberID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid user id"},
		)
		return
	}

	var req models.UpdateMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "Invalid member request"},
		)
		return
	}

	member, err := services.UpdateOfferingMember(
		ctx, offeringID, memberID, &req, claims,
	)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, member)
}

/*
RemoveOfferingMember drops a member from a class owned by the requesting user.
*/
func RemoveOfferingMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	offeringID, err := pathID(r, "id")
	if err != nil || offeringID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid offering id"},
		)
		return
	}

	memberID, err := pathID(r, "userId")
	if err != nil || memberID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid user id"},
		)
		return
	}

	if err := services.RemoveOfferingMember(
		ctx, offeringID, memberID, claims,
	); err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusNoContent, nil)
}
