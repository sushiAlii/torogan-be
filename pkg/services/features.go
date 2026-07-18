package services

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/sushiAlii/torogan-be/internal/models"
)

type FeatureService struct {
	db *gorm.DB
}

func NewFeatureService(db *gorm.DB) *FeatureService {
	return &FeatureService{db: db}
}

func (s *FeatureService) CreateFeature(f models.Feature) (*models.Feature, error) {
	newFeature := models.Feature{
		Name: f.Name,
	}

	if err := s.db.Create(&newFeature).Error; err != nil {
		return nil, fmt.Errorf("failed to create feature: %w", err)
	}

	return &newFeature, nil
}

func (s *FeatureService) GetFeatureByID(id uint) (*models.Feature, error) {
	var dbFeature models.Feature
	if err := s.db.First(&dbFeature, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to fetch feature: %w", err)
	}

	return &dbFeature, nil
}

func (s *FeatureService) ListFeatures(search string, limit int, cursor uint) ([]models.Feature, int64, error) {
	if limit <= 0 {
		limit = 25
	}

	query := s.db.Model(&models.Feature{})
	if search != "" {
		query = query.Where("name ILIKE ?", "%"+search+"%")
	}

	var totalCount int64
	if err := query.Count(&totalCount).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count features: %w", err)
	}

	query = query.Order("id ASC").Where("id > ?", cursor).Limit(limit)

	var dbFeatures []models.Feature
	if err := query.Find(&dbFeatures).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list features: %w", err)
	}

	return dbFeatures, totalCount, nil
}

func (s *FeatureService) UpdateFeatureByID(f models.Feature) (*models.Feature, error) {
	var dbFeature models.Feature
	if err := s.db.First(&dbFeature, "id = ?", f.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to fetch feature: %w", err)
	}

	dbFeature.Name = f.Name

	if err := s.db.Save(&dbFeature).Error; err != nil {
		return nil, fmt.Errorf("failed to update feature: %w", err)
	}

	return &dbFeature, nil
}

func (s *FeatureService) DeleteFeatureByID(id uint) error {
	result := s.db.Delete(&models.Feature{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete feature: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}
