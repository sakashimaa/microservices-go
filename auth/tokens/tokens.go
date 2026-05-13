package tokens

import (
	rand2 "crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
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
	accessSecret    []byte
	refreshSecret   []byte
	accessTTL       time.Duration
	refreshTTL      time.Duration
	issuer          string
	now             func() time.Time
	privateKeyBytes []byte
}

func NewJWTManager(cfg config.AuthConfig) *Manager {
	return &Manager{
		accessSecret:    cfg.AccessSecret,
		refreshSecret:   cfg.RefreshSecret,
		accessTTL:       cfg.AccessTTL,
		refreshTTL:      cfg.RefreshTTL,
		issuer:          cfg.Issuer,
		now:             time.Now,
		privateKeyBytes: cfg.PrivateKeyBytes,
	}
}

type Claims struct {
	TokenType string   `json:"token_type"`
	Roles     []string `json:"roles,omitempty"`
	jwt.RegisteredClaims
}

func (m *Manager) GeneratePair(user domain.User) (domain.TokenPair, error) {
	accessToken, accessExpiresAt, err := m.generate(user.Id, TokenTypeAccess, m.accessTTL, m.privateKeyBytes, user.Roles)
	if err != nil {
		return domain.TokenPair{}, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, refreshExpiresAt, err := m.generate(user.Id, TokenTypeRefresh, m.refreshTTL, m.privateKeyBytes, user.Roles)
	if err != nil {
		return domain.TokenPair{}, fmt.Errorf("generate refresh token: %w", err)
	}

	return domain.TokenPair{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		AccessExpiresAt:  &accessExpiresAt,
		RefreshExpiresAt: &refreshExpiresAt,
	}, nil
}

func (m *Manager) generate(userID string, tokenType string, ttl time.Duration, privateKeyBytes []byte, roles []string) (string, time.Time, error) {
	now := m.now().UTC()
	expiresAt := now.Add(ttl)

	jti, err := randomHex(16)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generate jti: %w", err)
	}

	claims := Claims{
		TokenType: tokenType,
		Roles:     roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Subject:   userID,
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(privateKeyBytes)
	if err != nil {
		log.Fatalf("failed to get private RSA key from pem: %v", err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	signed, err := token.SignedString(privateKey)
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
