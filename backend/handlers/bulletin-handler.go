package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/services"
)

/*
GetOfferingBulletin handles GET /api/v1/offerings/{id}/bulletin: the class's
newest posts with their comments, and whether the caller may publish one.

Every participant may read the board; the service checks that the caller can see
the class at all.
*/
func GetOfferingBulletin(w http.ResponseWriter, r *http.Request) {
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

	board, err := services.ListOfferingBulletin(ctx, offeringID, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, board)
}

/*
CreateOfferingPost handles POST /api/v1/offerings/{id}/bulletin/posts: an
announcement on the class board.

Only the class staff may post (the service enforces it). The enrolled students
are emailed the post in the background, which is why the request answers before
the deliveries finish.
*/
func CreateOfferingPost(w http.ResponseWriter, r *http.Request) {
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

	var req models.CreateBulletinPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid post request"},
		)
		return
	}

	post, err := services.CreateOfferingPost(ctx, offeringID, claims, &req)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusCreated, post)
}

/*
CreateBulletinComment handles
POST /api/v1/offerings/{id}/bulletin/posts/{postId}/comments.

Any participant of the class may comment. `parent_comment_id` answers another
comment of the same post, and it is what routes the notification to that
comment's author instead of to the class staff.
*/
func CreateBulletinComment(w http.ResponseWriter, r *http.Request) {
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

	postID, err := pathID(r, "postId")
	if err != nil || postID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid post id"},
		)
		return
	}

	var req models.CreateBulletinCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid comment request"},
		)
		return
	}

	comment, err := services.CreateBulletinComment(ctx, offeringID, postID, claims, &req)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusCreated, comment)
}
