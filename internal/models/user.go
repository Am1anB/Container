package models

import "time"

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

type User struct {
	ID              uint `gorm:"primaryKey"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Name            string
	Phone           string `gorm:"uniqueIndex"`
	Email           string `gorm:"uniqueIndex"`
	PasswordHash    string
	Role            Role    `gorm:"type:varchar(20);default:'user'"`
	LicensePlate    string  `gorm:"index" json:"licensePlate"`
	ProfileImageURL *string `json:"profileImageURL" gorm:"default:null"`
}
