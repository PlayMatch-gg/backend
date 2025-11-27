package models

import "gorm.io/gorm"

// Lobby represents a game lobby where users can gather.
type Lobby struct {
	gorm.Model
	GameID      uint   `gorm:"not null"`
	HostID      uint   `gorm:"not null"`
	Title       string `gorm:"size:255;not null"`
	Description string
	MaxPlayers  int `gorm:"not null;default:5"`

	Game    Game   `gorm:"foreignKey:GameID"`
	Host    User   `gorm:"foreignKey:HostID"`
	Members []User `gorm:"foreignKey:CurrentLobbyID"` // Has Many relationship
}
