package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID          uuid.UUID		`gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Email	 	string			`gorm:"type:varchar(100);unique;not null"`
	Password 	[]byte			`gorm:"type:bytea"`
	AvatarURL 	string			`gorm:"type:text"`
	Name	  	string			`gorm:"type:varchar(100)"`
	Phone	  	string			`gorm:"type:varchar(30)"`
	RoleID	  	uint			`gorm:"column:role_id"`
	CreatedAt   time.Time	
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt	`gorm:"index"`
}

func (User) TableName() string {
	return "users"
}