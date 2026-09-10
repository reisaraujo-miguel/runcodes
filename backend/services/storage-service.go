package services

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	s3EndpointEnv     = "RUNCODES_S3_ENDPOINT"
	s3RegionEnv       = "RUNCODES_S3_REGION"
	s3KeyEnv          = "RUNCODES_S3_CREDENTIALS_KEY"
	s3SecretEnv       = "RUNCODES_S3_CREDENTIALS_SECRET"
	s3BucketPrefixEnv = "RUNCODES_S3_BUCKET_PREFIX"

	defaultS3Endpoint = "http://seaweed:8333"
	defaultS3Region   = "sa-east-1"
	defaultS3Prefix   = "runcodes"
)

// Buckets holds the bucket names derived from the configured prefix.
type Buckets struct {
	Commits     string
	Cases       string
	Files       string
	OutputFiles string
}

var (
	storageOnce    sync.Once
	storageClient  *s3.Client
	storageBuckets Buckets
	storageErr     error
)

/*
initStorage reads the S3 configuration from the environment and builds a
path-style S3 client (SeaweedFS is S3 compatible but does not support virtual
hosted-style buckets).
*/
func initStorage() error {
	endpoint := os.Getenv(s3EndpointEnv)
	if endpoint == "" {
		endpoint = defaultS3Endpoint
	}

	region := os.Getenv(s3RegionEnv)
	if region == "" {
		region = defaultS3Region
	}

	prefix := strings.Trim(os.Getenv(s3BucketPrefixEnv), "-")
	if prefix == "" {
		prefix = defaultS3Prefix
	}

	opts := []func(*config.LoadOptions) error{config.WithRegion(region)}

	key := os.Getenv(s3KeyEnv)
	secret := os.Getenv(s3SecretEnv)
	if key != "" || secret != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(key, secret, ""),
		))
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return fmt.Errorf("loading S3 configuration: %w", err)
	}

	storageClient = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	storageBuckets = Buckets{
		Commits:     prefix + "-commits",
		Cases:       prefix + "-cases",
		Files:       prefix + "-files",
		OutputFiles: prefix + "-outputfiles",
	}

	ensureBuckets(storageClient, storageBuckets)

	slog.Info("S3 storage configured",
		slog.String("endpoint", endpoint),
		slog.String("region", region),
		slog.String("commits_bucket", storageBuckets.Commits),
	)

	return nil
}

/*
Storage lazily initializes (once) and returns the S3 client and the bucket
names. The first call performs the actual configuration.
*/
func Storage() (*s3.Client, Buckets, error) {
	storageOnce.Do(func() { storageErr = initStorage() })
	if storageErr != nil {
		return nil, Buckets{}, storageErr
	}
	return storageClient, storageBuckets, nil
}

/*
ensureBuckets best-effort creates the four buckets the platform uses. It is
idempotent: an already-existing bucket (or one we cannot create) only logs, so
startup never fails because of it. This keeps a fresh SeaweedFS usable without
manual provisioning.
*/
func ensureBuckets(client *s3.Client, buckets Buckets) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, name := range []string{
		buckets.Commits, buckets.Cases, buckets.Files, buckets.OutputFiles,
	} {
		if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(name),
		}); err != nil {
			slog.Debug("could not create S3 bucket (it may already exist)",
				slog.String("bucket", name),
				slog.String("error", err.Error()),
			)
		}
	}
}

/*
UploadCommitSource uploads the submitted source file to the commits bucket.
*/
func UploadCommitSource(
	ctx context.Context, key string, body io.Reader, size int64, contentType string,
) error {
	client, buckets, err := Storage()
	if err != nil {
		return err
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if _, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(buckets.Commits),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	}); err != nil {
		return fmt.Errorf("uploading commit source: %w", err)
	}

	return nil
}

/*
DeleteCommitSource removes an object from the commits bucket. It is used to
compensate a failed submission (best effort).
*/
func DeleteCommitSource(ctx context.Context, key string) error {
	client, buckets, err := Storage()
	if err != nil {
		return err
	}

	if _, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(buckets.Commits),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("deleting commit source: %w", err)
	}

	return nil
}
