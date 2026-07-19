package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/sushiAlii/torogan-be/internal/models"
)

// ErrMaxPropertyImages is returned when a property already has the maximum
// of 5 images and another add is attempted.
var ErrMaxPropertyImages = errors.New("property already has the maximum of 5 images")

// ErrNotOwner is returned when the authenticated caller does not own the
// property they're trying to mutate. Handlers map this to CodeNotFound on
// the wire (not CodePermissionDenied), matching every other not-found
// branch in this service and avoiding confirming to a prober that the
// resource exists under someone else's account.
var ErrNotOwner = errors.New("caller does not own this property")

const maxPropertyImages = 5

// verifyOwner checks an already-loaded property against the caller's ID.
// Call sites that already load the property row for another reason (e.g.
// the locking read in AddPropertyImage) should pass that row instead of
// triggering a second query.
func verifyOwner(p models.Property, ownerID uuid.UUID) error {
	if p.OwnerID != ownerID {
		return ErrNotOwner
	}
	return nil
}

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
		ExpiresAt:   p.ExpiresAt,
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

	// Public browse excludes rented and expired listings — they're no
	// longer available, so they shouldn't surface in search. Direct fetch
	// by ID (GetPropertyByID) is intentionally left unfiltered so a
	// bookmarked/shared link to a lapsed listing still resolves.
	query := s.db.Model(&models.Property{}).Where("is_rented = ? AND expires_at > ?", false, time.Now())
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

func (s *PropertyService) UpdatePropertyByID(p models.Property, ownerID uuid.UUID) (*models.Property, error) {
	var dbProperty models.Property
	if err := s.db.First(&dbProperty, "id = ?", p.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to fetch property: %w", err)
	}

	if err := verifyOwner(dbProperty, ownerID); err != nil {
		return nil, err
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

func (s *PropertyService) DeletePropertyByID(id uuid.UUID, ownerID uuid.UUID) error {
	var dbProperty models.Property
	if err := s.db.First(&dbProperty, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return fmt.Errorf("failed to fetch property: %w", err)
	}

	if err := verifyOwner(dbProperty, ownerID); err != nil {
		return err
	}

	result := s.db.Delete(&models.Property{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete property: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// GetMyPropertyList returns every listing owned by ownerID, regardless of
// status, ordered soonest-to-expire first. Unlike GetPropertyList (public
// browse), this is intentionally unfiltered and unpaginated: an owner needs
// to see everything, and the frontend filters this single fetched set by
// status client-side for the all/active/expired/rented tabs. If this ever
// grows a cursor, that tab filtering has to move server-side at the same
// time — see the comment in app/my-listings/page.tsx.
func (s *PropertyService) GetMyPropertyList(ownerID uuid.UUID) ([]models.Property, error) {
	var dbProperties []models.Property
	err := s.db.Where("owner_id = ?", ownerID).Order("expires_at ASC").Find(&dbProperties).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list my properties: %w", err)
	}

	return dbProperties, nil
}

// RenewProperty extends a listing's expiry to a fresh window starting now.
// It deliberately leaves IsRented untouched — renewal and rented-status are
// orthogonal concerns.
func (s *PropertyService) RenewProperty(id uuid.UUID, ownerID uuid.UUID, days int32) (*models.Property, error) {
	var dbProperty models.Property
	if err := s.db.First(&dbProperty, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to fetch property: %w", err)
	}

	if err := verifyOwner(dbProperty, ownerID); err != nil {
		return nil, err
	}

	dbProperty.ExpiresAt = time.Now().AddDate(0, 0, int(days))

	if err := s.db.Save(&dbProperty).Error; err != nil {
		return nil, fmt.Errorf("failed to renew property: %w", err)
	}

	return &dbProperty, nil
}

// MarkPropertyRented and MarkPropertyAvailable are unconditional — there is
// no expiry precondition on un-renting. If a listing's window already
// lapsed while it was rented, un-renting correctly leaves it "expired" (via
// Property.Status()'s rented > expired > active precedence), with Renew
// available right there for the owner's next action.
func (s *PropertyService) MarkPropertyRented(id uuid.UUID, ownerID uuid.UUID) (*models.Property, error) {
	return s.setRented(id, ownerID, true)
}

func (s *PropertyService) MarkPropertyAvailable(id uuid.UUID, ownerID uuid.UUID) (*models.Property, error) {
	return s.setRented(id, ownerID, false)
}

func (s *PropertyService) setRented(id uuid.UUID, ownerID uuid.UUID, rented bool) (*models.Property, error) {
	var dbProperty models.Property
	if err := s.db.First(&dbProperty, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to fetch property: %w", err)
	}

	if err := verifyOwner(dbProperty, ownerID); err != nil {
		return nil, err
	}

	dbProperty.IsRented = rented

	if err := s.db.Save(&dbProperty).Error; err != nil {
		return nil, fmt.Errorf("failed to update property rented status: %w", err)
	}

	return &dbProperty, nil
}

func (s *PropertyService) AddPropertyFeature(propertyID uuid.UUID, featureID uint, ownerID uuid.UUID) error {
	var dbProperty models.Property
	if err := s.db.First(&dbProperty, "id = ?", propertyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return fmt.Errorf("failed to fetch property: %w", err)
	}
	if err := verifyOwner(dbProperty, ownerID); err != nil {
		return err
	}

	link := models.PropertyFeature{
		PropertyID: propertyID,
		FeatureID:  featureID,
	}

	if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error; err != nil {
		return fmt.Errorf("failed to attach feature to property: %w", err)
	}

	return nil
}

func (s *PropertyService) RemovePropertyFeature(propertyID uuid.UUID, featureID uint, ownerID uuid.UUID) error {
	var dbProperty models.Property
	if err := s.db.First(&dbProperty, "id = ?", propertyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return fmt.Errorf("failed to fetch property: %w", err)
	}
	if err := verifyOwner(dbProperty, ownerID); err != nil {
		return err
	}

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
func (s *PropertyService) AddPropertyImage(propertyID uuid.UUID, url string, isMain bool, position int32, ownerID uuid.UUID) ([]models.PropertyImage, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Lock the parent property row for the duration of the transaction
		// so concurrent AddPropertyImage/RemovePropertyImage calls for the
		// same property serialize instead of racing: without this, two
		// concurrent transactions can both read count=0 before either
		// commits and both try to insert is_main=true, violating
		// property_images_one_main_idx.
		var dbProperty models.Property
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&dbProperty, "id = ?", propertyID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			return fmt.Errorf("failed to fetch property: %w", err)
		}
		if err := verifyOwner(dbProperty, ownerID); err != nil {
			return err
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
func (s *PropertyService) RemovePropertyImage(propertyID, imageID uuid.UUID, ownerID uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// See AddPropertyImage: lock the parent property row so this
		// serializes against concurrent Add/RemovePropertyImage calls
		// instead of racing on the is_main promotion logic below.
		var dbProperty models.Property
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&dbProperty, "id = ?", propertyID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			return fmt.Errorf("failed to fetch property: %w", err)
		}
		if err := verifyOwner(dbProperty, ownerID); err != nil {
			return err
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
