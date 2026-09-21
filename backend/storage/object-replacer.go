package storage

import (
	"bytes"
	"context"
	"io"
	"log/slog"
)

// replacedObject records an in-place object write so it can be undone if the
// surrounding database transaction fails. backupKey holds a copy of the object
// that was at key before the write, and is empty when the key did not exist.
type replacedObject struct {
	key       string
	backupKey string
}

/*
objectOps bundles the per-bucket object operations used to replace an object in
place with a rollback backup. It is a struct of functions so tests can swap in an
in-memory implementation.
*/
type objectOps struct {
	exists func(ctx context.Context, key string) (bool, error)
	copy   func(ctx context.Context, srcKey, dstKey string) error
	put    func(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	remove func(ctx context.Context, key string) error
}

var (
	// CaseOps operates on test-case objects in the cases bucket.
	CaseOps = objectOps{
		exists: CaseObjectExists,
		copy:   CopyCaseObject,
		put:    PutCaseObject,
		remove: DeleteCaseObject,
	}
	// FileOps operates on compilation files and attachments in the files bucket.
	FileOps = objectOps{
		exists: FileObjectExists,
		copy:   CopyFileObject,
		put:    PutFileObject,
		remove: DeleteFileObject,
	}
)

/*
ObjectReplacer overwrites objects in place, backing up their previous content so
every write can be undone (Restore) or finalized (Discard) once the surrounding
database transaction has resolved. Reusing the final keys keeps the judge's
`<case_id>/in`, `<case_id>/out` and `<case_id>/files/<name>` contract.
*/
type ObjectReplacer struct {
	ctx   context.Context
	ops   objectOps
	items []replacedObject
}

// NewObjectReplacer returns a replacer that writes through ops.
func NewObjectReplacer(ctx context.Context, ops objectOps) *ObjectReplacer {
	return &ObjectReplacer{ctx: ctx, ops: ops}
}

/*
Replace writes data to key, keeping a backup of the previous object when there
was one. If the write fails the partial backup is removed and the error returned,
leaving the previous object untouched.
*/
func (r *ObjectReplacer) Replace(key string, data []byte, contentType string) error {
	item := replacedObject{key: key}

	exists, err := r.ops.exists(r.ctx, key)
	if err != nil {
		return err
	}
	if exists {
		item.backupKey = backupKeyFor(key)
		if err := r.ops.copy(r.ctx, key, item.backupKey); err != nil {
			return err
		}
	}

	if err := r.ops.put(
		r.ctx, key, bytes.NewReader(data), int64(len(data)), contentType,
	); err != nil {
		r.remove(item.backupKey)
		return err
	}

	r.items = append(r.items, item)
	return nil
}

/*
Restore undoes every in-place write, putting the backed-up objects back and
removing the ones that were newly created. It is a no-op after Discard.
*/
func (r *ObjectReplacer) Restore() {
	for _, item := range r.items {
		if item.backupKey == "" {
			r.remove(item.key)
			continue
		}

		if err := r.ops.copy(r.ctx, item.backupKey, item.key); err != nil {
			slog.ErrorContext(r.ctx, "failed to restore object",
				slog.String("s3_key", item.key),
				slog.String("error", err.Error()),
			)
		}
		r.remove(item.backupKey)
	}

	r.items = nil
}

/*
Discard drops the backups once the transaction has committed.
*/
func (r *ObjectReplacer) Discard() {
	for _, item := range r.items {
		r.remove(item.backupKey)
	}

	r.items = nil
}

// remove deletes a key best-effort, ignoring an empty key.
func (r *ObjectReplacer) remove(key string) {
	if key == "" {
		return
	}

	if err := r.ops.remove(r.ctx, key); err != nil {
		slog.ErrorContext(r.ctx, "failed to remove object",
			slog.String("s3_key", key),
			slog.String("error", err.Error()),
		)
	}
}
