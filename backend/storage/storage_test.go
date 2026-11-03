package storage

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
)

// fakeS3Server is a minimal path-style S3 endpoint used by the storage tests. It
// implements just the operations storage.go performs: CreateBucket,
// PutObject, GetObject, HeadObject, DeleteObject, CopyObject, ListObjectsV2 and
// DeleteObjects.
type fakeS3Server struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func (f *fakeS3Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	bucket, key := splitS3Path(r.URL.Path)

	f.mu.Lock()
	defer f.mu.Unlock()

	switch {
	case key == "" && r.Method == http.MethodPut:
		// CreateBucket.
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodPut:
		if source := r.Header.Get("x-amz-copy-source"); source != "" {
			f.copyObject(w, bucket, key, source)
			return
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		f.objects[bucket+"/"+key] = data
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodHead:
		data, ok := f.objects[bucket+"/"+key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2":
		f.listObjects(w, bucket, r.URL.Query().Get("prefix"))
	case r.Method == http.MethodPost && r.URL.Query().Has("delete"):
		f.deleteObjects(w, bucket, r.Body)
	case r.Method == http.MethodGet:
		data, ok := f.objects[bucket+"/"+key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	case r.Method == http.MethodDelete:
		delete(f.objects, bucket+"/"+key)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotImplemented)
	}
}

func (f *fakeS3Server) copyObject(w http.ResponseWriter, bucket, key, source string) {
	if decoded, err := url.PathUnescape(source); err == nil {
		source = decoded
	}
	source = strings.TrimPrefix(source, "/")

	data, ok := f.objects[source]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	f.objects[bucket+"/"+key] = append([]byte(nil), data...)

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>`+
		`<CopyObjectResult><ETag>"etag"</ETag>`+
		`<LastModified>2024-01-01T00:00:00.000Z</LastModified></CopyObjectResult>`)
}

// listObjects answers ListObjectsV2 with every key under prefix. Pagination is
// not modelled: the tests store a handful of objects.
func (f *fakeS3Server) listObjects(w http.ResponseWriter, bucket, prefix string) {
	base := bucket + "/"

	var contents strings.Builder
	count := 0
	for name, data := range f.objects {
		key, ok := strings.CutPrefix(name, base)
		if !ok || !strings.HasPrefix(key, prefix) {
			continue
		}
		contents.WriteString("<Contents><Key>" + xmlEscape(key) + "</Key><Size>" +
			strconv.Itoa(len(data)) + "</Size>" +
			"<LastModified>2024-01-01T00:00:00.000Z</LastModified></Contents>")
		count++
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>`+
		`<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">`+
		`<Name>`+xmlEscape(bucket)+`</Name><Prefix>`+xmlEscape(prefix)+`</Prefix>`+
		`<KeyCount>`+strconv.Itoa(count)+`</KeyCount><MaxKeys>1000</MaxKeys>`+
		`<IsTruncated>false</IsTruncated>`+contents.String()+
		`</ListBucketResult>`)
}

// deleteObjects answers a multi-object DELETE, removing every key the request
// named.
func (f *fakeS3Server) deleteObjects(w http.ResponseWriter, bucket string, body io.Reader) {
	var request struct {
		Objects []struct {
			Key string `xml:"Key"`
		} `xml:"Object"`
	}
	if err := xml.NewDecoder(body).Decode(&request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	for _, object := range request.Objects {
		delete(f.objects, bucket+"/"+object.Key)
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>`+
		`<DeleteResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"></DeleteResult>`)
}

// xmlEscape escapes a value for inclusion in the fake's XML responses.
func xmlEscape(value string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(value))
	return buf.String()
}

func (f *fakeS3Server) get(bucket, key string) ([]byte, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, ok := f.objects[bucket+"/"+key]
	return data, ok
}

// splitS3Path splits a path-style request path ("/bucket/key") into its bucket
// and key. An absent key means the request targets the bucket itself.
func splitS3Path(p string) (bucket, key string) {
	p = strings.TrimPrefix(p, "/")
	parts := strings.SplitN(p, "/", 2)
	bucket = parts[0]
	if len(parts) == 2 {
		key = parts[1]
	}
	return bucket, key
}

// withFakeS3 points the storage layer at an in-memory S3 endpoint for the
// duration of the test.
func withFakeS3(t *testing.T) *fakeS3Server {
	t.Helper()

	fake := &fakeS3Server{objects: map[string][]byte{}}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)

	previous := config.C
	config.C = &config.Config{S3: config.S3Config{
		Endpoint:     server.URL,
		Region:       "us-east-1",
		AccessKey:    "test",
		SecretKey:    "test",
		BucketPrefix: "test",
	}}
	t.Cleanup(func() { config.C = previous })

	resetStorageForTest()
	t.Cleanup(resetStorageForTest)

	return fake
}

// resetStorageForTest clears the lazily-initialized storage state so a test can
// choose a different endpoint.
func resetStorageForTest() {
	storageOnce = sync.Once{}
	storageClient = nil
	storageBuckets = Buckets{}
	storageErr = nil
}

func TestCaseObjectStorageRoundTrip(t *testing.T) {
	fake := withFakeS3(t)
	ctx := context.Background()

	_, buckets, err := Client()
	if err != nil {
		t.Fatalf("Client: %v", err)
	}

	if err := PutCaseObject(ctx, "1/in", bytes.NewReader([]byte("hello")), 5, "text/plain"); err != nil {
		t.Fatalf("PutCaseObject: %v", err)
	}
	if data, ok := fake.get(buckets.Cases, "1/in"); !ok || string(data) != "hello" {
		t.Fatalf("stored object = %q (ok=%v), want hello", data, ok)
	}

	exists, err := CaseObjectExists(ctx, "1/in")
	if err != nil {
		t.Fatalf("CaseObjectExists: %v", err)
	}
	if !exists {
		t.Fatal("expected 1/in to exist")
	}

	if err := CopyCaseObject(ctx, "1/in", "1/in.backup"); err != nil {
		t.Fatalf("CopyCaseObject: %v", err)
	}
	if data, ok := fake.get(buckets.Cases, "1/in.backup"); !ok || string(data) != "hello" {
		t.Fatalf("backup object = %q (ok=%v), want hello", data, ok)
	}

	if err := DeleteCaseObject(ctx, "1/in"); err != nil {
		t.Fatalf("DeleteCaseObject: %v", err)
	}
	exists, err = CaseObjectExists(ctx, "1/in")
	if err != nil {
		t.Fatalf("CaseObjectExists after delete: %v", err)
	}
	if exists {
		t.Fatal("expected 1/in to be gone")
	}
}

func TestFileObjectStorageRoundTrip(t *testing.T) {
	fake := withFakeS3(t)
	ctx := context.Background()

	_, buckets, err := Client()
	if err != nil {
		t.Fatalf("Client: %v", err)
	}

	if err := PutFileObject(
		ctx, "compilationfiles/7/helper.h", bytes.NewReader([]byte("int x;")), 6, "text/plain",
	); err != nil {
		t.Fatalf("PutFileObject: %v", err)
	}

	exists, err := FileObjectExists(ctx, "compilationfiles/7/helper.h")
	if err != nil {
		t.Fatalf("FileObjectExists: %v", err)
	}
	if !exists {
		t.Fatal("expected the compilation file to exist")
	}

	if err := CopyFileObject(ctx, "compilationfiles/7/helper.h", "compilationfiles/7/backup"); err != nil {
		t.Fatalf("CopyFileObject: %v", err)
	}
	if data, ok := fake.get(buckets.Files, "compilationfiles/7/backup"); !ok || string(data) != "int x;" {
		t.Fatalf("backup object = %q (ok=%v), want int x;", data, ok)
	}

	if err := DeleteFileObject(ctx, "compilationfiles/7/helper.h"); err != nil {
		t.Fatalf("DeleteFileObject: %v", err)
	}
}

func TestCommitObjectStorageRoundTrip(t *testing.T) {
	fake := withFakeS3(t)
	ctx := context.Background()

	_, buckets, err := Client()
	if err != nil {
		t.Fatalf("Client: %v", err)
	}

	if err := UploadCommitSource(
		ctx, "uuid/main.py", bytes.NewReader([]byte("print('oi')")), 11, "text/plain",
	); err != nil {
		t.Fatalf("UploadCommitSource: %v", err)
	}

	object, err := GetCommitObject(ctx, "uuid/main.py")
	if err != nil {
		t.Fatalf("GetCommitObject: %v", err)
	}
	defer object.Body.Close()

	data, err := io.ReadAll(object.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(data) != "print('oi')" {
		t.Fatalf("source = %q, want print('oi')", data)
	}
	if _, ok := fake.get(buckets.Commits, "uuid/main.py"); !ok {
		t.Fatal("expected the source in the commits bucket")
	}

	if _, err := GetCommitObject(ctx, "missing/source.py"); !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("GetCommitObject missing = %v, want ErrObjectNotFound", err)
	}
}

func TestObjectExistsMissing(t *testing.T) {
	withFakeS3(t)
	ctx := context.Background()

	exists, err := CaseObjectExists(ctx, "does-not-exist")
	if err != nil {
		t.Fatalf("CaseObjectExists: %v", err)
	}
	if exists {
		t.Fatal("expected a missing cases object")
	}

	exists, err = FileObjectExists(ctx, "does-not-exist")
	if err != nil {
		t.Fatalf("FileObjectExists: %v", err)
	}
	if exists {
		t.Fatal("expected a missing files object")
	}
}

func TestEncodeCopySource(t *testing.T) {
	tests := []struct {
		name   string
		bucket string
		key    string
		want   string
	}{
		{"simple key", "b", "1/in", "b/1/in"},
		{"spaces are escaped per segment", "b", "1/files/some file.txt", "b/1/files/some%20file.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := encodeCopySource(tt.bucket, tt.key); got != tt.want {
				t.Fatalf("encodeCopySource = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBackupKeyFor(t *testing.T) {
	first := backupKeyFor("1/in")
	second := backupKeyFor("1/in")

	if !strings.HasPrefix(first, "1/in.backup-") {
		t.Fatalf("backup key = %q, want a prefix of 1/in.backup-", first)
	}
	if first == second {
		t.Fatalf("expected unique backup keys, both were %q", first)
	}
}

func TestOutputFileStorage(t *testing.T) {
	fake := withFakeS3(t)
	ctx := context.Background()

	_, buckets, err := Client()
	if err != nil {
		t.Fatalf("Client: %v", err)
	}

	objects := map[string]string{
		OutputFileKey(7, "test_1/a.txt"):        "alpha",
		OutputFileKey(7, "test_2/nested/b.txt"): "beta",
		OutputFileKey(8, "test_1/other.txt"):    "other",
	}
	for key, content := range objects {
		fake.objects[buckets.OutputFiles+"/"+key] = []byte(content)
	}

	files, err := ListOutputFiles(ctx, 7)
	if err != nil {
		t.Fatalf("ListOutputFiles: %v", err)
	}
	want := []OutputFile{
		{Path: "test_1/a.txt", Size: 5},
		{Path: "test_2/nested/b.txt", Size: 4},
	}
	if len(files) != len(want) {
		t.Fatalf("files = %v, want %v", files, want)
	}
	for i, expected := range want {
		if files[i].Path != expected.Path || files[i].Size != expected.Size {
			t.Fatalf("files[%d] = %+v, want %+v", i, files[i], expected)
		}
	}

	exists, err := OutputFilesExist(ctx, 7)
	if err != nil || !exists {
		t.Fatalf("OutputFilesExist(7) = %v, %v", exists, err)
	}
	exists, err = OutputFilesExist(ctx, 9)
	if err != nil || exists {
		t.Fatalf("OutputFilesExist(9) = %v, %v", exists, err)
	}

	object, err := GetOutputFile(ctx, 7, "test_1/a.txt")
	if err != nil {
		t.Fatalf("GetOutputFile: %v", err)
	}
	defer object.Body.Close()
	data, err := io.ReadAll(object.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(data) != "alpha" {
		t.Fatalf("content = %q, want alpha", data)
	}

	if _, err := GetOutputFile(ctx, 7, "../8/test_1/other.txt"); !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("traversal error = %v, want ErrObjectNotFound", err)
	}
	if _, err := GetOutputFile(ctx, 7, "test_1/missing.txt"); !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("missing error = %v, want ErrObjectNotFound", err)
	}

	if err := DeleteCommitOutputs(ctx, 7); err != nil {
		t.Fatalf("DeleteCommitOutputs: %v", err)
	}
	if files, err := ListOutputFiles(ctx, 7); err != nil || len(files) != 0 {
		t.Fatalf("files after delete = %v, %v", files, err)
	}
	// Another commit's files must survive.
	if files, err := ListOutputFiles(ctx, 8); err != nil || len(files) != 1 {
		t.Fatalf("files of commit 8 = %v, %v", files, err)
	}
}

func TestDeleteExpiredOutputFiles(t *testing.T) {
	fake := withFakeS3(t)
	ctx := context.Background()

	_, buckets, err := Client()
	if err != nil {
		t.Fatalf("Client: %v", err)
	}

	objects := []string{
		OutputFileKey(7, "test_1/a.txt"),
		OutputFileKey(8, "test_1/b.txt"),
	}
	for _, key := range objects {
		fake.objects[buckets.OutputFiles+"/"+key] = []byte("x")
	}
	// An object in another bucket must never be touched by the sweep.
	fake.objects[buckets.Commits+"/7/main.c"] = []byte("int main(void){}")

	// The fake reports every object as uploaded on 2024-01-01.
	removed, err := DeleteExpiredOutputFiles(ctx, time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("DeleteExpiredOutputFiles: %v", err)
	}
	if removed != 0 {
		t.Fatalf("removed = %d, want 0 (nothing is older than the cutoff)", removed)
	}

	removed, err = DeleteExpiredOutputFiles(ctx, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("DeleteExpiredOutputFiles: %v", err)
	}
	if removed != len(objects) {
		t.Fatalf("removed = %d, want %d", removed, len(objects))
	}
	if files, err := ListOutputFiles(ctx, 7); err != nil || len(files) != 0 {
		t.Fatalf("files after sweep = %v, %v", files, err)
	}
	if _, ok := fake.get(buckets.Commits, "7/main.c"); !ok {
		t.Fatal("the sweep removed an object outside the outputfiles bucket")
	}
}
