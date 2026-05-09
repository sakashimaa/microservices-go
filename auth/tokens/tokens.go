package tokens

import (
	rand2 "crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sakashimaa/billing-microservice/auth/config"
	"github.com/sakashimaa/billing-microservice/auth/domain"
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

type TokenManager interface {
	GeneratePair(user domain.User) (domain.TokenPair, error)
}

type Manager struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
	issuer        string
	now           func() time.Time
}

func NewJWTManager(cfg config.AuthConfig) *Manager {
	return &Manager{
		accessSecret:  cfg.AccessSecret,
		refreshSecret: cfg.RefreshSecret,
		accessTTL:     cfg.AccessTTL,
		refreshTTL:    cfg.RefreshTTL,
		issuer:        cfg.Issuer,
		now:           time.Now,
	}
}

type Claims struct {
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

func (m *Manager) GeneratePair(user domain.User) (domain.TokenPair, error) {
	accessToken, accessExpiresAt, err := m.generate(user.Id, TokenTypeAccess, m.accessTTL, m.accessSecret)
	if err != nil {
		return domain.TokenPair{}, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, refreshExpiresAt, err := m.generate(user.Id, TokenTypeRefresh, m.refreshTTL, m.refreshSecret)
	if err != nil {
		return domain.TokenPair{}, fmt.Errorf("generate refresh token: %w", err)
	}

	return domain.TokenPair{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		AccessExpiresAt:  accessExpiresAt,
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

func (m *Manager) generate(userID string, tokenType string, ttl time.Duration, secret []byte) (string, time.Time, error) {
	now := m.now().UTC()
	expiresAt := now.Add(ttl)

	jti, err := randomHex(16)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generate jti: %w", err)
	}

	claims := Claims{
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Subject:   userID,
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString(secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}

	return signed, expiresAt, nil
}

func randomHex(bytesCount int) (string, error) {
	b := make([]byte, bytesCount)
	if _, err := rand2.Read(b); err != nil {
		return "", nil
	}

	return hex.EncodeToString(b), nil
}
