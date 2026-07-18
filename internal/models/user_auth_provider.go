package models

import (
	"time"

	"github.com/google/uuid"
)

type UserAuthProvider struct {
	UserID         uuid.UUID `gorm:"type:uuid;primaryKey;column:user_id"`
	AuthProviderID uint      `gorm:"primaryKey;column:auth_provider_id"`
	SubID          string    `gorm:"type:varchar(255);column:sub_id"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (UserAuthProvider) TableName() string {
	return "user_auth_providers"
}
