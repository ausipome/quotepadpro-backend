package handlers

import (
	"fmt"
	"strings"
	"time"

	"quotepadpro/internal/models"
	"quotepadpro/internal/services"
)

func isQuoteExpired(quote models.Quote) bool {
	if quote.ExpiryDate == nil {
		return false
	}

	expiry := time.Date(quote.ExpiryDate.Year(), quote.ExpiryDate.Month(), quote.ExpiryDate.Day(), 23, 59, 59, 0, quote.ExpiryDate.Location())
	return time.Now().After(expiry)
}

func statusAfterAcceptedEdit(reqStatus string) string {
	normalized := normalizeQuoteStatus(reqStatus)
	if normalized == "accepted" {
		return "sent"
	}
	return normalized
}

func (h *QuoteHandler) sendQuoteAcceptedEmail(quote models.Quote, owner models.User) error {
	if strings.TrimSpace(owner.Email) == "" {
		return nil
	}

	businessName := owner.BusinessName
	if businessName == "" {
		businessName = "QuotePadPro"
	}

	customerName := "Customer"
	if quote.Contact != nil && strings.TrimSpace(quote.Contact.Name) != "" {
		customerName = strings.TrimSpace(quote.Contact.Name)
	}

	frontendURL := strings.TrimRight(h.Cfg.FrontendURL, "/")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	quoteURL := fmt.Sprintf("%s/q/%s", frontendURL, quote.PublicID)
	acceptedAt := "Just now"
	if quote.AcceptedAt != nil {
		acceptedAt = quote.AcceptedAt.Format("02 Jan 2006 15:04")
	}

	htmlBody := services.BuildBrandedEmail(services.EmailTemplateData{
		LogoURL:        owner.LogoURL,
		DefaultLogoURL: h.Cfg.DefaultEmailLogoURL,
		BusinessName:   businessName,
		Heading:        "Quote accepted",
		Intro:          fmt.Sprintf("Good news — %s has accepted a quote.", customerName),
		ButtonText:     "View Quote",
		ButtonURL:      quoteURL,
		BodyHTML: fmt.Sprintf(`
			<p style="margin:0 0 10px 0;"><strong>Customer:</strong> %s</p>
			<p style="margin:0 0 10px 0;"><strong>Quote Ref:</strong> %s</p>
			<p style="margin:0 0 10px 0;"><strong>Quote Title:</strong> %s</p>
			<p style="margin:0;"><strong>Accepted:</strong> %s</p>
		`, customerName, quote.QuoteNumber, quote.Title, acceptedAt),
	})

	subject := fmt.Sprintf("Quote accepted: %s", quote.QuoteNumber)
	return services.SendEmail(services.EmailConfig{
		Region:    h.Cfg.AWSRegion,
		AccessKey: h.Cfg.AWSSESAccessKey,
		SecretKey: h.Cfg.AWSSESSecretKey,
		From:      h.Cfg.EmailFrom,
		FromName:  h.Cfg.EmailFromName,
	}, []string{owner.Email}, subject, htmlBody)
}
