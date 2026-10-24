package services

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"regexp"
	"strings"

	"github.com/runcodes-icmc/runcodes/cache"
	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/validation"

	"github.com/lib/pq"
	"github.com/lib/pq/pqerror"
)

const (
	// MaxFileTypeNameLength bounds the display name of a catalog entry.
	MaxFileTypeNameLength = 100
	// MaxFileTypeExtensionLength bounds the extension. It accommodates compound
	// extensions such as "omp.cpp" or "mpi.c".
	MaxFileTypeExtensionLength = 24
)

// fileTypeExtensionPattern accepts an extension without its leading dot. The
// extension is the part after the last dot of the submitted file name, so it
// may contain inner dots ("omp.cpp") but never a path separator or a space.
var fileTypeExtensionPattern = regexp.MustCompile(
	`^[a-z0-9][a-z0-9.+#_-]*$`,
)

/*
normalizeFileTypeExtension canonicalises the catalog key: the extension is
stored without the leading dot and in lower case so a professor can write ".PY"
in one place and the submission check still matches "main.py".
*/
func normalizeFileTypeExtension(extension string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(extension))
	value = strings.TrimPrefix(value, ".")

	if err := validation.ValidateRequiredString(
		value, MaxFileTypeExtensionLength,
	); err != nil {
		return "", err
	}
	if !fileTypeExtensionPattern.MatchString(value) || strings.Contains(value, "..") {
		return "", errors.New("the extension may contain letters, digits, dots, '+', '#', '_' and '-'")
	}

	return value, nil
}

/*
AdminListAllowedFileTypes returns the whole catalog, including entries hidden
from students and professors. It backs the admin page, so the public
ListAllowedFileTypes cache is intentionally bypassed.
*/
func AdminListAllowedFileTypes(
	ctx context.Context,
) ([]models.AllowedFileType, error) {
	rows, err := database.DB.QueryContext(ctx, `
		SELECT id, name, extension, is_compilable, is_available
		FROM allowed_file_types
		ORDER BY name`)
	if err != nil {
		slog.ErrorContext(ctx, "error listing the file type catalog",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	types := make([]models.AllowedFileType, 0, 24)
	for rows.Next() {
		var fileType models.AllowedFileType
		if err := rows.Scan(
			&fileType.ID, &fileType.Name, &fileType.Extension,
			&fileType.IsCompilable, &fileType.IsAvailable,
		); err != nil {
			slog.ErrorContext(ctx, "error scanning a file type",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		types = append(types, fileType)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating the file type catalog",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	return types, nil
}

/*
CreateAllowedFileType registers a new extension in the catalog. The change is
effective immediately because the public catalog cache entry is dropped.
*/
func CreateAllowedFileType(
	ctx context.Context, req *models.CreateAllowedFileTypeRequest,
) (*models.AllowedFileType, error) {
	name := strings.TrimSpace(req.Name)
	if err := validation.ValidateRequiredString(
		name, MaxFileTypeNameLength,
	); err != nil {
		return nil, err
	}

	extension, err := normalizeFileTypeExtension(req.Extension)
	if err != nil {
		return nil, err
	}

	isCompilable := true
	if req.IsCompilable != nil {
		isCompilable = *req.IsCompilable
	}
	isAvailable := true
	if req.IsAvailable != nil {
		isAvailable = *req.IsAvailable
	}

	fileType := models.AllowedFileType{
		Name:         name,
		Extension:    extension,
		IsCompilable: isCompilable,
		IsAvailable:  isAvailable,
	}

	err = database.DB.QueryRowContext(ctx, `
		INSERT INTO allowed_file_types (name, extension, is_compilable, is_available)
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		fileType.Name, fileType.Extension, fileType.IsCompilable,
		fileType.IsAvailable,
	).Scan(&fileType.ID)
	if err != nil {
		if isFileTypeUniqueViolation(err) {
			return nil, ErrFileTypeExists
		}
		slog.ErrorContext(ctx, "error creating a file type",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	cache.Delete(ctx, cache.KeyAllowedFileTypes)

	slog.InfoContext(ctx, "file type registered",
		slog.Int64("file_type_id", fileType.ID),
		slog.String("extension", fileType.Extension),
	)

	return &fileType, nil
}

/*
UpdateAllowedFileType applies a partial update to a catalog entry.
*/
func UpdateAllowedFileType(
	ctx context.Context, id int64, req *models.UpdateAllowedFileTypeRequest,
) (*models.AllowedFileType, error) {
	if id <= 0 {
		return nil, ErrFileTypeNotFound
	}

	fileType, err := loadAllowedFileType(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		fileType.Name = strings.TrimSpace(*req.Name)
		if err := validation.ValidateRequiredString(
			fileType.Name, MaxFileTypeNameLength,
		); err != nil {
			return nil, err
		}
	}
	if req.Extension != nil {
		fileType.Extension, err = normalizeFileTypeExtension(*req.Extension)
		if err != nil {
			return nil, err
		}
	}
	if req.IsCompilable != nil {
		fileType.IsCompilable = *req.IsCompilable
	}
	if req.IsAvailable != nil {
		fileType.IsAvailable = *req.IsAvailable
	}

	_, err = database.DB.ExecContext(ctx, `
		UPDATE allowed_file_types
		SET name = $1, extension = $2, is_compilable = $3, is_available = $4
		WHERE id = $5`,
		fileType.Name, fileType.Extension, fileType.IsCompilable,
		fileType.IsAvailable, fileType.ID,
	)
	if err != nil {
		if isFileTypeUniqueViolation(err) {
			return nil, ErrFileTypeExists
		}
		slog.ErrorContext(ctx, "error updating a file type",
			slog.Int64("file_type_id", id),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	cache.Delete(ctx, cache.KeyAllowedFileTypes)

	return fileType, nil
}

/*
DeleteAllowedFileType removes a catalog entry that no exercise references.
Deleting a referenced entry would silently strip the allowed types of every
exercise that uses it, so the caller is told to disable it instead.
*/
func DeleteAllowedFileType(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrFileTypeNotFound
	}

	var references int
	err := database.DB.QueryRowContext(ctx, `
		SELECT count(*) FROM exercises_allowed_file_types
		WHERE allowed_file_type_id = $1`, id,
	).Scan(&references)
	if err != nil {
		slog.ErrorContext(ctx, "error counting file type references",
			slog.Int64("file_type_id", id),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}
	if references > 0 {
		return ErrFileTypeInUse
	}

	result, err := database.DB.ExecContext(ctx, `
		DELETE FROM allowed_file_types WHERE id = $1`, id,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error deleting a file type",
			slog.Int64("file_type_id", id),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return ErrServer
	}
	if affected == 0 {
		return ErrFileTypeNotFound
	}

	cache.Delete(ctx, cache.KeyAllowedFileTypes)

	return nil
}

func loadAllowedFileType(
	ctx context.Context, id int64,
) (*models.AllowedFileType, error) {
	var fileType models.AllowedFileType
	err := database.DB.QueryRowContext(ctx, `
		SELECT id, name, extension, is_compilable, is_available
		FROM allowed_file_types WHERE id = $1`, id,
	).Scan(
		&fileType.ID, &fileType.Name, &fileType.Extension,
		&fileType.IsCompilable, &fileType.IsAvailable,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrFileTypeNotFound
		}
		slog.ErrorContext(ctx, "error fetching a file type",
			slog.Int64("file_type_id", id),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	return &fileType, nil
}

func isFileTypeUniqueViolation(err error) bool {
	pgErr, ok := err.(*pq.Error)
	return ok && pgErr.Code == pqerror.UniqueViolation
}
