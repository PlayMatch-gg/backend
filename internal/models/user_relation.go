package models

import "time"

// FriendshipStatus defines the state of a relationship between two users.
type FriendshipStatus string

const (
	// StatusPending means a friend request has been sent but not yet accepted.
	// This can be seen as a one-way "subscription" or "follow".
	StatusPending FriendshipStatus = "pending"

	// StatusAccepted means the friend request was accepted, and the users are now friends.
	StatusAccepted FriendshipStatus = "accepted"
)

// UserRelation represents the relationship between two users.
// The primary key is a composite of (FromUserID, ToUserID) to ensure uniqueness.
type UserRelation struct {
	FromUserID uint             `gorm:"primaryKey"`
	ToUserID   uint             `gorm:"primaryKey"`
	Status     FriendshipStatus `gorm:"type:varchar(20);not null"`
	CreatedAt  time.Time
	UpdatedAt  time.Time

	// Define foreign key relationships
	FromUser User `gorm:"foreignKey:FromUserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	ToUser   User `gorm:"foreignKey:ToUserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
