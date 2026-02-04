package session

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// AWSS3Client implements the S3Client interface using AWS SDK v2
type AWSS3Client struct {
	s3Client *s3.Client
}

// NewAWSS3Client creates a new AWS S3 client
func NewAWSS3Client(s3Client *s3.Client) *AWSS3Client {
	return &AWSS3Client{
		s3Client: s3Client,
	}
}

// GetObject retrieves an object from S3
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

// PutObject uploads an object to S3
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

// HeadObject checks if an object exists in S3
func (c *AWSS3Client) HeadObject(ctx context.Context, bucket, key string) error {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	_, err := c.s3Client.HeadObject(ctx, input)
	if err != nil {
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return fmt.Errorf("object not found")
		}
		return fmt.Errorf("failed to head object %s in bucket %s: %w", key, bucket, err)
	}

	return nil
}

// DeleteObject removes an object from S3
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

// ListObjects lists objects with a given prefix in S3
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

// MockS3Client provides a mock implementation for testing
type MockS3Client struct {
	storage map[string][]byte
}

// NewMockS3Client creates a new mock S3 client for testing
func NewMockS3Client() *MockS3Client {
	return &MockS3Client{
		storage: make(map[string][]byte),
	}
}

// GetObject retrieves an object from mock storage
func (m *MockS3Client) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	fullKey := fmt.Sprintf("%s/%s", bucket, key)
	data, exists := m.storage[fullKey]
	if !exists {
		return nil, fmt.Errorf("object not found")
	}
	return data, nil
}

// PutObject stores an object in mock storage
func (m *MockS3Client) PutObject(ctx context.Context, bucket, key string, data []byte) error {
	fullKey := fmt.Sprintf("%s/%s", bucket, key)
	m.storage[fullKey] = make([]byte, len(data))
	copy(m.storage[fullKey], data)
	return nil
}

// HeadObject checks if an object exists in mock storage
func (m *MockS3Client) HeadObject(ctx context.Context, bucket, key string) error {
	fullKey := fmt.Sprintf("%s/%s", bucket, key)
	_, exists := m.storage[fullKey]
	if !exists {
		return fmt.Errorf("object not found")
	}
	return nil
}

// DeleteObject removes an object from mock storage
func (m *MockS3Client) DeleteObject(ctx context.Context, bucket, key string) error {
	fullKey := fmt.Sprintf("%s/%s", bucket, key)
	delete(m.storage, fullKey)
	return nil
}

// ListObjects lists objects with a given prefix in mock storage
func (m *MockS3Client) ListObjects(ctx context.Context, bucket, prefix string) ([]string, error) {
	searchPrefix := fmt.Sprintf("%s/%s", bucket, prefix)
	var keys []string

	for fullKey := range m.storage {
		if strings.HasPrefix(fullKey, searchPrefix) {
			// Remove bucket prefix to return just the key
			key := strings.TrimPrefix(fullKey, bucket+"/")
			keys = append(keys, key)
		}
	}

	return keys, nil
}
