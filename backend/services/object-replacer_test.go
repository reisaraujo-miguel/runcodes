package services

import (
	"context"
	"errors"
	"io"
	"sort"
	"strings"
	"testing"
)

// fakeObjectOps is an in-memory objectOps used to exercise the in-place
// replacement rollback without S3.
type fakeObjectOps struct {
	objects    map[string]string
	failExists map[string]bool
	failCopy   map[string]bool
	failPut    map[string]bool
}

func newFakeObjectOps() *fakeObjectOps {
	return &fakeObjectOps{
		objects:    map[string]string{},
		failExists: map[string]bool{},
		failCopy:   map[string]bool{},
		failPut:    map[string]bool{},
	}
}

func (f *fakeObjectOps) ops() objectOps {
	return objectOps{
		exists: func(_ context.Context, key string) (bool, error) {
			if f.failExists[key] {
				return false, errors.New("exists failed")
			}
			_, ok := f.objects[key]
			return ok, nil
		},
		copy: func(_ context.Context, srcKey, dstKey string) error {
			if f.failCopy[srcKey] {
				return errors.New("copy failed")
			}
			data, ok := f.objects[srcKey]
			if !ok {
				return errors.New("source object missing")
			}
			f.objects[dstKey] = data
			return nil
		},
		put: func(_ context.Context, key string, body io.Reader, _ int64, _ string) error {
			if f.failPut[key] {
				return errors.New("put failed")
			}
			data, err := io.ReadAll(body)
			if err != nil {
				return err
			}
			f.objects[key] = string(data)
			return nil
		},
		remove: func(_ context.Context, key string) error {
			delete(f.objects, key)
			return nil
		},
	}
}

// backupKeys returns the keys that hold a backup copy.
func (f *fakeObjectOps) backupKeys() []string {
	var keys []string
	for key := range f.objects {
		if strings.Contains(key, ".backup-") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func TestObjectReplacerRestoresPreviousObject(t *testing.T) {
	fake := newFakeObjectOps()
	fake.objects["case/in"] = "old"
	replacer := newObjectReplacer(context.Background(), fake.ops())

	if err := replacer.replace("case/in", []byte("new"), "text/plain"); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := fake.objects["case/in"]; got != "new" {
		t.Fatalf("object = %q, want new", got)
	}
	if got := fake.backupKeys(); len(got) != 1 {
		t.Fatalf("expected one backup, got %v", got)
	}

	replacer.restore()
	if got := fake.objects["case/in"]; got != "old" {
		t.Fatalf("after restore object = %q, want old", got)
	}
	if got := fake.backupKeys(); len(got) != 0 {
		t.Fatalf("backups not cleaned up: %v", got)
	}
}

func TestObjectReplacerRemovesNewObjectOnRestore(t *testing.T) {
	fake := newFakeObjectOps()
	replacer := newObjectReplacer(context.Background(), fake.ops())

	if err := replacer.replace("case/out", []byte("new"), "text/plain"); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := fake.backupKeys(); len(got) != 0 {
		t.Fatalf("a brand new object must not be backed up: %v", got)
	}

	replacer.restore()
	if _, ok := fake.objects["case/out"]; ok {
		t.Fatal("a newly created object must be removed on restore")
	}
}

func TestObjectReplacerDiscardKeepsNewObject(t *testing.T) {
	fake := newFakeObjectOps()
	fake.objects["case/in"] = "old"
	replacer := newObjectReplacer(context.Background(), fake.ops())

	if err := replacer.replace("case/in", []byte("new"), "text/plain"); err != nil {
		t.Fatalf("replace: %v", err)
	}

	replacer.discard()
	if got := fake.objects["case/in"]; got != "new" {
		t.Fatalf("object = %q, want new", got)
	}
	if got := fake.backupKeys(); len(got) != 0 {
		t.Fatalf("backups not cleaned up: %v", got)
	}

	// discard must also make a later restore a no-op.
	replacer.restore()
	if got := fake.objects["case/in"]; got != "new" {
		t.Fatalf("restore after discard changed the object: %q", got)
	}
}

func TestObjectReplacerRestoresEveryWrite(t *testing.T) {
	fake := newFakeObjectOps()
	fake.objects["case/in"] = "in-old"
	fake.objects["case/files/a.txt"] = "a-old"
	replacer := newObjectReplacer(context.Background(), fake.ops())

	replaces := []struct {
		key  string
		data string
	}{
		{"case/in", "in-new"},
		{"case/out", "out-new"},
		{"case/files/a.txt", "a-new"},
	}
	for _, r := range replaces {
		if err := replacer.replace(r.key, []byte(r.data), "text/plain"); err != nil {
			t.Fatalf("replace %s: %v", r.key, err)
		}
	}

	replacer.restore()

	if got := fake.objects["case/in"]; got != "in-old" {
		t.Fatalf("case/in = %q, want in-old", got)
	}
	if got := fake.objects["case/files/a.txt"]; got != "a-old" {
		t.Fatalf("case/files/a.txt = %q, want a-old", got)
	}
	if _, ok := fake.objects["case/out"]; ok {
		t.Fatal("case/out should have been removed")
	}
	if got := fake.backupKeys(); len(got) != 0 {
		t.Fatalf("backups not cleaned up: %v", got)
	}
}

func TestObjectReplacerPutFailureLeavesPreviousObject(t *testing.T) {
	fake := newFakeObjectOps()
	fake.objects["case/in"] = "old"
	fake.failPut["case/in"] = true
	replacer := newObjectReplacer(context.Background(), fake.ops())

	if err := replacer.replace("case/in", []byte("new"), "text/plain"); err == nil {
		t.Fatal("expected a put error")
	}
	if got := fake.objects["case/in"]; got != "old" {
		t.Fatalf("object = %q, want old (unchanged)", got)
	}
	if got := fake.backupKeys(); len(got) != 0 {
		t.Fatalf("partial backup not cleaned up: %v", got)
	}
}

func TestObjectReplacerBackupFailureLeavesObjectUntouched(t *testing.T) {
	fake := newFakeObjectOps()
	fake.objects["case/in"] = "old"
	fake.failCopy["case/in"] = true
	replacer := newObjectReplacer(context.Background(), fake.ops())

	if err := replacer.replace("case/in", []byte("new"), "text/plain"); err == nil {
		t.Fatal("expected a backup error")
	}
	if got := fake.objects["case/in"]; got != "old" {
		t.Fatalf("object = %q, want old (unchanged)", got)
	}
	if got := fake.backupKeys(); len(got) != 0 {
		t.Fatalf("unexpected backups: %v", got)
	}
}

func TestObjectReplacerExistsFailureAborts(t *testing.T) {
	fake := newFakeObjectOps()
	fake.objects["case/in"] = "old"
	fake.failExists["case/in"] = true
	replacer := newObjectReplacer(context.Background(), fake.ops())

	if err := replacer.replace("case/in", []byte("new"), "text/plain"); err == nil {
		t.Fatal("expected an existence-check error")
	}
	if got := fake.objects["case/in"]; got != "old" {
		t.Fatalf("object changed: %q", got)
	}
}
