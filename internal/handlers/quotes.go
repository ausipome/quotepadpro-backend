package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"quotepadpro/internal/config"
	"quotepadpro/internal/models"
	"quotepadpro/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type QuoteHandler struct {
	DB  *gorm.DB
	Cfg *config.Config
}

type QuoteItemRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unitPrice"`
	LineTotal   float64 `json:"lineTotal"`
	SortOrder   int     `json:"sortOrder"`
}

type QuoteRequest struct {
	ContactID     *uint              `json:"contactId"`
	QuoteNumber   string             `json:"quoteNumber"`
	Title         string             `json:"title"`
	Status        string             `json:"status"`
	QuoteDate     *string            `json:"quoteDate"`
	ExpiryDate    *string            `json:"expiryDate"`
	Notes         string             `json:"notes"`
	Subtotal      float64            `json:"subtotal"`
	DiscountType  string             `json:"discountType"`
	DiscountValue float64            `json:"discountValue"`
	VATMode       string             `json:"vatMode"`
	VATRate       float64            `json:"vatRate"`
	VATAmount     float64            `json:"vatAmount"`
	Total         float64            `json:"total"`
	Items         []QuoteItemRequest `json:"items"`
}

func NewQuoteHandler(db *gorm.DB, cfg *config.Config) *QuoteHandler {
	return &QuoteHandler{DB: db, Cfg: cfg}
}

func (h *QuoteHandler) SendQuote(c *gin.Context) {
	userID := c.MustGet("userId").(uint)
	if !ensureSubscribed(h.DB, userID, c) {
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quote id"})
		return
	}

	var quote models.Quote
	if err := h.DB.
		Where("id = ? AND user_id = ?", id, userID).
		Preload("Contact").
		First(&quote).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "quote not found"})
		return
	}

	if quote.Contact == nil || strings.TrimSpace(quote.Contact.Email) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "quote contact has no email address"})
		return
	}

	if isQuoteExpired(quote) && quote.Status != "accepted" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This quote has expired. Update the expiry date before sending it."})
		return
	}

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load user"})
		return
	}

	publicBase := h.Cfg.FrontendURL
	if publicBase == "" {
		publicBase = "http://localhost:3000"
	}
	publicURL := fmt.Sprintf("%s/q/%s", strings.TrimRight(publicBase, "/"), quote.PublicID)

	subject := fmt.Sprintf("%s - %s", quote.QuoteNumber, quote.Title)
	if strings.TrimSpace(subject) == "-" || strings.TrimSpace(subject) == "" {
		subject = fmt.Sprintf("Your Quote %s", quote.QuoteNumber)
	}

	businessName := user.BusinessName
	if businessName == "" {
		businessName = user.Name
	}

	htmlBody := services.BuildBrandedEmail(services.EmailTemplateData{
		LogoURL:        user.LogoURL,
		DefaultLogoURL: h.Cfg.DefaultEmailLogoURL,
		BusinessName:   businessName,
		Heading:        quote.Title,
		Intro:          fmt.Sprintf("Hello %s, your quote is ready to view online.", quote.Contact.Name),
		ButtonText:     "View Quote",
		ButtonURL:      publicURL,
		BodyHTML: fmt.Sprintf(`
			<p style="margin:0 0 10px 0;"><strong>Quote Number:</strong> %s</p>
			<p style="margin:0;">You can open the quote, review the details, and accept it online.</p>
		`, quote.QuoteNumber),
	})

	err = services.SendEmail(services.EmailConfig{
		Region:    h.Cfg.AWSRegion,
		AccessKey: h.Cfg.AWSSESAccessKey,
		SecretKey: h.Cfg.AWSSESSecretKey,
		From:      h.Cfg.EmailFrom,
		FromName:  h.Cfg.EmailFromName,
	}, []string{quote.Contact.Email}, subject, htmlBody)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send quote email"})
		return
	}

	if quote.Status == "draft" {
		quote.Status = "sent"
		_ = h.DB.Save(&quote).Error
	}

	c.JSON(http.StatusOK, gin.H{
		"sent":   true,
		"to":     quote.Contact.Email,
		"status": quote.Status,
	})
}

