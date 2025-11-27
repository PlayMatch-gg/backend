package models

import "gorm.io/gorm"

// Game represents a game in the system.
type Game struct {
	gorm.Model
	Name        string `gorm:"size:255;not null"`
	Description string
	SteamURL    string `gorm:"size:512;unique"`
	Tags        []*Tag `gorm:"many2many:game_tags;"`
}
