// Package storage moves the judge's inputs and outputs in and out of SeaweedFS
// over the S3 API: the submitted source, test-case files and exercise
// compilation files are read, and the files a submission generates are written
// back so the backend can serve them to the author.
package storage

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/runcodes-icmc/judge/internal/config"
	"github.com/runcodes-icmc/judge/internal/telemetry"
)

// S3 is a path-style S3 client for SeaweedFS.
type S3 struct {
	client *s3.Client
	cfg    config.S3Config
}

// NewS3 builds the client. Static credentials are always used, matching the
// platform's SeaweedFS setup.
func NewS3(ctx context.Context, cfg config.S3Config) (*S3, error) {
	// load the AWS SDK config with static credentials and the given region
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey, cfg.SecretKey, "",
		)),
		// Propagate the claimed commit's trace context to every S3 request.
		awsconfig.WithHTTPClient(&http.Client{
			Timeout:   60 * time.Second,
			Transport: telemetry.Transport(http.DefaultTransport),
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	// create the S3 client with the custom endpoint and path-style addressing
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true
	})

	return &S3{client: client, cfg: cfg}, nil
}

// download fetches the object from S3 and writes it to dest, creating parent directories as needed.
func (s *S3) download(ctx context.Context, bucket, key, dest string) error {
	// fetch the object from S3
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("get s3://%s/%s: %w", bucket, key, err)
	}
	defer out.Body.Close()

	// create parent directories for dest if they don't exist
	if err := os.MkdirAll(filepath.Dir(dest), 0o777); err != nil {
		return fmt.Errorf("create dir for %s: %w", dest, err)
	}

	// create the destination file and copy the object body into it
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, out.Body); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}

	return nil
}

// downloadBytes fetches the object from S3 and returns its contents as a byte slice.
/* [unused for now, but could be useful in the future]
func (s *S3) downloadBytes(ctx context.Context, bucket, key string) ([]byte, error) {
	// fetch the object from S3
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get s3://%s/%s: %w", bucket, key, err)
	}
	defer out.Body.Close()

	return io.ReadAll(out.Body)
}
*/

// FetchCommitSource downloads the submitted source file.
func (s *S3) FetchCommitSource(ctx context.Context, key, dest string) error {
	return s.download(ctx, s.cfg.CommitsBucket(), key, dest)
}

// FetchCaseInput downloads `<case>/in` into dest.
func (s *S3) FetchCaseInput(ctx context.Context, caseID int64, dest string) error {
	return s.download(ctx, s.cfg.CasesBucket(), fmt.Sprintf("%d/in", caseID), dest)
}

// FetchCaseOutput downloads `<case>/out` into dest.
func (s *S3) FetchCaseOutput(ctx context.Context, caseID int64, dest string) error {
	return s.download(ctx, s.cfg.CasesBucket(), fmt.Sprintf("%d/out", caseID), dest)
}

// FetchCaseFiles downloads a test case's extra files into dir.
func (s *S3) FetchCaseFiles(ctx context.Context, caseID int64, files []string, dir string) error {
	for _, name := range files {
		key := fmt.Sprintf("%d/files/%s", caseID, name)
		dest := filepath.Join(dir, filepath.Base(name))

		if err := s.download(ctx, s.cfg.CasesBucket(), key, dest); err != nil {
			return err
		}
	}

	return nil
}

// FetchCompilationFile downloads an exercise compilation file.
func (s *S3) FetchCompilationFile(ctx context.Context, exerciseID int64, path, dest string) error {
	key := fmt.Sprintf("compilationfiles/%d/%s", exerciseID, path)
	return s.download(ctx, s.cfg.FilesBucket(), key, dest)
}

// UploadGeneratedFile stores one file a submission produced under the commit's
// prefix in the outputfiles bucket (`<commit_id>/<rel>`) and returns its size.
//
// The key keeps the workspace layout the runner collected the file in
// (`test_<case_id>/<relative>` or `outputfiles/<relative>`), so the backend can
// present a file's origin from its path alone. rel is built by the judge from a
// workspace walk, never from user input, but it is still cleaned to a
// slash-separated relative path before it becomes a key.
func (s *S3) UploadGeneratedFile(
	ctx context.Context, commitID int64, rel, src string,
) (int64, error) {
	clean := path.Clean(filepath.ToSlash(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return 0, fmt.Errorf("invalid generated file path %q", rel)
	}

	f, err := os.Open(src)
	if err != nil {
		return 0, fmt.Errorf("open %s: %w", src, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat %s: %w", src, err)
	}
	if !info.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	// Generated content is arbitrary bytes, so it is stored opaquely; the
	// backend serves it as a download and never renders it.
	contentType := mime.TypeByExtension(path.Ext(clean))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	key := fmt.Sprintf("%d/%s", commitID, clean)
	if _, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.cfg.OutputsBucket()),
		Key:           aws.String(key),
		Body:          f,
		ContentLength: aws.Int64(info.Size()),
		ContentType:   aws.String(contentType),
	}); err != nil {
		return 0, fmt.Errorf("put s3://%s/%s: %w", s.cfg.OutputsBucket(), key, err)
	}

	return info.Size(), nil
}
