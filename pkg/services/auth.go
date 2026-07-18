package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/idtoken"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/sushiAlii/torogan-be/internal/models"
	utils "github.com/sushiAlii/torogan-be/pkg"
)

type TokenDetails struct {
	AccessToken  string
	RefreshToken string
}

type AuthService struct {
	db *gorm.DB
}

func NewAuthService(db *gorm.DB) *AuthService {
	return &AuthService{db: db}
}

func (as *AuthService) Register(email, password string) (*models.User, string, *TokenDetails, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to process password: %w", err)
	}

	var user models.User
	err = as.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.User{}).Where("email = ?", email).Count(&count).Error; err != nil {
			return err
		}

		if count > 0 {
			return fmt.Errorf("user with email %s already exists", email)
		}

		var defaultRole models.Role
		if err := tx.First(&defaultRole, "name = ?", models.RoleUser).Error; err != nil {
			return fmt.Errorf("failed to find default role: %w", err)
		}

		user = models.User{
			Email:    email,
			Password: hashedPassword,
			RoleID:   defaultRole.ID,
		}

		return tx.Create(&user).Error
	})
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to register user: %w", err)
	}

	tokens, err := as.GenerateTokenPair(user.ID.String(), models.RoleUser)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to generate token pair: %w", err)
	}

	return &user, models.RoleUser, tokens, nil
}

func (as *AuthService) Login(email, password string) (*models.User, string, *TokenDetails, error) {
	var user models.User

	if err := as.db.First(&user, "email = ?", email).Error; err != nil {
		return nil, "", nil, fmt.Errorf("user not found: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword(user.Password, []byte(password)); err != nil {
		return nil, "", nil, fmt.Errorf("invalid password: %w", err)
	}

	var role models.Role
	if err := as.db.First(&role, "id = ?", user.RoleID).Error; err != nil {
		return nil, "", nil, fmt.Errorf("failed to resolve user role: %w", err)
	}

	tokens, err := as.GenerateTokenPair(user.ID.String(), role.Name)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to generate token pair: %w", err)
	}

	return &user, role.Name, tokens, nil
}

// SignInWithGoogle verifies a Google-issued ID token, finds or creates the
// corresponding local user, and links the Google identity via
// user_auth_providers.
func (as *AuthService) SignInWithGoogle(ctx context.Context, idTokenStr string) (*models.User, string, *TokenDetails, error) {
	clientID := utils.GetEnv("GOOGLE_CLIENT_ID", "")
	if clientID == "" {
		return nil, "", nil, errors.New("GOOGLE_CLIENT_ID environment variable is not set")
	}

	payload, err := idtoken.Validate(ctx, idTokenStr, clientID)
	if err != nil {
		return nil, "", nil, fmt.Errorf("invalid google id token: %w", err)
	}

	email, _ := payload.Claims["email"].(string)
	if email == "" {
		return nil, "", nil, errors.New("google id token missing email")
	}
	picture, _ := payload.Claims["picture"].(string)

	var user models.User
	var roleName string

	err = as.db.Transaction(func(tx *gorm.DB) error {
		var provider models.AuthProvider
		if err := tx.Where(&models.AuthProvider{Name: models.AuthProviderGoogle}).FirstOrCreate(&provider).Error; err != nil {
			return fmt.Errorf("failed to resolve auth provider: %w", err)
		}

		if err := tx.First(&user, "email = ?", email).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("failed to look up user: %w", err)
			}

			var defaultRole models.Role
			if err := tx.First(&defaultRole, "name = ?", models.RoleUser).Error; err != nil {
				return fmt.Errorf("failed to find default role: %w", err)
			}

			user = models.User{
				Email:     email,
				AvatarURL: picture,
				RoleID:    defaultRole.ID,
			}
			if err := tx.Create(&user).Error; err != nil {
				return fmt.Errorf("failed to create user: %w", err)
			}
		}

		var role models.Role
		if err := tx.First(&role, "id = ?", user.RoleID).Error; err != nil {
			return fmt.Errorf("failed to resolve user role: %w", err)
		}
		roleName = role.Name

		link := models.UserAuthProvider{
			UserID:         user.ID,
			AuthProviderID: provider.ID,
			SubID:          payload.Subject,
		}
		err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "auth_provider_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"sub_id"}),
		}).Create(&link).Error
		if err != nil {
			return fmt.Errorf("failed to link auth provider: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, "", nil, err
	}

	tokens, err := as.GenerateTokenPair(user.ID.String(), roleName)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to generate token pair: %w", err)
	}

	return &user, roleName, tokens, nil
}

// GenerateAccessToken mints a short-lived (15 min) HS256 access token.
func (as *AuthService) GenerateAccessToken(userID, role string) (string, error) {
	secret := utils.GetEnv("JWT_SECRET", "default_secret")
	if secret == "" {
		return "", errors.New("JWT_SECRET environment variable is not set")
	}

	claims := jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"exp":  time.Now().Add(time.Minute * 15).Unix(),
		"iat":  time.Now().Unix(),
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("failed to generate access token: %w", err)
	}

	return accessToken, nil
}

// generateRefreshToken mints a stateless, longer-lived (7 day) HS256 JWT
// carrying typ="refresh" and a jti so it can be individually revoked later
// (e.g. via a Redis/DB denylist keyed on jti) without needing that
// infrastructure today.
func (as *AuthService) generateRefreshToken(userID, role string) (string, error) {
	secret := utils.GetEnv("JWT_SECRET", "default_secret")
	if secret == "" {
		return "", errors.New("JWT_SECRET environment variable is not set")
	}

	jtiBytes := make([]byte, 16)
	if _, err := rand.Read(jtiBytes); err != nil {
		return "", fmt.Errorf("failed to generate refresh token id: %w", err)
	}

	claims := jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"typ":  "refresh",
		"jti":  hex.EncodeToString(jtiBytes),
		"exp":  time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":  time.Now().Unix(),
	}
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return refreshToken, nil
}

func (as *AuthService) GenerateTokenPair(userID, role string) (*TokenDetails, error) {
	accessToken, err := as.GenerateAccessToken(userID, role)
	if err != nil {
		return nil, err
	}

	refreshToken, err := as.generateRefreshToken(userID, role)
	if err != nil {
		return nil, err
	}

	return &TokenDetails{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// ValidateRefreshToken validates a refresh JWT and returns the subject
// (user ID) and role carried in its claims.
func (as *AuthService) ValidateRefreshToken(tokenStr string) (userID, role string, err error) {
	secret := utils.GetEnv("JWT_SECRET", "default_secret")
	if secret == "" {
		return "", "", errors.New("JWT_SECRET environment variable is not set")
	}

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return "", "", fmt.Errorf("invalid refresh token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", errors.New("invalid refresh token claims")
	}

	if typ, _ := claims["typ"].(string); typ != "refresh" {
		return "", "", errors.New("token is not a refresh token")
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", "", errors.New("refresh token missing subject")
	}
	roleClaim, _ := claims["role"].(string)

	return sub, roleClaim, nil
}
