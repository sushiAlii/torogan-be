package models

import (
	"time"

	"github.com/google/uuid"
)

type Address struct {
	ID              uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	StreetAddress   string    `gorm:"type:varchar(255);not null;column:street_address"`
	ExtendedAddress string    `gorm:"type:varchar(255);column:extended_address"`
	City            string    `gorm:"type:varchar(100);not null"`
	State           string    `gorm:"type:varchar(100);not null"`
	CountryCode     string    `gorm:"type:varchar(2);not null;column:country_code"`
	Latitude        float64   `gorm:"type:decimal(10,8);not null"`
	Longitude       float64   `gorm:"type:decimal(11,8);not null"`
	GooglePlaceID   string    `gorm:"type:text;column:google_place_id"`
	PropertyID      uuid.UUID `gorm:"type:uuid;column:property_id"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (Address) TableName() string {
	return "addresses"
}
