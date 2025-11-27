package models

import "gorm.io/gorm"

// Tag represents a game tag (e.g., "RPG", "Shooter", "Co-op").
type Tag struct {
	gorm.Model
	Name string `gorm:"size:100;unique;not null"`
}
