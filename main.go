package main

import (
	"log"
	"strings"
	"time"

	"quotepadpro/internal/config"
	"quotepadpro/internal/db"
	"quotepadpro/internal/handlers"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	database := db.Connect(cfg)

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     strings.Split(cfg.CORSOrigins, ","),
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	authHandler := handlers.NewAuthHandler(database, cfg)
	contactHandler := handlers.NewContactHandler(database)
	quoteHandler := handlers.NewQuoteHandler(database, cfg)
	billingHandler := handlers.NewBillingHandler(database, cfg)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	r.GET("/health/db", func(c *gin.Context) {
		sqlDB, err := database.DB()
		if err != nil {
			c.JSON(500, gin.H{"status": "error", "error": err.Error()})
			return
		}

		if err := sqlDB.Ping(); err != nil {
			c.JSON(500, gin.H{"status": "error", "error": err.Error()})
			return
		}

		c.JSON(200, gin.H{
			"status": "ok",
			"db":     "connected",
		})
	})

	r.POST("/auth/signup", authHandler.Signup)
	r.POST("/auth/login", authHandler.Login)
	r.GET("/auth/confirm-email", authHandler.ConfirmEmail)
	r.POST("/auth/forgot-password", authHandler.ForgotPassword)
	r.POST("/auth/reset-password", authHandler.ResetPassword)
	r.POST("/auth/resend-confirmation", authHandler.ResendConfirmation)
	r.POST("/billing/webhook", billingHandler.StripeWebhook)

	protected := r.Group("/")
	protected.Use(handlers.AuthMiddleware(cfg))

	protected.GET("/me", authHandler.Me)
	protected.PUT("/me", authHandler.UpdateMe)
	protected.POST("/me/logo", authHandler.UploadLogo)

	protected.POST("/billing/create-checkout-session", billingHandler.CreateCheckoutSession)
	protected.POST("/billing/cancel-subscription", billingHandler.CancelSubscription)

	protected.POST("/contacts", contactHandler.Create)
	protected.GET("/contacts", contactHandler.List)
	protected.GET("/contacts/:id", contactHandler.Get)
	protected.PUT("/contacts/:id", contactHandler.Update)
	protected.DELETE("/contacts/:id", contactHandler.Delete)

	protected.POST("/quotes", quoteHandler.Create)
	protected.GET("/quotes", quoteHandler.List)
	protected.GET("/quotes/:id", quoteHandler.Get)
	protected.PUT("/quotes/:id", quoteHandler.Update)
	protected.DELETE("/quotes/:id", quoteHandler.Delete)
	protected.POST("/quotes/:id/send", quoteHandler.SendQuote)

	r.GET("/public/quotes/:publicId", quoteHandler.PublicGet)
	r.POST("/public/quotes/:publicId/accept", quoteHandler.PublicAccept)

	log.Println("server running on port", cfg.Port)
	r.Run(":" + cfg.Port)
}
