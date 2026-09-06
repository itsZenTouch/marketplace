package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var ErrInvalidToken = errors.New("invalid token")

type JWT struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewJWT(
	secret string,
	issuer string,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) *JWT {
	return &JWT{
		secret:     []byte(secret),
		issuer:     issuer,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

type AccessClaim struct {
	UserID string `json:"uid"`
	Type   string `json:"type"`

	jwt.RegisteredClaims
}

func (j *JWT) RefreshTTL() time.Duration {
	return j.refreshTTL
}

func (j *JWT) CreateAccessToken(userID uuid.UUID) (string, error) {
	now := time.Now()

	claims := AccessClaim{
		UserID: userID.String(),
		Type:   "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			Subject:   userID.String(),
			Issuer:    j.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.accessTTL)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(j.secret)
}

func (j *JWT) CreateRefreshToken(sessionID uuid.UUID) (
	string,
	string,
	error,
) {
	randomBytes := make([]byte, 32)

	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	randomPart := base64.RawURLEncoding.EncodeToString(randomBytes)

	token := sessionID.String() + "." + randomPart

	hash := sha256.Sum256([]byte(token))

	hashString := hex.EncodeToString(hash[:])

	return token, hashString, nil
}

func (j *JWT) ParseAccessToken(tokenString string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&AccessClaim{},
		func(t *jwt.Token) (any, error) {
			if t.Method != jwt.SigningMethodHS256 {
				return nil, ErrInvalidToken
			}
			return []byte(j.secret), nil
		},
	)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*AccessClaim)
	if !ok || !token.Valid {
		return uuid.Nil, ErrInvalidToken
	}

	if claims.Type != "access" {
		return uuid.Nil, ErrInvalidToken
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}

	return userID, nil
}

func HashRefreshToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func ParseRefreshSessionID(refreshToken string) (uuid.UUID, error) {
	parts := strings.SplitN(refreshToken, ".", 2)

	if len(parts) != 2 {
		return uuid.Nil, errors.New("invalid refresh token")
	}

	sessionID, err := uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, errors.New("invalid refresh token")
	}

	return sessionID, nil
}
