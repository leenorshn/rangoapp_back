package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashPassword(t *testing.T) {
	t.Run("Hash password successfully", func(t *testing.T) {
		password := "testPassword123"
		hash, err := HashPassword(password)
		
		require.NoError(t, err)
		assert.NotEmpty(t, hash)
		assert.NotEqual(t, password, hash) // Hash should be different from original
		assert.Len(t, hash, 60) // bcrypt hash is always 60 characters
	})

	t.Run("Different passwords produce different hashes", func(t *testing.T) {
		password1 := "password1"
		password2 := "password2"
		
		hash1, err1 := HashPassword(password1)
		require.NoError(t, err1)
		
		hash2, err2 := HashPassword(password2)
		require.NoError(t, err2)
		
		assert.NotEqual(t, hash1, hash2)
	})

	t.Run("Same password produces different hashes (salt)", func(t *testing.T) {
		password := "samePassword"
		
		hash1, err1 := HashPassword(password)
		require.NoError(t, err1)
		
		hash2, err2 := HashPassword(password)
		require.NoError(t, err2)
		
		// Hashes should be different due to salt
		assert.NotEqual(t, hash1, hash2)
	})

	t.Run("Empty password", func(t *testing.T) {
		hash, err := HashPassword("")
		require.NoError(t, err)
		assert.NotEmpty(t, hash)
	})

	t.Run("Long password", func(t *testing.T) {
		longPassword := "a" + string(make([]byte, 1000))
		hash, err := HashPassword(longPassword)
		require.NoError(t, err)
		assert.NotEmpty(t, hash)
	})
}

func TestComparePassword(t *testing.T) {
	t.Run("Correct password matches hash", func(t *testing.T) {
		password := "testPassword123"
		hash, err := HashPassword(password)
		require.NoError(t, err)
		
		result := ComparePassword(password, hash)
		assert.True(t, result)
	})

	t.Run("Incorrect password does not match hash", func(t *testing.T) {
		password := "testPassword123"
		wrongPassword := "wrongPassword"
		hash, err := HashPassword(password)
		require.NoError(t, err)
		
		result := ComparePassword(wrongPassword, hash)
		assert.False(t, result)
	})

	t.Run("Empty password with hash", func(t *testing.T) {
		hash, err := HashPassword("somePassword")
		require.NoError(t, err)
		
		result := ComparePassword("", hash)
		assert.False(t, result)
	})

	t.Run("Password with empty hash", func(t *testing.T) {
		result := ComparePassword("anyPassword", "")
		assert.False(t, result)
	})

	t.Run("Invalid hash format", func(t *testing.T) {
		invalidHash := "not-a-valid-bcrypt-hash"
		result := ComparePassword("anyPassword", invalidHash)
		assert.False(t, result)
	})

	t.Run("Case sensitive password", func(t *testing.T) {
		password := "TestPassword123"
		hash, err := HashPassword(password)
		require.NoError(t, err)
		
		// Different case should not match
		result := ComparePassword("testpassword123", hash)
		assert.False(t, result)
		
		// Same case should match
		result = ComparePassword("TestPassword123", hash)
		assert.True(t, result)
	})

	t.Run("Special characters in password", func(t *testing.T) {
		password := "P@ssw0rd!#$%^&*()"
		hash, err := HashPassword(password)
		require.NoError(t, err)
		
		result := ComparePassword(password, hash)
		assert.True(t, result)
	})

	t.Run("Unicode characters in password", func(t *testing.T) {
		password := "Pässwörd测试"
		hash, err := HashPassword(password)
		require.NoError(t, err)
		
		result := ComparePassword(password, hash)
		assert.True(t, result)
	})
}

func TestPasswordSecurity(t *testing.T) {
	t.Run("Hash uses bcrypt cost factor", func(t *testing.T) {
		password := "testPassword"
		hash, err := HashPassword(password)
		require.NoError(t, err)
		
		// bcrypt hash should start with $2a$ or $2b$ and have cost 10
		assert.Contains(t, hash, "$2a$10$")
	})

	t.Run("Password comparison is constant time", func(t *testing.T) {
		// This is a basic test - actual constant time verification would require
		// more sophisticated timing analysis
		password := "testPassword"
		hash, err := HashPassword(password)
		require.NoError(t, err)
		
		// Both correct and incorrect should take similar time
		_ = ComparePassword(password, hash)
		_ = ComparePassword("wrongPassword", hash)
		// If we get here without hanging, basic functionality works
	})
}





