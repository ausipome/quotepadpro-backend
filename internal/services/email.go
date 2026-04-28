package services

import (
	"bytes"
	"fmt"
	"net/smtp"
	"strings"
)

type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
	FromName string
}

func SendSMTPEmail(cfg SMTPConfig, to []string, subject string, htmlBody string) error {
	if cfg.Host == "" || cfg.Port == "" || cfg.From == "" {
		return fmt.Errorf("smtp config incomplete")
	}

	fromName := strings.TrimSpace(cfg.FromName)
	if fromName == "" {
		fromName = "QuotePad"
	}

	var msg bytes.Buffer
	msg.WriteString(fmt.Sprintf("From: %s <%s>\r\n", fromName, cfg.From))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ",")))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(htmlBody)

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)

	var auth smtp.Auth
	if cfg.Username != "" && cfg.Password != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}

	return smtp.SendMail(addr, auth, cfg.From, to, msg.Bytes())
}
