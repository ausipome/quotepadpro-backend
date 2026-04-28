package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"quotepadpro/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ContactHandler struct {
	DB *gorm.DB
}

type ContactRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Company  string `json:"company"`
	Address1 string `json:"address1"`
	Address2 string `json:"address2"`
	City     string `json:"city"`
	County   string `json:"county"`
	Postcode string `json:"postcode"`
}

func NewContactHandler(db *gorm.DB) *ContactHandler {
	return &ContactHandler{DB: db}
}

func (h *ContactHandler) Create(c *gin.Context) {
	userID := c.MustGet("userId").(uint)

	var req ContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	contact := models.Contact{
		UserID:   userID,
		Name:     req.Name,
		Email:    req.Email,
		Phone:    strings.TrimSpace(req.Phone),
		Company:  strings.TrimSpace(req.Company),
		Address1: strings.TrimSpace(req.Address1),
		Address2: strings.TrimSpace(req.Address2),
		City:     strings.TrimSpace(req.City),
		County:   strings.TrimSpace(req.County),
		Postcode: strings.TrimSpace(req.Postcode),
	}

	if err := h.DB.Create(&contact).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create contact"})
		return
	}

	c.JSON(http.StatusCreated, contact)
}

func (h *ContactHandler) List(c *gin.Context) {
	userID := c.MustGet("userId").(uint)

	var contacts []models.Contact
	if err := h.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&contacts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch contacts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"contacts": contacts})
}

func (h *ContactHandler) Get(c *gin.Context) {
	userID := c.MustGet("userId").(uint)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid contact id"})
		return
	}

	var contact models.Contact
	if err := h.DB.Where("id = ? AND user_id = ?", id, userID).First(&contact).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contact not found"})
		return
	}

	c.JSON(http.StatusOK, contact)
}

func (h *ContactHandler) Update(c *gin.Context) {
	userID := c.MustGet("userId").(uint)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid contact id"})
		return
	}

	var contact models.Contact
	if err := h.DB.Where("id = ? AND user_id = ?", id, userID).First(&contact).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contact not found"})
		return
	}

	var req ContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	contact.Name = req.Name
	contact.Email = req.Email
	contact.Phone = strings.TrimSpace(req.Phone)
	contact.Company = strings.TrimSpace(req.Company)
	contact.Address1 = strings.TrimSpace(req.Address1)
	contact.Address2 = strings.TrimSpace(req.Address2)
	contact.City = strings.TrimSpace(req.City)
	contact.County = strings.TrimSpace(req.County)
	contact.Postcode = strings.TrimSpace(req.Postcode)

	if err := h.DB.Save(&contact).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update contact"})
		return
	}

	c.JSON(http.StatusOK, contact)
}

func (h *ContactHandler) Delete(c *gin.Context) {
	userID := c.MustGet("userId").(uint)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid contact id"})
		return
	}

	var contact models.Contact
	if err := h.DB.Where("id = ? AND user_id = ?", id, userID).First(&contact).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contact not found"})
		return
	}

	var quoteCount int64
	if err := h.DB.Model(&models.Quote{}).
		Where("contact_id = ? AND user_id = ?", contact.ID, userID).
		Count(&quoteCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check linked quotes"})
		return
	}

	if quoteCount > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error": "This contact is linked to one or more quotes and cannot be deleted.",
		})
		return
	}

	if err := h.DB.Delete(&contact).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete contact"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
