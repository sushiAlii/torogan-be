package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID          uuid.UUID		`gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Email	 	string			`gorm:"type:varchar(100);unique;not null"`
	Password 	[]byte			`gorm:"type:bytea;not null"`
	AvatarURL 	string			`gorm:"type:text"`
	RoleID	  	uint			`gorm:"not null"`
	CreatedAt   time.Time	
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt	`gorm:"index"`
}

func (User) TableName() string {
	return "users"
}