package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"quotepadpro/internal/config"
	"quotepadpro/internal/models"
	"quotepadpro/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuthHandler struct {
	DB  *gorm.DB
	Cfg *config.Config
}

func NewAuthHandler(db *gorm.DB, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		DB:  db,
		Cfg: cfg,
	}
}

type SignupRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type ResendConfirmationRequest struct {
	Email string `json:"email"`
}

type UpdateMeRequest struct {
	Name         string `json:"name"`
	BusinessName string `json:"businessName"`
	Phone        string `json:"phone"`
}

func (h *AuthHandler) Signup(c *gin.Context) {
	var req SignupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Name = strings.TrimSpace(req.Name)

	if req.Email == "" || req.Password == "" || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, email and password are required"})
		return
	}

	var existing models.User
	if err := h.DB.Where("email = ?", req.Email).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "email already in use"})
		return
	}

	hash, err := services.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	verifyToken, _ := services.GenerateRandomToken()
	expires := time.Now().Add(24 * time.Hour)

	user := models.User{
		Name:                 req.Name,
		Email:                req.Email,
		PasswordHash:         hash,
		EmailVerified:        false,
		EmailVerifyToken:     verifyToken,
		EmailVerifyExpiresAt: &expires,
	}

	if err := h.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	verifyURL := fmt.Sprintf("%s/confirm-email?token=%s",
		strings.TrimRight(h.Cfg.FrontendURL, "/"),
		verifyToken,
	)

	businessName := user.BusinessName
	if businessName == "" {
		businessName = "QuotePadPro"
	}

	htmlBody := services.BuildBrandedEmail(services.EmailTemplateData{
		LogoURL:        user.LogoURL,
		DefaultLogoURL: h.Cfg.DefaultEmailLogoURL,
		BusinessName:   businessName,
		Heading:        "Confirm your email",
		Intro:          fmt.Sprintf("Hello %s, please confirm your email.", user.Name),
		ButtonText:     "Confirm Email",
		ButtonURL:      verifyURL,
		BodyHTML:       "If you did not create this account, you can ignore this email.",
	})

	if err := services.SendEmail(h.emailCfg(), []string{user.Email}, "Confirm your email", htmlBody); err != nil {
		fmt.Println("confirm email error:", err)
	}

	token, _ := services.GenerateJWT(user.ID, user.Email, h.Cfg.JWTSecret)

	c.JSON(http.StatusCreated, gin.H{
		"user":  h.userResponse(user),
		"token": token,
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))

	var user models.User
	if err := h.DB.Where("email = ?", email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if !services.CheckPassword(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, _ := services.GenerateJWT(user.ID, user.Email, h.Cfg.JWTSecret)

	c.JSON(http.StatusOK, gin.H{
		"user":  h.userResponse(user),
		"token": token,
	})
}

func (h *AuthHandler) Me(c *gin.Context) {
	user := h.getUser(c)
	if user == nil {
		return
	}
	c.JSON(http.StatusOK, h.userResponse(*user))
}

func (h *AuthHandler) UpdateMe(c *gin.Context) {
	user := h.getUser(c)
	if user == nil {
		return
	}

	var req UpdateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	user.Name = req.Name
	user.BusinessName = req.BusinessName
	user.Phone = req.Phone

	h.DB.Save(user)

	c.JSON(http.StatusOK, h.userResponse(*user))
}

func (h *AuthHandler) UploadLogo(c *gin.Context) {
	user := h.getUser(c)
	if user == nil {
		return
	}

	file, header, err := c.Request.FormFile("logo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "logo file is required"})
		return
	}

	logoURL, err := services.UploadFileToS3(services.S3UploadConfig{
		Region:     h.Cfg.AWSRegion,
		AccessKey:  h.Cfg.AWSAccessKey,
		SecretKey:  h.Cfg.AWSSecretKey,
		Bucket:     h.Cfg.S3Bucket,
		LogoPrefix: h.Cfg.S3LogoPrefix,
		PublicBase: h.Cfg.S3PublicBase,
	}, file, header)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upload logo"})
		return
	}

	user.LogoURL = logoURL
	user.LogoFilename = header.Filename

	if err := h.DB.Save(user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save logo"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logoUrl":      user.LogoURL,
		"logoFilename": user.LogoFilename,
	})
}

func (h *AuthHandler) ConfirmEmail(c *gin.Context) {
	token := c.Query("token")

	var user models.User
	if err := h.DB.Where("email_verify_token = ?", token).First(&user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token"})
		return
	}

	if user.EmailVerifyExpiresAt == nil || user.EmailVerifyExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token expired"})
		return
	}

	user.EmailVerified = true
	user.EmailVerifyToken = ""
	user.EmailVerifyExpiresAt = nil

	h.DB.Save(&user)

	c.JSON(http.StatusOK, gin.H{"verified": true})
}

