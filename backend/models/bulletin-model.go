package models

import "time"

// BulletinComment is one comment on a bulletin-board post, optionally answering
// another comment of the same post.
type BulletinComment struct {
	ID int64 `json:"id"`
	// ParentCommentID is the comment this one answers, null for a comment on the
	// post itself.
	ParentCommentID *int64 `json:"parent_comment_id"`
	AuthorID        int64  `json:"author_id"`
	AuthorName      string `json:"author_name"`
	// AuthorRole is the author's role in the class: professor (including the
	// class owner), monitor or student. It is what the board shows next to a
	// name, and it mirrors the offering's enrollment roles.
	AuthorRole string    `json:"author_role"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}

// BulletinPost is one board post together with its discussion.
type BulletinPost struct {
	ID         int64     `json:"id"`
	AuthorID   int64     `json:"author_id"`
	AuthorName string    `json:"author_name"`
	AuthorRole string    `json:"author_role"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
	// Comments are ordered oldest first, which is how a discussion reads.
	Comments []BulletinComment `json:"comments"`
}

// BulletinBoard is the class page's bulletin board: the newest posts with their
// comments, and what the caller may do on it.
type BulletinBoard struct {
	OfferingID int64 `json:"offering_id"`
	// CanPost is true for the professors and monitors of the class. Every member
	// may comment, so there is no matching "can_comment" flag.
	CanPost bool `json:"can_post"`
	// PostCount is the board's total post count. The response carries only the
	// newest posts, so the UI can say which slice it is showing.
	PostCount int64          `json:"post_count"`
	Posts     []BulletinPost `json:"posts"`
}

// CreateBulletinPostRequest is the JSON body of
// POST /api/v1/offerings/{id}/bulletin/posts.
type CreateBulletinPostRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// CreateBulletinCommentRequest is the JSON body of
// POST /api/v1/offerings/{id}/bulletin/posts/{postId}/comments.
type CreateBulletinCommentRequest struct {
	Body string `json:"body"`
	// ParentCommentID answers another comment of the same post. Null (or absent)
	// comments on the post itself.
	ParentCommentID *int64 `json:"parent_comment_id"`
}
