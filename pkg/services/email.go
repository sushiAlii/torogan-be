package services

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	utils "github.com/sushiAlii/torogan-be/pkg"
)

// EmailService sends transactional email via AWS SES. When SES_FROM_ADDRESS
// is not configured (e.g. local dev without AWS set up), it degrades to
// logging the message to stdout instead of failing outright — mirroring how
// UploadService is skipped (not fatal) when AWS_REGION/S3_BUCKET are absent
// (see cmd/server/main.go).
type EmailService struct {
	client *sesv2.Client
	from   string
}

// NewEmailService loads AWS config from SES-specific credentials
// (SES_AWS_ACCESS_KEY_ID/SES_AWS_SECRET_ACCESS_KEY), deliberately separate
// from the AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY pair UploadService uses
// for S3 — a single set of env vars can't hold two different credentials at
// once, and least-privilege means the SES-sending identity shouldn't also
// carry S3 permissions (or vice versa). If SES_FROM_ADDRESS is unset, it
// returns a service that logs instead of sending — this is intentional, not
// an error, so local dev works without AWS credentials.
func NewEmailService(ctx context.Context) (*EmailService, error) {
	from := utils.GetEnv("SES_FROM_ADDRESS", "")
	if from == "" {
		log.Println("⚠️  SES_FROM_ADDRESS not set — verification emails will be logged to stdout instead of sent")
		return &EmailService{}, nil
	}

	region := utils.GetEnv("AWS_REGION", "")
	if region == "" {
		return nil, fmt.Errorf("AWS_REGION must be set when SES_FROM_ADDRESS is configured")
	}

	accessKeyID := utils.GetEnv("SES_AWS_ACCESS_KEY_ID", "")
	secretAccessKey := utils.GetEnv("SES_AWS_SECRET_ACCESS_KEY", "")
	if accessKeyID == "" || secretAccessKey == "" {
		return nil, fmt.Errorf("SES_AWS_ACCESS_KEY_ID and SES_AWS_SECRET_ACCESS_KEY must be set when SES_FROM_ADDRESS is configured")
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &EmailService{
		client: sesv2.NewFromConfig(cfg),
		from:   from,
	}, nil
}

// SendVerificationEmail sends (or, in dev-log mode, logs) the email
// verification link to the given address.
func (e *EmailService) SendVerificationEmail(ctx context.Context, to, link string) error {
	subject := "Verify your Torogan email"
	body := fmt.Sprintf("Welcome to Torogan!\n\nVerify your email address by clicking the link below:\n\n%s\n\nThis link expires in 24 hours. If you didn't create a Torogan account, you can ignore this email.", link)

	if e.client == nil {
		log.Printf("📧 [dev] verification email for %s: %s", to, link)
		return nil
	}

	_, err := e.client.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(e.from),
		Destination:      &types.Destination{ToAddresses: []string{to}},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{Data: aws.String(subject)},
				Body: &types.Body{
					Text: &types.Content{Data: aws.String(body)},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send verification email: %w", err)
	}

	return nil
}