func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	c.ShouldBindJSON(&req)

	email := strings.ToLower(strings.TrimSpace(req.Email))

	var user models.User
	if err := h.DB.Where("email = ?", email).First(&user).Error; err == nil {

		token, _ := services.GenerateRandomToken()
		expires := time.Now().Add(1 * time.Hour)

		user.ResetPasswordToken = token
		user.ResetPasswordExpiresAt = &expires
		h.DB.Save(&user)

		resetURL := fmt.Sprintf("%s/reset-password?token=%s",
			strings.TrimRight(h.Cfg.FrontendURL, "/"),
			token,
		)

		htmlBody := services.BuildBrandedEmail(services.EmailTemplateData{
			LogoURL:        "",
			DefaultLogoURL: h.Cfg.DefaultEmailLogoURL,
			BusinessName:   "QuotePadPro",
			Heading:        "Reset your password",
			Intro:          fmt.Sprintf("Hello %s,", user.Name),
			ButtonText:     "Reset Password",
			ButtonURL:      resetURL,
			BodyHTML:       "If you did not request this, ignore this email.",
		})

		if err := services.SendEmail(h.emailCfg(), []string{user.Email}, "Reset your password", htmlBody); err != nil {
			fmt.Println("reset email error:", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "If account exists, email sent."})
}

func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	c.ShouldBindJSON(&req)

	var user models.User
	if err := h.DB.Where("reset_password_token = ?", req.Token).First(&user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token"})
		return
	}

	if user.ResetPasswordExpiresAt == nil || user.ResetPasswordExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token expired"})
		return
	}

	hash, _ := services.HashPassword(req.Password)

	user.PasswordHash = hash
	user.ResetPasswordToken = ""
	user.ResetPasswordExpiresAt = nil

	h.DB.Save(&user)

	c.JSON(http.StatusOK, gin.H{"reset": true})
}

func (h *AuthHandler) ResendConfirmation(c *gin.Context) {
	var req ResendConfirmationRequest
	c.ShouldBindJSON(&req)

	email := strings.ToLower(strings.TrimSpace(req.Email))

	var user models.User
	if err := h.DB.Where("email = ?", email).First(&user).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "If account exists, email sent."})
		return
	}

	if user.EmailVerified {
		c.JSON(http.StatusOK, gin.H{"message": "Already verified."})
		return
	}

	token, _ := services.GenerateRandomToken()
	expires := time.Now().Add(24 * time.Hour)

	user.EmailVerifyToken = token
	user.EmailVerifyExpiresAt = &expires
	h.DB.Save(&user)

	verifyURL := fmt.Sprintf("%s/confirm-email?token=%s",
		strings.TrimRight(h.Cfg.FrontendURL, "/"),
		token,
	)

	htmlBody := services.BuildBrandedEmail(services.EmailTemplateData{
		LogoURL:        "",
		DefaultLogoURL: h.Cfg.DefaultEmailLogoURL,
		BusinessName:   "QuotePadPro",
		Heading:        "Confirm your email",
		Intro:          "Here is a new confirmation link.",
		ButtonText:     "Confirm Email",
		ButtonURL:      verifyURL,
	})

	services.SendEmail(h.emailCfg(), []string{user.Email}, "Confirm your email", htmlBody)

	c.JSON(http.StatusOK, gin.H{"message": "Confirmation email sent."})
}

/* ---------------- helpers ---------------- */

func (h *AuthHandler) getUser(c *gin.Context) *models.User {
	userIDVal, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil
	}

	var user models.User
	if err := h.DB.First(&user, userIDVal.(uint)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return nil
	}

	return &user
}

func (h *AuthHandler) userResponse(u models.User) gin.H {
	return gin.H{
		"id":            u.ID,
		"name":          u.Name,
		"email":         u.Email,
		"emailVerified": u.EmailVerified,
		"businessName":  u.BusinessName,
		"phone":         u.Phone,
		"logoUrl":       u.LogoURL,
		"logoFilename":  u.LogoFilename,
	}
}

func (h *AuthHandler) emailCfg() services.EmailConfig {
	return services.EmailConfig{
		Region:    h.Cfg.AWSRegion,
		AccessKey: h.Cfg.AWSAccessKey,
		SecretKey: h.Cfg.AWSSecretKey,
		From:      h.Cfg.EmailFrom,
		FromName:  h.Cfg.EmailFromName,
	}
}
