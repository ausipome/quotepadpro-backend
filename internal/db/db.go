package db

import (
	"log"

	"quotepadpro/internal/config"
	"quotepadpro/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(cfg *config.Config) *gorm.DB {
	database, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect to database:", err)
	}

	err = database.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Quote{},
		&models.QuoteItem{},
	)
	if err != nil {
		log.Fatal("failed to migrate database:", err)
	}

	log.Println("database connected and migrated")
	return database
}