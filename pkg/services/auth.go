package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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
	db         *gorm.DB
	emailSvc   *EmailService
	appBaseURL string

	// resendCooldowns is a lightweight in-memory per-email rate limit for
	// ResendVerificationEmail, keyed regardless of whether the address
	// belongs to an account (so the cooldown itself never leaks account
	// existence). Fine for a single instance; swap for Redis if the backend
	// is ever scaled horizontally.
	resendCooldowns   map[string]time.Time
	resendCooldownsMu sync.Mutex
}

func NewAuthService(db *gorm.DB, emailSvc *EmailService, appBaseURL string) *AuthService {
	return &AuthService{
		db:              db,
		emailSvc:        emailSvc,
		appBaseURL:      appBaseURL,
		resendCooldowns: make(map[string]time.Time),
	}
}

// ErrIncompleteContactInfo is returned when a caller-supplied name or phone
// is empty/whitespace-only. Name and phone are required at registration
// (see also ErrIncompleteProfile in properties.go, the analogous check for
// listing creation on accounts predating this requirement).
var ErrIncompleteContactInfo = errors.New("name and phone are required")

// ErrEmailNotVerified is returned by Login when the account's email has not
// been verified yet. Only reachable for traditional (password) accounts —
// Google sign-ins verify email automatically from the id_token claim.
var ErrEmailNotVerified = errors.New("email not verified")

const resendVerificationCooldown = 60 * time.Second

func (as *AuthService) Register(ctx context.Context, email, password, name, phone string) (*models.User, string, *TokenDetails, error) {
	name = strings.TrimSpace(name)
	phone = strings.TrimSpace(phone)
	if name == "" || phone == "" {
		return nil, "", nil, ErrIncompleteContactInfo
	}

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
			Name:     name,
			Phone:    phone,
			RoleID:   defaultRole.ID,
		}

		return tx.Create(&user).Error
	})
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to register user: %w", err)
	}

	// Traditional registrations aren't logged in until email is verified —
	// send the link and return without minting tokens. A nil TokenDetails
	// is the handler's signal that email_verification_required is true.
	// Google accounts skip this entirely; see SignInWithGoogle. A failure to
	// send here doesn't fail registration (the account already exists and
	// re-registering the same email would now error) — the user can always
	// hit ResendVerificationEmail instead.
	as.sendVerificationEmail(ctx, user)

	return &user, models.RoleUser, nil, nil
}

// sendVerificationEmail mints an email-verification token for user and
// emails the link. Errors are logged, not returned — callers treat sending
// as best-effort so a transient email-provider failure never masks an
// otherwise-successful account action (registration, resend).
func (as *AuthService) sendVerificationEmail(ctx context.Context, user models.User) {
	token, err := as.generateEmailVerifyToken(user.ID.String())
	if err != nil {
		log.Printf("failed to generate verification token for %s: %v", user.Email, err)
		return
	}

	link := fmt.Sprintf("%s/verify-email?token=%s", strings.TrimRight(as.appBaseURL, "/"), token)
	if err := as.emailSvc.SendVerificationEmail(ctx, user.Email, link); err != nil {
		log.Printf("failed to send verification email to %s: %v", user.Email, err)
	}
}

