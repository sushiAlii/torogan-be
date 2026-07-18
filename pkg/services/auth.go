package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/sushiAlii/torogan-be/internal/database"
	"github.com/sushiAlii/torogan-be/internal/models"
	utils "github.com/sushiAlii/torogan-be/pkg"
)

type TokenDetails struct {
	AccessToken string
	RefreshToken string
}

type AuthService struct {
	db *gorm.DB
}

func NewAuthService(db *gorm.DB) *AuthService {
	return &AuthService{db: database.GetDB()}
}

func (as *AuthService) Register(email, password string) (*models.User, *TokenDetails, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to process password: %w", err)
	}

	var user models.User
	err = as.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.User{}).Where("email = ?", email).Count(&count).Error; err != nil {
			return err
		}

		if count > 0 {
			return fmt.Errorf("User with email %s already exists", email)
		}

		var defaultRole models.Role
		if err := tx.First(&defaultRole, "name = ?", models.RoleUser).Error; err != nil {
			return fmt.Errorf("Failed to find default role: %w", err)
		}

		user = models.User{
			Email:    email,
			Password: hashedPassword,
			RoleID:   defaultRole.ID,
		}

		return tx.Create(&user).Error
	})

	if err != nil {
		return nil, nil, fmt.Errorf("Failed to register user: %w", err)
	}

	tokens, err := as.GenerateTokenPair(user.ID.String(), models.RoleUser)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to generate token pair: %w", err)
	}

	return &user, tokens, nil
}

func (as *AuthService) Login(email, password string) (*models.User, *TokenDetails, error) {
	var user models.User

	if err := as.db.First(&user, "email = ?", email).Error; err != nil {
        return nil, nil, fmt.Errorf("User not found: %w", err)
    }

	if err := bcrypt.CompareHashAndPassword(user.Password, []byte(password)); err != nil {
		return nil, nil, fmt.Errorf("Invalid password: %w", err)
	}

	var role models.Role
    if err := as.db.First(&role, "id = ?", user.RoleID).Error; err != nil {
        return nil, nil, fmt.Errorf("Failed to resolve user role: %w", err)
    }

	tokens, err := as.GenerateTokenPair(user.ID.String(), role.Name)
    if err != nil {
        return nil, nil, fmt.Errorf("Failed to generate token pair: %w", err)
    }

	return &user, tokens, nil
}

func (as *AuthService) GenerateTokenPair(userID, role string) (*TokenDetails, error) {
	secret := utils.GetEnv("JWT_SECRET", "default_secret")
	if secret == "" {
		return nil, errors.New("JWT_SECRET environment variable is not set")
	}
	
	accessClaims := jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"exp":  time.Now().Add(time.Minute * 15).Unix(),
		"iat": 	time.Now().Unix(),
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(secret))
	if err != nil {
		return nil, fmt.Errorf("Failed to generate access token: %w", err)
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("Failed to generate random bytes for refresh token: %w", err)
	}
	refreshToken := hex.EncodeToString(b)

	// TODO: Later on, we will drop a line here to save this token to Redis!

	return &TokenDetails{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}