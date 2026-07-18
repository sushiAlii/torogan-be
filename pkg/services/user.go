package services

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sushiAlii/torogan-be/internal/models"
)

type UserService struct {
	db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

func (s *UserService) GetByID(id uuid.UUID) (*models.User, string, error) {
	var user models.User
	if err := s.db.First(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", err
		}
		return nil, "", fmt.Errorf("failed to fetch user: %w", err)
	}

	role, err := s.resolveRole(user.RoleID)
	if err != nil {
		return nil, "", err
	}

	return &user, role, nil
}

func (s *UserService) resolveRole(roleID uint) (string, error) {
	var role models.Role
	if err := s.db.First(&role, "id = ?", roleID).Error; err != nil {
		return "", fmt.Errorf("failed to resolve user role: %w", err)
	}
	return role.Name, nil
}

// UpdateContact updates only the editable profile fields (name, phone) on a
// user, leaving email/password/avatar/role untouched.
func (s *UserService) UpdateContact(id uuid.UUID, name, phone string) (*models.User, string, error) {
	var user models.User
	if err := s.db.First(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", err
		}
		return nil, "", fmt.Errorf("failed to fetch user: %w", err)
	}

	user.Name = name
	user.Phone = phone

	if err := s.db.Save(&user).Error; err != nil {
		return nil, "", fmt.Errorf("failed to update user: %w", err)
	}

	role, err := s.resolveRole(user.RoleID)
	if err != nil {
		return nil, "", err
	}

	return &user, role, nil
}
