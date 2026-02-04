package storage_manager

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/aws/smithy-go"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Client defines the S3 operations needed by S3FileProvider.
type S3Client interface {
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
	PutObject(ctx context.Context, bucket, key string, data []byte) error
	HeadObject(ctx context.Context, bucket, key string) error
	DeleteObject(ctx context.Context, bucket, key string) error
	ListObjects(ctx context.Context, bucket, prefix string) ([]string, error)
}

// AWSS3Client implements the S3Client interface using AWS SDK v2.
type AWSS3Client struct {
	s3Client *s3.Client
}

// NewAWSS3Client creates a new AWS S3 client.
func NewAWSS3Client(s3Client *s3.Client) *AWSS3Client {
	return &AWSS3Client{
		s3Client: s3Client,
	}
}

// GetObject retrieves an object from S3.
func (c *AWSS3Client) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	result, err := c.s3Client.GetObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get object %s from bucket %s: %w", key, bucket, err)
	}
	defer func() { _ = result.Body.Close() }()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read object body: %w", err)
	}

	return data, nil
}

// PutObject uploads an object to S3.
func (c *AWSS3Client) PutObject(ctx context.Context, bucket, key string, data []byte) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	}

	_, err := c.s3Client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to put object %s to bucket %s: %w", key, bucket, err)
	}

	return nil
}

// ErrNotFound is returned when an object does not exist in S3.
var ErrNotFound = errors.New("object not found")

// HeadObject checks if an object exists in S3.
// Returns ErrNotFound if the object doesn't exist.
func (c *AWSS3Client) HeadObject(ctx context.Context, bucket, key string) error {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	_, err := c.s3Client.HeadObject(ctx, input)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			// Check for "NotFound" error code
			if apiErr.ErrorCode() == "NotFound" {
				return ErrNotFound
			}
		}

		return fmt.Errorf("failed to head object %s in bucket %s: %w", key, bucket, err)
	}

	return nil
}

// DeleteObject removes an object from S3.
func (c *AWSS3Client) DeleteObject(ctx context.Context, bucket, key string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	_, err := c.s3Client.DeleteObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete object %s from bucket %s: %w", key, bucket, err)
	}

	return nil
}

// ListObjects lists objects with a given prefix in S3.
// Returns an empty list if the bucket/prefix doesn't exist or has no matching objects.
func (c *AWSS3Client) ListObjects(ctx context.Context, bucket, prefix string) ([]string, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	}

	var keys []string
	paginator := s3.NewListObjectsV2Paginator(c.s3Client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			// Handle "not found" type errors gracefully - return empty list
			// This allows the application to start even if the bucket/prefix
			// hasn't been initialized yet (e.g., no skills have been created)
			var noSuchBucket *types.NoSuchBucket
			if errors.As(err, &noSuchBucket) {
				return []string{}, nil
			}
			var notFound *types.NotFound
			if errors.As(err, &notFound) {
				return []string{}, nil
			}
			return nil, fmt.Errorf("failed to list objects with prefix %s in bucket %s: %w", prefix, bucket, err)
		}

		for _, obj := range page.Contents {
			if obj.Key != nil {
				keys = append(keys, *obj.Key)
			}
		}
	}

	return keys, nil
}
