package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                string
	DatabaseURL         string
	JWTSecret           string
	AppBaseURL          string
	FrontendURL         string
	CORSOrigins         string
	EmailProvider       string
	EmailFrom           string
	AWSRegion           string
	AWSAccessKey        string
	AWSSecretKey        string
	S3Bucket            string
	S3LogoPrefix        string
	S3PublicBase        string
	SMTPHost            string
	SMTPPort            string
	SMTPUsername        string
	SMTPPassword        string
	DefaultEmailLogoURL string
	EmailFromName       string
}

func Load() *Config {
	_ = godotenv.Load(".env.local")

	cfg := &Config{
		Port:                getEnv("PORT", "8080"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		JWTSecret:           os.Getenv("JWT_SECRET"),
		AppBaseURL:          os.Getenv("APP_BASE_URL"),
		FrontendURL:         os.Getenv("FRONTEND_BASE_URL"),
		CORSOrigins:         os.Getenv("CORS_ORIGINS"),
		EmailProvider:       getEnv("EMAIL_PROVIDER", "ses"),
		EmailFrom:           os.Getenv("EMAIL_FROM"),
		AWSRegion:           os.Getenv("AWS_REGION"),
		AWSAccessKey:        os.Getenv("AWS_ACCESS_KEY_ID"),
		AWSSecretKey:        os.Getenv("AWS_SECRET_ACCESS_KEY"),
		S3Bucket:            os.Getenv("S3_BUCKET"),
		S3LogoPrefix:        getEnv("S3_LOGO_PREFIX", "logos"),
		S3PublicBase:        os.Getenv("S3_PUBLIC_BASE_URL"),
		SMTPHost:            os.Getenv("SMTP_HOST"),
		SMTPPort:            os.Getenv("SMTP_PORT"),
		SMTPUsername:        os.Getenv("SMTP_USERNAME"),
		SMTPPassword:        os.Getenv("SMTP_PASSWORD"),
		DefaultEmailLogoURL: os.Getenv("DEFAULT_EMAIL_LOGO_URL"),
		EmailFromName:       os.Getenv("EMAIL_FROM_NAME"),
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	return cfg
}

func getEnv(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}
