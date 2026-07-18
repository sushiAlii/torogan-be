package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"

	utils "github.com/sushiAlii/torogan-be/pkg"
)

const presignedUploadExpiry = 5 * time.Minute

// UploadService mints short-lived presigned S3 PUT URLs so the browser can
// upload files directly to the bucket without routing bytes through this
// server. Objects are stored under the "properties/" prefix, which the
// bucket policy (configured out-of-band in AWS) makes public-read.
type UploadService struct {
	presignClient *s3.PresignClient
	bucket        string
	region        string
}

func NewUploadService(ctx context.Context) (*UploadService, error) {
	region := utils.GetEnv("AWS_REGION", "")
	bucket := utils.GetEnv("S3_BUCKET", "")
	if region == "" || bucket == "" {
		return nil, errors.New("AWS_REGION and S3_BUCKET environment variables must be set")
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg)

	return &UploadService{
		presignClient: s3.NewPresignClient(client),
		bucket:        bucket,
		region:        region,
	}, nil
}

// CreatePresignedUpload mints a presigned PUT URL for a new object under
// "properties/", plus the public URL the object will be reachable at once
// uploaded.
func (s *UploadService) CreatePresignedUpload(ctx context.Context, contentType, fileExt string) (uploadURL, publicURL, key string, err error) {
	key = fmt.Sprintf("properties/%s", uuid.NewString())
	if fileExt != "" {
		key = fmt.Sprintf("%s.%s", key, fileExt)
	}

	req, err := s.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(presignedUploadExpiry))
	if err != nil {
		return "", "", "", fmt.Errorf("failed to presign upload: %w", err)
	}

	publicURL = fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, key)

	return req.URL, publicURL, key, nil
}
