package services

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sushiAlii/torogan-be/internal/models"
)

type AddressService struct {
	db *gorm.DB
}

func NewAddressService(db *gorm.DB) *AddressService {
	return &AddressService{db: db}
}

func (s *AddressService) CreateAddress(a models.Address, ownerID uuid.UUID) (*models.Address, error) {
	if err := s.verifyPropertyOwner(a.PropertyID, ownerID); err != nil {
		return nil, err
	}

	newAddress := models.Address{
		StreetAddress:   a.StreetAddress,
		ExtendedAddress: a.ExtendedAddress,
		City:            a.City,
		State:           a.State,
		CountryCode:     a.CountryCode,
		Latitude:        a.Latitude,
		Longitude:       a.Longitude,
		GooglePlaceID:   a.GooglePlaceID,
		PropertyID:      a.PropertyID,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		return tx.Create(&newAddress).Error
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create address: %w", err)
	}

	return &newAddress, nil
}

func (s *AddressService) GetAddressByID(id uuid.UUID) (*models.Address, error) {
	var dbAddress models.Address
	if err := s.db.First(&dbAddress, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to fetch address: %w", err)
	}

	return &dbAddress, nil
}

func (s *AddressService) GetAddressByPropertyID(propertyID uuid.UUID) (*models.Address, error) {
	var dbAddress models.Address
	if err := s.db.First(&dbAddress, "property_id = ?", propertyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to fetch address: %w", err)
	}

	return &dbAddress, nil
}

func (s *AddressService) UpdateAddressByID(a models.Address, ownerID uuid.UUID) (*models.Address, error) {
	var dbAddress models.Address
	if err := s.db.First(&dbAddress, "id = ?", a.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to fetch address: %w", err)
	}

	if err := s.verifyPropertyOwner(dbAddress.PropertyID, ownerID); err != nil {
		return nil, err
	}

	dbAddress.StreetAddress = a.StreetAddress
	dbAddress.ExtendedAddress = a.ExtendedAddress
	dbAddress.City = a.City
	dbAddress.State = a.State
	dbAddress.CountryCode = a.CountryCode
	dbAddress.Latitude = a.Latitude
	dbAddress.Longitude = a.Longitude
	dbAddress.GooglePlaceID = a.GooglePlaceID

	if err := s.db.Save(&dbAddress).Error; err != nil {
		return nil, fmt.Errorf("failed to update address: %w", err)
	}

	return &dbAddress, nil
}

func (s *AddressService) DeleteAddressByID(id uuid.UUID, ownerID uuid.UUID) error {
	var dbAddress models.Address
	if err := s.db.First(&dbAddress, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return fmt.Errorf("failed to fetch address: %w", err)
	}

	if err := s.verifyPropertyOwner(dbAddress.PropertyID, ownerID); err != nil {
		return err
	}

	result := s.db.Delete(&models.Address{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete address: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// verifyPropertyOwner loads the property that owns an address (Address has
// no owner_id of its own — ownership only exists via the property it
// belongs to) and confirms ownerID matches, reusing the same verifyOwner/
// ErrNotOwner convention as PropertyService (same package).
func (s *AddressService) verifyPropertyOwner(propertyID uuid.UUID, ownerID uuid.UUID) error {
	var property models.Property
	if err := s.db.First(&property, "id = ?", propertyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return fmt.Errorf("failed to fetch property: %w", err)
	}

	return verifyOwner(property, ownerID)
}
