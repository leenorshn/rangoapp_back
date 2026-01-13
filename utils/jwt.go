package utils

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
)

type JwtCustomClaim struct {
	ID            string   `json:"_id"`
	CompanyID    string   `json:"companyId"`
	Role          string   `json:"role"`
	StoreIDs      []string `json:"storeIds"`
	AssignedStoreID string `json:"assignedStoreId,omitempty"`
	jwt.StandardClaims
}

var jwtSecret = []byte(getJwtSecret())

func getJwtSecret() string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		// In production, this should be required
		// For development only, generate a warning
		fmt.Println("WARNING: JWT_SECRET not set, using default. This is insecure for production!")
		return "xzaako_secret_23_@_"
	}
	if len(secret) < 32 {
		fmt.Println("WARNING: JWT_SECRET should be at least 32 characters long for security")
	}
	return secret
}

func JwtGenerate(ctx context.Context, userID, companyID, role string, storeIDs []string, assignedStoreID string) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, &JwtCustomClaim{
		ID:              userID,
		CompanyID:       companyID,
		Role:            role,
		StoreIDs:        storeIDs,
		AssignedStoreID: assignedStoreID,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(JWTTokenExpiration).Unix(),
			IssuedAt:  time.Now().Unix(),
		},
	})

	token, err := t.SignedString(jwtSecret)
	if err != nil {
		return "", err
	}

	return token, nil
}

// JwtGenerateRefresh generates a refresh token with longer expiration (7 days)
func JwtGenerateRefresh(ctx context.Context, userID, companyID, role string, storeIDs []string, assignedStoreID string) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, &JwtCustomClaim{
		ID:              userID,
		CompanyID:       companyID,
		Role:            role,
		StoreIDs:        storeIDs,
		AssignedStoreID: assignedStoreID,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(JWTRefreshTokenExpiration).Unix(),
			IssuedAt:  time.Now().Unix(),
		},
	})

	token, err := t.SignedString(jwtSecret)
	if err != nil {
		return "", err
	}

	return token, nil
}

func JwtValidate(ctx context.Context, token string) (*jwt.Token, error) {
	return jwt.ParseWithClaims(token, &JwtCustomClaim{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("there's a problem with the signing method")
		}
		return jwtSecret, nil
	})
}
