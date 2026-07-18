package services

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/sushiAlii/torogan-be/internal/models"
)

// ErrMaxPropertyImages is returned when a property already has the maximum
// of 5 images and another add is attempted.
var ErrMaxPropertyImages = errors.New("property already has the maximum of 5 images")

const maxPropertyImages = 5

type PropertyService struct {
	db *gorm.DB
}

func NewPropertyService(db *gorm.DB) *PropertyService {
	return &PropertyService{db: db}
}

func (s *PropertyService) CreateProperty(p models.Property) (*models.Property, error) {
	newProperty := models.Property{
		Title:       p.Title,
		Type:        p.Type,
		SizeSqM:     p.SizeSqM,
		Description: p.Description,
		Bedrooms:    p.Bedrooms,
		Bathrooms:   p.Bathrooms,
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

// GetOwner loads the user record for a property's owner, for populating
// OwnerContact on authenticated GetPropertyByID responses.
func (s *PropertyService) GetOwner(ownerID uuid.UUID) (*models.User, error) {
	var owner models.User
	if err := s.db.First(&owner, "id = ?", ownerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to fetch property owner: %w", err)
	}

	return &owner, nil
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
	dbProperty.Type = p.Type
	dbProperty.SizeSqM = p.SizeSqM
	dbProperty.Description = p.Description
	dbProperty.Bedrooms = p.Bedrooms
	dbProperty.Bathrooms = p.Bathrooms
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

// AddPropertyImage attaches an image to a property. The first image on a
// property is always forced to be the main image; setting is_main=true on a
// later image demotes whichever image currently holds it, keeping the
// property_images_one_main_idx partial unique index satisfied.
func (s *PropertyService) AddPropertyImage(propertyID uuid.UUID, url string, isMain bool, position int32) ([]models.PropertyImage, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Lock the parent property row for the duration of the transaction
		// so concurrent AddPropertyImage/RemovePropertyImage calls for the
		// same property serialize instead of racing: without this, two
		// concurrent transactions can both read count=0 before either
		// commits and both try to insert is_main=true, violating
		// property_images_one_main_idx.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&models.Property{}, "id = ?", propertyID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			return fmt.Errorf("failed to fetch property: %w", err)
		}

		var count int64
		if err := tx.Model(&models.PropertyImage{}).Where("property_id = ?", propertyID).Count(&count).Error; err != nil {
			return fmt.Errorf("failed to count property images: %w", err)
		}

		if count >= maxPropertyImages {
			return ErrMaxPropertyImages
		}

		if count == 0 {
			isMain = true
		}

		if isMain {
			err := tx.Model(&models.PropertyImage{}).
				Where("property_id = ? AND is_main", propertyID).
				Update("is_main", false).Error
			if err != nil {
				return fmt.Errorf("failed to demote current main image: %w", err)
			}
		}

		image := models.PropertyImage{
			PropertyID: propertyID,
			URL:        url,
			IsMain:     isMain,
			Position:   position,
		}
		if err := tx.Create(&image).Error; err != nil {
			return fmt.Errorf("failed to add property image: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.ListPropertyImages(propertyID)
}

// RemovePropertyImage removes an image from a property. If the removed
// image was the main image and other images remain, the lowest-position
// survivor is promoted to main.
func (s *PropertyService) RemovePropertyImage(propertyID, imageID uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// See AddPropertyImage: lock the parent property row so this
		// serializes against concurrent Add/RemovePropertyImage calls
		// instead of racing on the is_main promotion logic below.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&models.Property{}, "id = ?", propertyID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			return fmt.Errorf("failed to fetch property: %w", err)
		}

		var image models.PropertyImage
		if err := tx.First(&image, "id = ? AND property_id = ?", imageID, propertyID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			return fmt.Errorf("failed to fetch property image: %w", err)
		}

		if err := tx.Delete(&models.PropertyImage{}, "id = ?", imageID).Error; err != nil {
			return fmt.Errorf("failed to remove property image: %w", err)
		}

		if !image.IsMain {
			return nil
		}

		var survivor models.PropertyImage
		err := tx.Where("property_id = ?", propertyID).
			Order("position ASC, created_at ASC").
			First(&survivor).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return fmt.Errorf("failed to find replacement main image: %w", err)
		}

		if err := tx.Model(&survivor).Update("is_main", true).Error; err != nil {
			return fmt.Errorf("failed to promote replacement main image: %w", err)
		}

		return nil
	})
}

func (s *PropertyService) ListPropertyImages(propertyID uuid.UUID) ([]models.PropertyImage, error) {
	var images []models.PropertyImage
	err := s.db.Where("property_id = ?", propertyID).
		Order("position ASC, created_at ASC").
		Find(&images).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list property images: %w", err)
	}

	return images, nil
}

// GetMainImageURLs batches the main-image lookup for a page of properties,
// used by GetPropertyList to avoid an N+1 query per property.
func (s *PropertyService) GetMainImageURLs(propertyIDs []uuid.UUID) (map[uuid.UUID]string, error) {
	urls := make(map[uuid.UUID]string, len(propertyIDs))
	if len(propertyIDs) == 0 {
		return urls, nil
	}

	var images []models.PropertyImage
	err := s.db.Where("property_id IN ? AND is_main", propertyIDs).Find(&images).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch main image urls: %w", err)
	}

	for _, img := range images {
		urls[img.PropertyID] = img.URL
	}

	return urls, nil
}
