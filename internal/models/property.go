package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Property struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Title       string    `gorm:"type:varchar(255)"`
	Type        string    `gorm:"type:varchar(50)"`
	SizeSqM     float64   `gorm:"type:decimal(8,2);column:size_sq_m"`
	Description string    `gorm:"type:text"`
	Bedrooms    int32     `gorm:"type:integer"`
	Bathrooms   float64   `gorm:"type:numeric(3,1)"`
	Price       float64   `gorm:"type:numeric(12,2)"`
	OwnerID     uuid.UUID `gorm:"type:uuid;column:owner_id"`
	ExpiresAt   time.Time `gorm:"column:expires_at;not null"`
	IsRented    bool      `gorm:"column:is_rented;not null;default:false"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

const (
	PropertyTypeApartment = "Apartment"
	PropertyTypeStudio    = "Studio"
	PropertyTypeHouse     = "House"
	PropertyTypeTownhouse = "Townhouse"
)

// IsValidPropertyType reports whether t is one of the known property types.
func IsValidPropertyType(t string) bool {
	switch t {
	case PropertyTypeApartment, PropertyTypeStudio, PropertyTypeHouse, PropertyTypeTownhouse:
		return true
	default:
		return false
	}
}

// Valid listing durations, in days, a landlord can pick at creation or renewal.
const (
	ExpirationDaysShort  int32 = 7
	ExpirationDaysMedium int32 = 15
	ExpirationDaysLong   int32 = 30
)

// IsValidExpirationDays reports whether days is one of the offered listing durations.
func IsValidExpirationDays(days int32) bool {
	switch days {
	case ExpirationDaysShort, ExpirationDaysMedium, ExpirationDaysLong:
		return true
	default:
		return false
	}
}

const (
	PropertyStatusActive  = "active"
	PropertyStatusExpired = "expired"
	PropertyStatusRented  = "rented"
)

// Status derives the listing's display status from IsRented and ExpiresAt.
// It is intentionally not persisted — storing it would be a second,
// driftable source of truth alongside those two columns.
func (p Property) Status() string {
	if p.IsRented {
		return PropertyStatusRented
	}
	if time.Now().After(p.ExpiresAt) {
		return PropertyStatusExpired
	}
	return PropertyStatusActive
}

func (Property) TableName() string {
	return "properties"
}
