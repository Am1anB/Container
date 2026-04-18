package db

import (
	"log"
	"time"
	"haulagex/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"golang.org/x/crypto/bcrypt"
)

var DB *gorm.DB

func Connect(dsn string) *gorm.DB {
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("db connect error: %v", err)
	}
	sqlDB, _ := DB.DB()
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(60 * time.Minute)
	return DB
}

func Migrate() error {
	return DB.AutoMigrate(&models.User{}, &models.Job{})
}

func SeedAdmin() error {
	var count int64
	if err := DB.Model(&models.User{}).Where("email = ?", "admin@hx.local").Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	admin := models.User{
		Name:         "System Admin",
		Phone:        "0999999999",
		Email:        "admin@hx.local",
		PasswordHash: string(hash),
		Role:         models.RoleAdmin,
	}
	return DB.Create(&admin).Error
}