func (as *AuthService) Login(email, password string) (*models.User, string, *TokenDetails, error) {
	var user models.User

	if err := as.db.First(&user, "email = ?", email).Error; err != nil {
		return nil, "", nil, fmt.Errorf("user not found: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword(user.Password, []byte(password)); err != nil {
		return nil, "", nil, fmt.Errorf("invalid password: %w", err)
	}

	if user.EmailVerifiedAt == nil {
		return nil, "", nil, ErrEmailNotVerified
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
	name, _ := payload.Claims["name"].(string)
	emailVerified, _ := payload.Claims["email_verified"].(bool)

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
				Name:      name,
				RoleID:    defaultRole.ID,
			}
			// Google only asserts an id_token's email as verified once the
			// account itself has completed Google's own verification, so we
			// trust the claim directly rather than sending our own email.
			if emailVerified {
				now := time.Now()
				user.EmailVerifiedAt = &now
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

// GetUserByID loads a user and their resolved role name by user ID string
// (as carried in JWT "sub" claims).
func (as *AuthService) GetUserByID(userID string) (*models.User, string, error) {
	parsedID, err := uuid.Parse(userID)
	if err != nil {
		return nil, "", fmt.Errorf("invalid user id: %w", err)
	}

	var user models.User
	if err := as.db.First(&user, "id = ?", parsedID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", err
		}
		return nil, "", fmt.Errorf("failed to fetch user: %w", err)
	}

	var role models.Role
	if err := as.db.First(&role, "id = ?", user.RoleID).Error; err != nil {
		return nil, "", fmt.Errorf("failed to resolve user role: %w", err)
	}

	return &user, role.Name, nil
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

// ValidateAccessToken validates an access JWT (as sent in the Authorization
// header) and returns the subject (user ID) and role carried in its claims.
// It rejects any typed token (refresh, email_verify, ...) since a plain
// access token never carries a "typ" claim — only those must ever be
// presented via the HttpOnly refresh cookie / verification link respectively.
func (as *AuthService) ValidateAccessToken(tokenStr string) (userID, role string, err error) {
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
		return "", "", fmt.Errorf("invalid access token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", errors.New("invalid access token claims")
	}

	if typ, _ := claims["typ"].(string); typ != "" {
		return "", "", errors.New("token is not a valid access token")
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", "", errors.New("access token missing subject")
	}
	roleClaim, _ := claims["role"].(string)

	return sub, roleClaim, nil
}

const emailVerifyTokenExpiry = 24 * time.Hour

// generateEmailVerifyToken mints a stateless, short-lived (24h) HS256 JWT
// carrying typ="email_verify". This is the token embedded in the
// verification email link — no server-side storage is needed, the JWT
// signature itself is the proof, mirroring the refresh token's design.
func (as *AuthService) generateEmailVerifyToken(userID string) (string, error) {
	secret := utils.GetEnv("JWT_SECRET", "default_secret")
	if secret == "" {
		return "", errors.New("JWT_SECRET environment variable is not set")
	}

	claims := jwt.MapClaims{
		"sub": userID,
		"typ": "email_verify",
		"exp": time.Now().Add(emailVerifyTokenExpiry).Unix(),
		"iat": time.Now().Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("failed to generate email verification token: %w", err)
	}

	return token, nil
}

// ValidateEmailVerifyToken validates an email-verification JWT and returns
// the subject (user ID) carried in its claims.
func (as *AuthService) ValidateEmailVerifyToken(tokenStr string) (userID string, err error) {
	secret := utils.GetEnv("JWT_SECRET", "default_secret")
	if secret == "" {
		return "", errors.New("JWT_SECRET environment variable is not set")
	}

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return "", fmt.Errorf("invalid verification token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid verification token claims")
	}

	if typ, _ := claims["typ"].(string); typ != "email_verify" {
		return "", errors.New("token is not an email verification token")
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", errors.New("verification token missing subject")
	}

	return sub, nil
}

// VerifyEmail consumes an email-verification token and marks the
// corresponding user as verified. Idempotent: verifying an already-verified
// account succeeds without error.
func (as *AuthService) VerifyEmail(token string) error {
	userID, err := as.ValidateEmailVerifyToken(token)
	if err != nil {
		return err
	}

	parsedID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user id in verification token: %w", err)
	}

	var user models.User
	if err := as.db.First(&user, "id = ?", parsedID).Error; err != nil {
		return fmt.Errorf("failed to look up user: %w", err)
	}

	if user.EmailVerifiedAt != nil {
		return nil
	}

	if err := as.db.Model(&user).Update("email_verified_at", time.Now()).Error; err != nil {
		return fmt.Errorf("failed to mark email verified: %w", err)
	}

	return nil
}

// ResendVerificationEmail re-sends the verification link for an unverified
// account. It never reports whether the address exists or is already
// verified — it silently no-ops in both cases — so the RPC can't be used to
// enumerate accounts. The per-email cooldown is applied before that lookup,
// so even the cooldown itself can't be used to infer existence.
func (as *AuthService) ResendVerificationEmail(ctx context.Context, email string) {
	as.resendCooldownsMu.Lock()
	last, onCooldown := as.resendCooldowns[email]
	if onCooldown && time.Since(last) < resendVerificationCooldown {
		as.resendCooldownsMu.Unlock()
		return
	}
	as.resendCooldowns[email] = time.Now()
	as.resendCooldownsMu.Unlock()

	var user models.User
	if err := as.db.First(&user, "email = ?", email).Error; err != nil {
		return
	}
	if user.EmailVerifiedAt != nil {
		return
	}

	as.sendVerificationEmail(ctx, user)
}
