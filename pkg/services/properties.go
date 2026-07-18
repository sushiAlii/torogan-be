package services

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/sushiAlii/torogan-be/internal/models"
)

type PropertyService struct {
	db *gorm.DB
}

func NewPropertyService(db *gorm.DB) *PropertyService {
	return &PropertyService{db: db}
}

func (s *PropertyService) CreateProperty(p models.Property) (*models.Property, error) {
	newProperty := models.Property{
		Title:       p.Title,
		SizeSqM:     p.SizeSqM,
		Description: p.Description,
		Price:       p.Price,
		OwnerID:     p.OwnerID,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		return tx.Create(&newProperty).Error
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create property: %w", err)
	}

	return &newProperty, nil
}

func (s *PropertyService) GetPropertyByID(id uuid.UUID) (*models.Property, error) {
	var dbProperty models.Property
	if err := s.db.First(&dbProperty, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to fetch property: %w", err)
	}

	return &dbProperty, nil
}

func (s *PropertyService) GetPropertyList(search string, limit int, cursorUUID uuid.UUID) ([]models.Property, int64, error) {
	if limit <= 0 {
		limit = 25
	}

	query := s.db.Model(&models.Property{})
	if search != "" {
		query = query.Where("title ILIKE ?", "%"+search+"%")
	}

	var totalCount int64
	if err := query.Count(&totalCount).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count properties: %w", err)
	}

	query = query.Order("id ASC").Where("id > ?", cursorUUID).Limit(limit)

	var dbProperties []models.Property
	if err := query.Find(&dbProperties).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list properties: %w", err)
	}

	return dbProperties, totalCount, nil
}

func (s *PropertyService) UpdatePropertyByID(p models.Property) (*models.Property, error) {
	var dbProperty models.Property
	if err := s.db.First(&dbProperty, "id = ?", p.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to fetch property: %w", err)
	}

	dbProperty.Title = p.Title
	dbProperty.SizeSqM = p.SizeSqM
	dbProperty.Description = p.Description
	dbProperty.Price = p.Price

	if err := s.db.Save(&dbProperty).Error; err != nil {
		return nil, fmt.Errorf("failed to update property: %w", err)
	}

	return &dbProperty, nil
}

func (s *PropertyService) DeletePropertyByID(id uuid.UUID) error {
	result := s.db.Delete(&models.Property{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete property: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (s *PropertyService) AddPropertyFeature(propertyID uuid.UUID, featureID uint) error {
	link := models.PropertyFeature{
		PropertyID: propertyID,
		FeatureID:  featureID,
	}

	if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error; err != nil {
		return fmt.Errorf("failed to attach feature to property: %w", err)
	}

	return nil
}

func (s *PropertyService) RemovePropertyFeature(propertyID uuid.UUID, featureID uint) error {
	result := s.db.Where("property_id = ? AND feature_id = ?", propertyID, featureID).Delete(&models.PropertyFeature{})
	if result.Error != nil {
		return fmt.Errorf("failed to detach feature from property: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (s *PropertyService) ListPropertyFeatures(propertyID uuid.UUID) ([]models.Feature, error) {
	var features []models.Feature
	err := s.db.Model(&models.Feature{}).
		Joins("JOIN properties_features pf ON pf.feature_id = features.id").
		Where("pf.property_id = ?", propertyID).
		Order("features.id ASC").
		Find(&features).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list property features: %w", err)
	}

	return features, nil
}
