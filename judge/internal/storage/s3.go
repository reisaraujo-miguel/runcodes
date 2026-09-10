// Package storage reads the judge's inputs (submitted source, test-case files
// and exercise compilation files) from SeaweedFS over the S3 API. The judge is
// read-only here: artifacts are handed back to the backend through the event
// stream, never written to S3.
package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/runcodes-icmc/judge/internal/config"
)

// S3 is a path-style S3 client for SeaweedFS.
type S3 struct {
	client *s3.Client
	cfg    config.S3Config
}

// NewS3 builds the client. Static credentials are always used, matching the
// platform's SeaweedFS setup.
func NewS3(ctx context.Context, cfg config.S3Config) (*S3, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey, cfg.SecretKey, "",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true
	})
	return &S3{client: client, cfg: cfg}, nil
}

func (s *S3) download(ctx context.Context, bucket, key, dest string) error {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("get s3://%s/%s: %w", bucket, key, err)
	}
	defer out.Body.Close()

	if err := os.MkdirAll(filepath.Dir(dest), 0o777); err != nil {
		return fmt.Errorf("create dir for %s: %w", dest, err)
	}
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

func (s *S3) downloadBytes(ctx context.Context, bucket, key string) ([]byte, error) {
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
