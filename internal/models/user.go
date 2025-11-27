package models

import "gorm.io/gorm"

// User represents a user in the system.
type User struct {
	gorm.Model
	Nickname     string `gorm:"size:255;unique;not null"`
	Email        string `gorm:"size:255;unique;not null"`
	PasswordHash string `gorm:"size:255;not null"`
}
