package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3UploadConfig struct {
	Region      string
	AccessKey   string
	SecretKey   string
	Bucket      string
	LogoPrefix  string
	PublicBase  string
}

func UploadFileToS3(cfg S3UploadConfig, file multipart.File, header *multipart.FileHeader) (string, error) {
	if cfg.Region == "" || cfg.AccessKey == "" || cfg.SecretKey == "" || cfg.Bucket == "" {
		return "", fmt.Errorf("s3 config incomplete")
	}

	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == "" {
		ext = ".png"
	}

	key := fmt.Sprintf("%s/%d%s",
		strings.Trim(strings.TrimSpace(cfg.LogoPrefix), "/"),
		time.Now().UnixNano(),
		ext,
	)

	awsCfg, err := awsconfig.LoadDefaultConfig(
		context.TODO(),
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		),
	)
	if err != nil {
		return "", err
	}

	client := s3.NewFromConfig(awsCfg)

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(cfg.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", err
	}

	if cfg.PublicBase != "" {
		return fmt.Sprintf("%s/%s", strings.TrimRight(cfg.PublicBase, "/"), key), nil
	}

	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.Bucket, cfg.Region, key), nil
}