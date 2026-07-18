package models

import (
	"time"

	"github.com/google/uuid"
)

type PropertyFeature struct {
	PropertyID uuid.UUID `gorm:"type:uuid;primaryKey;column:property_id"`
	FeatureID  uint      `gorm:"primaryKey;column:feature_id"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (PropertyFeature) TableName() string {
	return "properties_features"
}
