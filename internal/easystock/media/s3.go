package media

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// Config points at an S3-compatible endpoint (AWS, Spaces, MinIO, R2).
type Config struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	PathStyle bool
}

// S3 signs upload and download URLs. Callers never send the object through the API process.
type S3 struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
}

// OpenS3 builds a client. It does not contact the bucket until a later call.
func OpenS3(cfg Config) (*S3, error) {
	if cfg.Bucket == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, errors.New("s3 bucket and credentials are required")
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	options := s3.Options{
		Region:       cfg.Region,
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		UsePathStyle: cfg.PathStyle,
	}
	if cfg.Endpoint != "" {
		options.BaseEndpoint = aws.String(cfg.Endpoint)
	}
	client := s3.New(options)
	return &S3{client: client, presign: s3.NewPresignClient(client), bucket: cfg.Bucket}, nil
}

func (s *S3) PresignPut(ctx context.Context, key, contentType string, size int64, expires time.Duration) (string, error) {
	req, err := s.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func (s *S3) PresignGet(ctx context.Context, key string, expires time.Duration) (string, error) {
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func (s *S3) Stat(ctx context.Context, key string) (int64, string, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var response *smithyhttp.ResponseError
		if errors.As(err, &response) && response.HTTPStatusCode() == http.StatusNotFound {
			return 0, "", ErrMissing
		}
		return 0, "", fmt.Errorf("head object: %w", err)
	}
	return aws.ToInt64(out.ContentLength), aws.ToString(out.ContentType), nil
}