func ensureSubscribed(db *gorm.DB, userID uint, c *gin.Context) bool {
	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return false
	}

	status := strings.ToLower(strings.TrimSpace(user.SubscriptionStatus))
	if status == "trialing" || status == "active" {
		return true
	}

	c.JSON(http.StatusPaymentRequired, gin.H{"error": "An active subscription or trial is required to use this feature."})
	return false
}

func nextQuoteNumber(db *gorm.DB, userID uint) (string, error) {
	var count int64
	if err := db.Model(&models.Quote{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return "", err
	}

	return fmt.Sprintf("Q-%04d", count+1), nil
}

func todayDatePtr() *time.Time {
	now := time.Now()
	d := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return &d
}

func parseDatePtr(input *string) (*time.Time, error) {
	if input == nil || strings.TrimSpace(*input) == "" {
		return nil, nil
	}

	t, err := time.Parse("2006-01-02", strings.TrimSpace(*input))
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func randomPublicID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func normalizeQuoteStatus(status string) string {
	s := strings.TrimSpace(strings.ToLower(status))
	switch s {
	case "draft", "sent", "accepted":
		return s
	default:
		return "draft"
	}
}

func buildQuoteItems(items []QuoteItemRequest) []models.QuoteItem {
	out := make([]models.QuoteItem, 0, len(items))
	for _, item := range items {
		out = append(out, models.QuoteItem{
			Name:        strings.TrimSpace(item.Name),
			Description: strings.TrimSpace(item.Description),
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			LineTotal:   item.LineTotal,
			SortOrder:   item.SortOrder,
		})
	}
	return out
}

func (h *QuoteHandler) Create(c *gin.Context) {
	userID := c.MustGet("userId").(uint)
	if !ensureSubscribed(h.DB, userID, c) {
		return
	}

	var req QuoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	quoteDate := todayDatePtr()

	expiryDate, err := parseDatePtr(req.ExpiryDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid expiryDate, use YYYY-MM-DD"})
		return
	}

	if req.Title == "" {
		req.Title = "Quote"
	}

	quote := models.Quote{
		UserID:        userID,
		ContactID:     req.ContactID,
		Title:         strings.TrimSpace(req.Title),
		Status:        normalizeQuoteStatus(req.Status),
		QuoteDate:     quoteDate,
		ExpiryDate:    expiryDate,
		Notes:         strings.TrimSpace(req.Notes),
		Subtotal:      req.Subtotal,
		DiscountType:  strings.TrimSpace(req.DiscountType),
		DiscountValue: req.DiscountValue,
		VATMode:       strings.TrimSpace(req.VATMode),
		VATRate:       req.VATRate,
		VATAmount:     req.VATAmount,
		Total:         req.Total,
		PublicID:      randomPublicID(),
		Items:         buildQuoteItems(req.Items),
	}

	if req.ContactID != nil {
		var contact models.Contact
		if err := h.DB.Where("id = ? AND user_id = ?", *req.ContactID, userID).First(&contact).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid contactId"})
			return
		}
	}

	quoteNumber, err := nextQuoteNumber(h.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate quote number"})
		return
	}
	quote.QuoteNumber = quoteNumber

	if err := h.DB.Create(&quote).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create quote"})
		return
	}

	if err := h.DB.Preload("Items").Preload("Contact").First(&quote, quote.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load quote"})
		return
	}

	c.JSON(http.StatusCreated, quote)
}

func (h *QuoteHandler) List(c *gin.Context) {
	userID := c.MustGet("userId").(uint)

	var quotes []models.Quote
	if err := h.DB.
		Where("user_id = ?", userID).
		Preload("Contact").
		Order("created_at desc").
		Find(&quotes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch quotes"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"quotes": quotes})
}

func (h *QuoteHandler) Get(c *gin.Context) {
	userID := c.MustGet("userId").(uint)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quote id"})
		return
	}

	var quote models.Quote
	if err := h.DB.
		Where("id = ? AND user_id = ?", id, userID).
		Preload("Items").
		Preload("Contact").
		First(&quote).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "quote not found"})
		return
	}

	c.JSON(http.StatusOK, quote)
}

