package services

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/storage"
)

/*
ListCompilationFiles returns the compilation files of an exercise (owner only).
*/
func ListCompilationFiles(
	ctx context.Context, exerciseID int64, claims map[string]any,
) ([]models.CompilationFile, error) {
	acc, err := requireExerciseAuthor(ctx, exerciseID, claims)
	if err != nil {
		return nil, err
	}
	return queryCompilationFiles(ctx, acc.authoringExerciseID())
}

func queryCompilationFiles(
	ctx context.Context, exerciseID int64,
) ([]models.CompilationFile, error) {
	rows, err := database.DB.QueryContext(ctx, `
		SELECT id, exercise_id, path
		FROM exercises_compilation_files
		WHERE exercise_id = $1
		ORDER BY id`, exerciseID)
	if err != nil {
		slog.ErrorContext(ctx, "error fetching compilation files",
			slog.Int64("exercise_id", exerciseID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	files := make([]models.CompilationFile, 0, 4)
	for rows.Next() {
		var f models.CompilationFile
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

/*
GetCompilationFile returns a single compilation file (owner only).
*/
func GetCompilationFile(
	ctx context.Context, exerciseID, fileID int64, claims map[string]any,
) (*models.CompilationFile, error) {
	acc, err := requireExerciseAuthor(ctx, exerciseID, claims)
	if err != nil {
		return nil, err
	}

	var f models.CompilationFile
	err = database.DB.QueryRowContext(ctx, `
		SELECT id, exercise_id, path
		FROM exercises_compilation_files
		WHERE id = $1 AND exercise_id = $2`,
		fileID, acc.authoringExerciseID(),
	).Scan(&f.ID, &f.ExerciseID, &f.Path)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCompilationFileNotFound
		}
		slog.ErrorContext(ctx, "error fetching compilation file",
			slog.Int64("compilation_file_id", fileID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	f.Filename = f.Path
	return &f, nil
}

/*
CreateCompilationFile stores a compilation file for an exercise owned by the
caller. A file with the same path is replaced.
*/
func CreateCompilationFile(
	ctx context.Context, exerciseID int64, file UploadedFile, claims map[string]any,
) (*models.CompilationFile, error) {
	acc, err := requireExerciseAuthor(ctx, exerciseID, claims)
	if err != nil {
		return nil, err
	}
	authoringID := acc.authoringExerciseID()

	name := storage.FileBasename(file.Filename)
	key := storage.CompilationFileKey(authoringID, name)

	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		slog.ErrorContext(ctx, "error initializing compilation file transaction",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM exercises_compilation_files
		WHERE exercise_id = $1 AND path = $2`, authoringID, name,
	); err != nil {
		slog.ErrorContext(ctx, "error replacing compilation file row",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	var fileID int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO exercises_compilation_files (exercise_id, path)
		VALUES ($1, $2)
		RETURNING id`, authoringID, name,
	).Scan(&fileID); err != nil {
		slog.ErrorContext(ctx, "error inserting compilation file",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	// A file with this path may already exist and share the key; the replacer
	// backs up the previous object so the transaction can be rolled back.
	replacer := storage.NewObjectReplacer(ctx, storage.FileOps)

	if err := replacer.Replace(key, file.Data, file.ContentType); err != nil {
		slog.ErrorContext(ctx, "failed to upload compilation file",
			slog.String("s3_key", key),
			slog.String("error", err.Error()),
		)
		replacer.Restore()
		return nil, ErrServer
	}

	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx, "error committing compilation file",
			slog.String("error", err.Error()),
		)
		replacer.Restore()
		return nil, ErrServer
	}

	replacer.Discard()

	return &models.CompilationFile{ID: fileID, ExerciseID: authoringID, Path: name, Filename: name}, nil
}

/*
DeleteCompilationFile removes a compilation file (row and S3 object).
*/
func DeleteCompilationFile(
	ctx context.Context, exerciseID, fileID int64, claims map[string]any,
) error {
	acc, err := requireExerciseAuthor(ctx, exerciseID, claims)
	if err != nil {
		return err
	}
	authoringID := acc.authoringExerciseID()

	var path string
	err = database.DB.QueryRowContext(ctx, `
		DELETE FROM exercises_compilation_files
		WHERE id = $1 AND exercise_id = $2
		RETURNING path`, fileID, authoringID,
	).Scan(&path)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrCompilationFileNotFound
		}
		slog.ErrorContext(ctx, "error deleting compilation file",
			slog.Int64("compilation_file_id", fileID),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	deleteFileObjectBestEffort(ctx, storage.CompilationFileKey(authoringID, path))
	return nil
}

func deleteFileObjectBestEffort(ctx context.Context, key string) {
	if err := storage.DeleteFileObject(ctx, key); err != nil {
		slog.ErrorContext(ctx, "failed to clean up file object",
			slog.String("s3_key", key),
			slog.String("error", err.Error()),
		)
	}
}
