package models

import (
	"time"

	"github.com/google/uuid"
)

type PropertyImage struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	PropertyID uuid.UUID `gorm:"type:uuid;column:property_id"`
	URL        string    `gorm:"type:text"`
	IsMain     bool      `gorm:"column:is_main"`
	Position   int32     `gorm:"type:integer"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (PropertyImage) TableName() string {
	return "property_images"
}
