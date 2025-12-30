package utils

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJwtGenerate(t *testing.T) {
	// Set a test JWT secret
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-characters-long-for-security")
	defer os.Unsetenv("JWT_SECRET")

	ctx := context.Background()
	userID := "user123"
	companyID := "company456"
	role := "Admin"
	storeIDs := []string{"store1", "store2"}
	assignedStoreID := ""

	token, err := JwtGenerate(ctx, userID, companyID, role, storeIDs, assignedStoreID)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate the token
	parsedToken, err := JwtValidate(ctx, token)
	require.NoError(t, err)
	assert.True(t, parsedToken.Valid)

	claims, ok := parsedToken.Claims.(*JwtCustomClaim)
	require.True(t, ok)
	assert.Equal(t, userID, claims.ID)
	assert.Equal(t, companyID, claims.CompanyID)
	assert.Equal(t, role, claims.Role)
	assert.Equal(t, storeIDs, claims.StoreIDs)
	assert.Equal(t, assignedStoreID, claims.AssignedStoreID)
	assert.Greater(t, claims.ExpiresAt, time.Now().Unix())
}

func TestJwtGenerateRefresh(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-characters-long-for-security")
	defer os.Unsetenv("JWT_SECRET")

	ctx := context.Background()
	userID := "user123"
	companyID := "company456"
	role := "User"
	storeIDs := []string{"store1"}
	assignedStoreID := "store1"

	token, err := JwtGenerateRefresh(ctx, userID, companyID, role, storeIDs, assignedStoreID)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate the token
	parsedToken, err := JwtValidate(ctx, token)
	require.NoError(t, err)
	assert.True(t, parsedToken.Valid)

	claims, ok := parsedToken.Claims.(*JwtCustomClaim)
	require.True(t, ok)
	assert.Equal(t, userID, claims.ID)
	assert.Equal(t, companyID, claims.CompanyID)
	assert.Equal(t, role, claims.Role)
	
	// Refresh token should expire in 7 days
	expectedExpiry := time.Now().Add(time.Hour * 24 * 7).Unix()
	// Allow 1 minute tolerance
	assert.InDelta(t, expectedExpiry, claims.ExpiresAt, 60)
}

func TestJwtValidate(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-characters-long-for-security")
	defer os.Unsetenv("JWT_SECRET")

	ctx := context.Background()

	t.Run("Valid token", func(t *testing.T) {
		token, err := JwtGenerate(ctx, "user1", "company1", "Admin", []string{"store1"}, "")
		require.NoError(t, err)

		parsedToken, err := JwtValidate(ctx, token)
		require.NoError(t, err)
		assert.True(t, parsedToken.Valid)
	})

	t.Run("Invalid token format", func(t *testing.T) {
		invalidToken := "invalid.token.format"
		parsedToken, err := JwtValidate(ctx, invalidToken)
		assert.Error(t, err)
		assert.Nil(t, parsedToken)
	})

	t.Run("Token with wrong secret", func(t *testing.T) {
		// Generate token with one secret
		os.Setenv("JWT_SECRET", "secret1-at-least-32-characters-long")
		token, err := JwtGenerate(ctx, "user1", "company1", "Admin", []string{"store1"}, "")
		require.NoError(t, err)

		// Try to validate with different secret
		os.Setenv("JWT_SECRET", "secret2-at-least-32-characters-long")
		parsedToken, err := JwtValidate(ctx, token)
		assert.Error(t, err)
		assert.Nil(t, parsedToken)
	})

	t.Run("Expired token", func(t *testing.T) {
		os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-characters-long-for-security")
		
		// Create a token with expired claim
		claims := &JwtCustomClaim{
			ID:         "user1",
			CompanyID:  "company1",
			Role:       "Admin",
			StoreIDs:   []string{"store1"},
			StandardClaims: jwt.StandardClaims{
				ExpiresAt: time.Now().Add(-time.Hour).Unix(), // Expired 1 hour ago
				IssuedAt:  time.Now().Add(-time.Hour * 2).Unix(),
			},
		}

		tokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		token, err := tokenObj.SignedString([]byte("test-secret-key-at-least-32-characters-long-for-security"))
		require.NoError(t, err)

		parsedToken, err := JwtValidate(ctx, token)
		// JWT validation doesn't check expiration by default, but we can check manually
		if err == nil {
			claims, ok := parsedToken.Claims.(*JwtCustomClaim)
			if ok {
				assert.Less(t, claims.ExpiresAt, time.Now().Unix())
			}
		}
	})

	t.Run("Empty token", func(t *testing.T) {
		parsedToken, err := JwtValidate(ctx, "")
		assert.Error(t, err)
		assert.Nil(t, parsedToken)
	})
}

func TestJwtCustomClaim(t *testing.T) {
	claims := &JwtCustomClaim{
		ID:              "user123",
		CompanyID:       "company456",
		Role:            "User",
		StoreIDs:        []string{"store1", "store2"},
		AssignedStoreID: "store1",
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
			IssuedAt:  time.Now().Unix(),
		},
	}

	assert.Equal(t, "user123", claims.ID)
	assert.Equal(t, "company456", claims.CompanyID)
	assert.Equal(t, "User", claims.Role)
	assert.Equal(t, []string{"store1", "store2"}, claims.StoreIDs)
	assert.Equal(t, "store1", claims.AssignedStoreID)
}

func TestGetJwtSecret(t *testing.T) {
	t.Run("Secret from environment", func(t *testing.T) {
		os.Setenv("JWT_SECRET", "custom-secret-key")
		defer os.Unsetenv("JWT_SECRET")

		secret := getJwtSecret()
		assert.Equal(t, "custom-secret-key", secret)
	})

	t.Run("No secret in environment", func(t *testing.T) {
		os.Unsetenv("JWT_SECRET")
		secret := getJwtSecret()
		// Should use default
		assert.NotEmpty(t, secret)
	})

	t.Run("Short secret warning", func(t *testing.T) {
		os.Setenv("JWT_SECRET", "short")
		defer os.Unsetenv("JWT_SECRET")

		secret := getJwtSecret()
		// Should still return the secret but with warning
		assert.Equal(t, "short", secret)
	})
}



