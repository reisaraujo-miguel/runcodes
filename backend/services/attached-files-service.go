package services

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"

	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/storage"
)

/*
canReadExerciseContent reports whether the caller may read an exercise's
attachments. It applies the same enrollment and visibility rules as GetExercise
and ListTestCases: a removed or not-yet-open exercise is hidden from students,
not just from the exercise metadata endpoint.
*/
func canReadExerciseContent(acc *exerciseAccess) error {
	if !acc.canAuthor && !acc.isEnrolled {
		return ErrNotEnrolled
	}
	if !acc.canAuthor && !exerciseVisible(acc.exercise, nowFunc()) {
		return ErrExerciseNotFound
	}
	return nil
}

/*
ListAttachedFiles returns an exercise's attachments. Owners and enrolled
students may read them.
*/
func ListAttachedFiles(
	ctx context.Context, exerciseID int64, claims map[string]any,
) ([]models.AttachedFile, error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}
	acc, err := loadExerciseAccess(ctx, exerciseID, userID, role)
	if err != nil {
		return nil, err
	}
	if err := canReadExerciseContent(acc); err != nil {
		return nil, err
	}
	return queryAttachedFiles(ctx, exerciseID)
}

func queryAttachedFiles(
	ctx context.Context, exerciseID int64,
) ([]models.AttachedFile, error) {
	rows, err := database.DB.QueryContext(ctx, `
		SELECT id, exercise_id, path
		FROM exercises_attached_files
		WHERE exercise_id = $1
		ORDER BY id`, exerciseID)
	if err != nil {
		slog.ErrorContext(ctx, "error fetching attached files",
			slog.Int64("exercise_id", exerciseID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	files := make([]models.AttachedFile, 0, 4)
	for rows.Next() {
		var f models.AttachedFile
		if err := rows.Scan(&f.ID, &f.ExerciseID, &f.Path); err != nil {
			return nil, ErrServer
		}
		f.Filename = f.Path
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}
	return files, nil
}

// AttachedFileObject is an attachment opened for download.
type AttachedFileObject struct {
	Filename    string
	ContentType string
	Size        int64
	Body        io.ReadCloser
}

/*
GetAttachedFile opens an exercise attachment for streaming. Owners and enrolled
students may read it, matching the listing endpoint.
*/
func GetAttachedFile(
	ctx context.Context, exerciseID, fileID int64, claims map[string]any,
) (*AttachedFileObject, error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}
	acc, err := loadExerciseAccess(ctx, exerciseID, userID, role)
	if err != nil {
		return nil, err
	}
	if err := canReadExerciseContent(acc); err != nil {
		return nil, err
	}

	var path string
	err = database.DB.QueryRowContext(ctx, `
		SELECT path
		FROM exercises_attached_files
		WHERE id = $1 AND exercise_id = $2`,
		fileID, exerciseID,
	).Scan(&path)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAttachedFileNotFound
		}
		slog.ErrorContext(ctx, "error fetching attached file",
			slog.Int64("attached_file_id", fileID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	object, err := storage.GetFileObject(
		ctx, storage.AttachmentKey(exerciseID, path),
	)
	if err != nil {
		slog.ErrorContext(ctx, "failed to open attached file object",
			slog.Int64("attached_file_id", fileID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	return &AttachedFileObject{
		Filename:    path,
		ContentType: object.ContentType,
		Size:        object.Size,
		Body:        object.Body,
	}, nil
}

/*
CreateAttachedFile stores an attachment for an exercise owned by the caller
under the files bucket (`attachments/<exercise_id>/<basename>`).
*/
func CreateAttachedFile(
	ctx context.Context, exerciseID int64, file UploadedFile, claims map[string]any,
) (*models.AttachedFile, error) {
	if _, err := requireExerciseAuthor(ctx, exerciseID, claims); err != nil {
		return nil, err
	}

	name := storage.FileBasename(file.Filename)
	key := storage.AttachmentKey(exerciseID, name)

	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		slog.ErrorContext(ctx, "error initializing attached file transaction",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM exercises_attached_files
		WHERE exercise_id = $1 AND path = $2`, exerciseID, name,
	); err != nil {
		slog.ErrorContext(ctx, "error replacing attached file row",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	var fileID int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO exercises_attached_files (exercise_id, path)
		VALUES ($1, $2)
		RETURNING id`, exerciseID, name,
	).Scan(&fileID); err != nil {
		slog.ErrorContext(ctx, "error inserting attached file",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	// An attachment with this name may already exist and share the key; the
	// replacer backs up the previous object so the transaction can be rolled back.
	replacer := storage.NewObjectReplacer(ctx, storage.FileOps)

	if err := replacer.Replace(key, file.Data, file.ContentType); err != nil {
		slog.ErrorContext(ctx, "failed to upload attached file",
			slog.String("s3_key", key),
			slog.String("error", err.Error()),
		)
		replacer.Restore()
		return nil, ErrServer
	}

	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx, "error committing attached file",
			slog.String("error", err.Error()),
		)
		replacer.Restore()
		return nil, ErrServer
	}

	replacer.Discard()

	return &models.AttachedFile{ID: fileID, ExerciseID: exerciseID, Path: name, Filename: name}, nil
}

/*
DeleteAttachedFile removes an attachment (row and S3 object).
*/
func DeleteAttachedFile(
	ctx context.Context, exerciseID, fileID int64, claims map[string]any,
) error {
	if _, err := requireExerciseAuthor(ctx, exerciseID, claims); err != nil {
		return err
	}

	var path string
	err := database.DB.QueryRowContext(ctx, `
		DELETE FROM exercises_attached_files
		WHERE id = $1 AND exercise_id = $2
		RETURNING path`, fileID, exerciseID,
	).Scan(&path)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrAttachedFileNotFound
		}
		slog.ErrorContext(ctx, "error deleting attached file",
			slog.Int64("attached_file_id", fileID),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	deleteFileObjectBestEffort(ctx, storage.AttachmentKey(exerciseID, path))
	return nil
}
