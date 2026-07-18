package models

import "time"

type AuthProvider struct {
	ID        uint   `gorm:"primaryKey;autoIncrement"`
	Name      string `gorm:"type:varchar(100);unique;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

const AuthProviderGoogle = "google"

func (AuthProvider) TableName() string {
	return "auth_providers"
}
