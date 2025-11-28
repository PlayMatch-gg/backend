package models

import "gorm.io/gorm"

type MessageType string

const (
	MessageTypeText   MessageType = "text"
	MessageTypeSystem MessageType = "system"
)

// Message represents a chat message within a lobby.
type Message struct {
	gorm.Model
	LobbyID uint   `gorm:"not null;index"`
	UserID  *uint  // Nullable for system messages
	Type    MessageType `gorm:"size:50;not null;default:'text'"`
	Content string `gorm:"not null"`

	User User `gorm:"foreignKey:UserID"` // Belongs to User
}