func (h *QuoteHandler) Update(c *gin.Context) {
	userID := c.MustGet("userId").(uint)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quote id"})
		return
	}

	var quote models.Quote
	if err := h.DB.Where("id = ? AND user_id = ?", id, userID).First(&quote).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "quote not found"})
		return
	}

	wasAccepted := quote.Status == "accepted"

	var req QuoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	expiryDate, err := parseDatePtr(req.ExpiryDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid expiryDate, use YYYY-MM-DD"})
		return
	}

	if req.ContactID != nil {
		var contact models.Contact
		if err := h.DB.Where("id = ? AND user_id = ?", *req.ContactID, userID).First(&contact).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid contactId"})
			return
		}
	}

	quote.ContactID = req.ContactID
	quote.Title = strings.TrimSpace(req.Title)
	quote.Status = normalizeQuoteStatus(req.Status)
	if wasAccepted {
		quote.Status = statusAfterAcceptedEdit(req.Status)
		quote.AcceptedAt = nil
	}
	quote.ExpiryDate = expiryDate
	quote.Notes = strings.TrimSpace(req.Notes)
	quote.Subtotal = req.Subtotal
	quote.DiscountType = strings.TrimSpace(req.DiscountType)
	quote.DiscountValue = req.DiscountValue
	quote.VATMode = strings.TrimSpace(req.VATMode)
	quote.VATRate = req.VATRate
	quote.VATAmount = req.VATAmount
	quote.Total = req.Total

	err = h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&quote).Error; err != nil {
			return err
		}

		if err := tx.Where("quote_id = ?", quote.ID).Delete(&models.QuoteItem{}).Error; err != nil {
			return err
		}

		items := buildQuoteItems(req.Items)
		for i := range items {
			items[i].QuoteID = quote.ID
		}

		if len(items) > 0 {
			if err := tx.Create(&items).Error; err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update quote"})
		return
	}

	if err := h.DB.Preload("Items").Preload("Contact").First(&quote, quote.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load quote"})
		return
	}

	c.JSON(http.StatusOK, quote)
}

func (h *QuoteHandler) Delete(c *gin.Context) {
	userID := c.MustGet("userId").(uint)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quote id"})
		return
	}

	var quote models.Quote
	if err := h.DB.Where("id = ? AND user_id = ?", id, userID).First(&quote).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "quote not found"})
		return
	}

	err = h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("quote_id = ?", quote.ID).Delete(&models.QuoteItem{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&quote).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete quote"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (h *QuoteHandler) PublicGet(c *gin.Context) {
	publicID := c.Param("publicId")

	var quote models.Quote
	if err := h.DB.
		Where("public_id = ?", publicID).
		Preload("Items").
		Preload("Contact").
		First(&quote).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "quote not found"})
		return
	}

	var user models.User
	if err := h.DB.First(&user, quote.UserID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load quote owner"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"quote":     quote,
		"isExpired": isQuoteExpired(quote),
		"owner": gin.H{
			"name":            user.Name,
			"businessName":    user.BusinessName,
			"businessAddress": user.BusinessAddress,
			"phone":           user.Phone,
			"logoUrl":         user.LogoURL,
			"email":           user.Email,
		},
	})
}

func (h *QuoteHandler) PublicAccept(c *gin.Context) {
	publicID := c.Param("publicId")

	var quote models.Quote
	if err := h.DB.Preload("Contact").Where("public_id = ?", publicID).First(&quote).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "quote not found"})
		return
	}

	if quote.Status == "accepted" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This quote has already been accepted."})
		return
	}

	if isQuoteExpired(quote) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This quote has expired and can no longer be accepted."})
		return
	}

	now := time.Now()
	quote.Status = "accepted"
	quote.AcceptedAt = &now

	if err := h.DB.Save(&quote).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to accept quote"})
		return
	}

	var owner models.User
	if err := h.DB.First(&owner, quote.UserID).Error; err == nil {
		if err := h.sendQuoteAcceptedEmail(quote, owner); err != nil {
			fmt.Println("quote accepted email error:", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"accepted":   true,
		"status":     quote.Status,
		"acceptedAt": quote.AcceptedAt,
	})
}
