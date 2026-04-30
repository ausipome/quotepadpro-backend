package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

type EmailConfig struct {
	Region    string
	AccessKey string
	SecretKey string
	From      string
	FromName  string
}

func SendEmail(cfg EmailConfig, to []string, subject string, htmlBody string) error {
	if cfg.Region == "" || cfg.AccessKey == "" || cfg.SecretKey == "" || cfg.From == "" {
		return fmt.Errorf("email config incomplete")
	}

	recipients := make([]string, 0, len(to))
	for _, addr := range to {
		addr = strings.TrimSpace(addr)
		if addr != "" {
			recipients = append(recipients, addr)
		}
	}

	if len(recipients) == 0 {
		return fmt.Errorf("no email recipients supplied")
	}

	from := strings.TrimSpace(cfg.From)
	fromName := strings.TrimSpace(cfg.FromName)

	if fromName != "" {
		from = fmt.Sprintf("%s <%s>", fromName, from)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	awsCfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		),
	)
	if err != nil {
		return err
	}

	client := sesv2.NewFromConfig(awsCfg)

	_, err = client.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(from),
		Destination: &types.Destination{
			ToAddresses: recipients,
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{
					Data:    aws.String(subject),
					Charset: aws.String("UTF-8"),
				},
				Body: &types.Body{
					Html: &types.Content{
						Data:    aws.String(htmlBody),
						Charset: aws.String("UTF-8"),
					},
				},
			},
		},
	})

	return err
}
