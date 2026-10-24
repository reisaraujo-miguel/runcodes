package services

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/storage"
)

type storageBucket int

const (
	bucketCases storageBucket = iota
	bucketFiles
	bucketCommits
)

func (b storageBucket) String() string {
	switch b {
	case bucketCases:
		return "cases"
	case bucketFiles:
		return "files"
	case bucketCommits:
		return "commits"
	default:
		return "unknown"
	}
}

// storageObject names one object to clean up, and which bucket holds it.
type storageObject struct {
	bucket storageBucket
	key    string
}

// deleteObjectsBestEffort removes a set of objects, grouping them by bucket so a
// class deletion is a handful of batched S3 requests instead of one per object.
// Duplicate keys are collapsed: a user's commits inside an offering they own
// appear both in the class's key set and in the user's.
func deleteObjectsBestEffort(ctx context.Context, objects []storageObject) {
	if _, _, err := storage.Client(); err != nil {
		slog.ErrorContext(ctx, "storage unavailable, skipping object cleanup",
			slog.String("error", err.Error()),
		)
		return
	}

	grouped := make(map[storageBucket][]string)
	seen := make(map[storageBucket]map[string]struct{})
	for _, object := range objects {
		if seen[object.bucket] == nil {
			seen[object.bucket] = make(map[string]struct{})
		}
		if _, duplicate := seen[object.bucket][object.key]; duplicate {
			continue
		}
		seen[object.bucket][object.key] = struct{}{}
		grouped[object.bucket] = append(grouped[object.bucket], object.key)
	}

	for bucket, keys := range grouped {
		var err error
		switch bucket {
		case bucketCases:
			err = storage.DeleteCaseObjects(ctx, keys)
		case bucketFiles:
			err = storage.DeleteFileObjects(ctx, keys)
		case bucketCommits:
			err = storage.DeleteCommitObjects(ctx, keys)
		default:
			err = fmt.Errorf("unknown storage bucket %d", bucket)
		}
		if err != nil {
			slog.ErrorContext(ctx, "error deleting objects",
				slog.String("bucket", bucket.String()),
				slog.Int("objects", len(keys)),
				slog.String("error", err.Error()),
			)
		}
	}
}

/*
offeringStorageKeys collects the objects an offering owns: test case inputs,
outputs and extra files, compilation files, attachments and submitted sources.
They are removed after the rows are gone, so a deleted class does not leave its
authoring content and submissions behind in the object store.
*/
func offeringStorageKeys(ctx context.Context, offeringID int64) ([]storageObject, error) {
	keys := make([]storageObject, 0, 16)
	seenCase := make(map[int64]bool)

	rows, err := database.DB.QueryContext(ctx, `
		SELECT tc.id, COALESCE(files.path, '')
		FROM exercises_test_cases tc
		JOIN exercises e ON e.id = tc.exercise_id
		LEFT JOIN exercises_test_cases_files files
		       ON files.exercise_test_case_id = tc.id
		WHERE e.offering_id = $1`, offeringID,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error collecting test case objects",
			slog.String("error", err.Error()),
			slog.Int64("offering_id", offeringID),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	for rows.Next() {
		var (
			caseID       int64
			attachedPath string
		)
		if err := rows.Scan(&caseID, &attachedPath); err != nil {
			slog.ErrorContext(ctx, "error scanning test case object",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		// Every case has an input and an expected-output object, including text
		// cases: the backend still uploads text objects for compatibility. Legacy
		// text cases may not have one, but deleting a missing key is harmless. The
		// file join can return one row per attached file, so only add the case
		// objects once.
		if !seenCase[caseID] {
			keys = append(keys, storageObject{bucketCases, storage.CaseInputKey(caseID)})
			keys = append(keys, storageObject{bucketCases, storage.CaseOutputKey(caseID)})
			seenCase[caseID] = true
		}
		if attachedPath != "" {
			keys = append(keys, storageObject{
				bucketCases, storage.CaseFileKey(caseID, attachedPath),
			})
		}
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating test case objects",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	rows, err = database.DB.QueryContext(ctx, `
		SELECT files.exercise_id, files.path, 'compilation'
		FROM exercises_compilation_files files
		JOIN exercises e ON e.id = files.exercise_id
		WHERE e.offering_id = $1
		UNION ALL
		SELECT files.exercise_id, files.path, 'attachment'
		FROM exercises_attached_files files
		JOIN exercises e ON e.id = files.exercise_id
		WHERE e.offering_id = $1`, offeringID,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error collecting exercise file objects",
			slog.String("error", err.Error()),
			slog.Int64("offering_id", offeringID),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	for rows.Next() {
		var (
			exerciseID int64
			filePath   string
			kind       string
		)
		if err := rows.Scan(&exerciseID, &filePath, &kind); err != nil {
			slog.ErrorContext(ctx, "error scanning exercise file object",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		if kind == "compilation" {
			keys = append(keys, storageObject{
				bucketFiles, storage.CompilationFileKey(exerciseID, filePath),
			})
		} else {
			keys = append(keys, storageObject{
				bucketFiles, storage.AttachmentKey(exerciseID, filePath),
			})
		}
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating exercise file objects",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	rows, err = database.DB.QueryContext(ctx, `
		SELECT c.s3_key
		FROM commits c
		JOIN exercises e ON e.id = c.exercise_id
		WHERE e.offering_id = $1 AND c.s3_key IS NOT NULL`, offeringID,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error collecting submitted sources",
			slog.String("error", err.Error()),
			slog.Int64("offering_id", offeringID),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			slog.ErrorContext(ctx, "error scanning submitted source",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		keys = append(keys, storageObject{bucketCommits, key})
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating submitted sources",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	return keys, nil
}

/*
ListOfferingMembers lists the people in a class: students who joined with the
code, monitors and co-professors.
*/
