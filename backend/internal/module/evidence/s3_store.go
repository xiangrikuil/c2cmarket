package evidence

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type S3API interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

type S3ObjectStore struct {
	client S3API
	bucket string
}

func NewS3ObjectStore(client S3API, bucket string) (*S3ObjectStore, error) {
	if client == nil || bucket == "" {
		return nil, errors.New("evidence S3 client and bucket are required")
	}
	return &S3ObjectStore{client: client, bucket: bucket}, nil
}

func (s *S3ObjectStore) Put(ctx context.Context, key, contentType string, body []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key),
		Body: bytes.NewReader(body), ContentType: aws.String(contentType),
		ContentLength: aws.Int64(int64(len(body))),
	})
	if err != nil {
		return fmt.Errorf("put evidence object: %w", err)
	}
	return nil
}

func (s *S3ObjectStore) Get(ctx context.Context, key string) (Object, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NoSuchKey" || apiErr.ErrorCode() == "NotFound") {
			return Object{}, ErrObjectNotFound
		}
		return Object{}, fmt.Errorf("get evidence object: %w", err)
	}
	size := int64(-1)
	if result.ContentLength != nil {
		size = *result.ContentLength
	}
	return Object{Body: result.Body, ContentType: aws.ToString(result.ContentType), Size: size}, nil
}

func (s *S3ObjectStore) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return fmt.Errorf("delete evidence object: %w", err)
	}
	return nil
}
