package models

import "gorm.io/gorm"

// User represents a user in the system.
type User struct {
	gorm.Model
	Nickname      string  `gorm:"size:255;unique;not null"`
	Email         string  `gorm:"size:255;unique;not null"`
	PasswordHash  string  `gorm:"size:255;not null"`
	Role          string  `gorm:"size:50;not null;default:'user';index"`
	FavoriteGames []*Game `gorm:"many2many:user_favorite_games;"`

	// A user can only be in one lobby at a time.
	CurrentLobbyID *uint  `gorm:"index"`
	CurrentLobby   *Lobby `gorm:"foreignKey:CurrentLobbyID"`
}
