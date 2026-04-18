package auth

import (
	"time"
	"github.com/golang-jwt/jwt/v5"
	"haulagex/internal/models"
)

type Claims struct {
	UserID uint        `json:"uid"`
	Role   models.Role `json:"role"`
	jwt.RegisteredClaims
}

type JWTService struct {
	AccessSecret  string
	RefreshSecret string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
}

func NewJWTService(accessSecret, refreshSecret string, accessTTL, refreshTTL time.Duration) *JWTService {
	return &JWTService{
		AccessSecret:  accessSecret,
		RefreshSecret: refreshSecret,
		AccessTTL:     accessTTL,
		RefreshTTL:    refreshTTL,
	}
}

func (j *JWTService) GenerateAccessToken(uid uint, role models.Role) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID: uid,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(j.AccessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.AccessSecret))
}

func (j *JWTService) GenerateRefreshToken(uid uint, role models.Role) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID: uid,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(j.RefreshTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.RefreshSecret))
}

func (j *JWTService) ParseAccessToken(tokenStr string) (*Claims, error) {
	return parseToken(tokenStr, j.AccessSecret)
}

func (j *JWTService) ParseRefreshToken(tokenStr string) (*Claims, error) {
	return parseToken(tokenStr, j.RefreshSecret)
}

func parseToken(tokenStr, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrTokenInvalidClaims
}
