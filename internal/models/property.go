package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Property struct {
	ID          uuid.UUID 		`gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Title       string    		`gorm:"type:varchar(255)"`
	SizeSqM     float64   		`gorm:"type:decimal(8,2);column:size_sq_m"`
	Description string    		`gorm:"type:text"`
	Bedrooms    int32     		`gorm:"type:integer"`
	Bathrooms   float64   		`gorm:"type:numeric(3,1)"`
	Price       float64   		`gorm:"type:numeric(12,2)"`
	OwnerID     uuid.UUID 		`gorm:"type:uuid;column:owner_id"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt	`gorm:"index"`
}

func (Property) TableName() string {
	return "properties"
}
