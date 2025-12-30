package validators

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestValidateEmail(t *testing.T) {
	t.Run("Valid email", func(t *testing.T) {
		err := ValidateEmail("test@example.com")
		assert.NoError(t, err)
	})

	t.Run("Valid email with subdomain", func(t *testing.T) {
		err := ValidateEmail("user@mail.example.com")
		assert.NoError(t, err)
	})

	t.Run("Valid email with plus", func(t *testing.T) {
		err := ValidateEmail("user+tag@example.com")
		assert.NoError(t, err)
	})

	t.Run("Empty email (optional)", func(t *testing.T) {
		err := ValidateEmail("")
		assert.NoError(t, err) // Email is optional
	})

	t.Run("Invalid email - no @", func(t *testing.T) {
		err := ValidateEmail("invalidemail.com")
		assert.Error(t, err)
	})

	t.Run("Invalid email - no domain", func(t *testing.T) {
		err := ValidateEmail("user@")
		assert.Error(t, err)
	})

	t.Run("Invalid email - no local part", func(t *testing.T) {
		err := ValidateEmail("@example.com")
		assert.Error(t, err)
	})

	t.Run("Email too long", func(t *testing.T) {
		longEmail := "a" + string(make([]byte, 255))
		err := ValidateEmail(longEmail)
		assert.Error(t, err)
	})
}

func TestValidatePhone(t *testing.T) {
	t.Run("Valid international phone", func(t *testing.T) {
		err := ValidatePhone("+1234567890")
		assert.NoError(t, err)
	})

	t.Run("Valid local phone", func(t *testing.T) {
		err := ValidatePhone("1234567890")
		assert.NoError(t, err)
	})

	t.Run("Valid phone with country code", func(t *testing.T) {
		err := ValidatePhone("+243123456789")
		assert.NoError(t, err)
	})

	t.Run("Empty phone", func(t *testing.T) {
		err := ValidatePhone("")
		assert.Error(t, err)
	})

	t.Run("Phone too short", func(t *testing.T) {
		err := ValidatePhone("12345")
		assert.Error(t, err)
	})

	t.Run("Phone too long", func(t *testing.T) {
		err := ValidatePhone("123456789012345678901")
		assert.Error(t, err)
	})

	t.Run("Invalid phone format", func(t *testing.T) {
		err := ValidatePhone("abc123")
		assert.Error(t, err)
	})

	t.Run("Phone with spaces", func(t *testing.T) {
		err := ValidatePhone("+1 234 567 890")
		// Should fail because spaces are not in regex
		assert.Error(t, err)
	})
}

func TestValidatePassword(t *testing.T) {
	t.Run("Valid password", func(t *testing.T) {
		err := ValidatePassword("password123")
		assert.NoError(t, err)
	})

	t.Run("Valid password with special chars", func(t *testing.T) {
		err := ValidatePassword("P@ssw0rd!")
		assert.NoError(t, err)
	})

	t.Run("Valid password minimum length", func(t *testing.T) {
		err := ValidatePassword("12345678")
		assert.NoError(t, err)
	})

	t.Run("Empty password", func(t *testing.T) {
		err := ValidatePassword("")
		assert.Error(t, err)
	})

	t.Run("Password too short", func(t *testing.T) {
		err := ValidatePassword("short")
		assert.Error(t, err)
	})

	t.Run("Password too long", func(t *testing.T) {
		longPassword := "a" + string(make([]byte, 129))
		err := ValidatePassword(longPassword)
		assert.Error(t, err)
	})
}

func TestValidateString(t *testing.T) {
	t.Run("Valid string", func(t *testing.T) {
		err := ValidateString("test", "Field", true, 2, 100)
		assert.NoError(t, err)
	})

	t.Run("Valid string at minimum length", func(t *testing.T) {
		err := ValidateString("ab", "Field", true, 2, 100)
		assert.NoError(t, err)
	})

	t.Run("Valid string at maximum length", func(t *testing.T) {
		longStr := string(make([]byte, 100))
		err := ValidateString(longStr, "Field", true, 2, 100)
		assert.NoError(t, err)
	})

	t.Run("Required string empty", func(t *testing.T) {
		err := ValidateString("", "Field", true, 2, 100)
		assert.Error(t, err)
	})

	t.Run("Optional string empty", func(t *testing.T) {
		err := ValidateString("", "Field", false, 2, 100)
		assert.NoError(t, err)
	})

	t.Run("String too short", func(t *testing.T) {
		err := ValidateString("a", "Field", true, 2, 100)
		assert.Error(t, err)
	})

	t.Run("String too long", func(t *testing.T) {
		longStr := string(make([]byte, 101))
		err := ValidateString(longStr, "Field", true, 2, 100)
		assert.Error(t, err)
	})

	t.Run("String with whitespace trimmed", func(t *testing.T) {
		err := ValidateString("  test  ", "Field", true, 2, 100)
		// Should pass because trimmed length is valid
		assert.NoError(t, err)
	})

	t.Run("String with only whitespace", func(t *testing.T) {
		err := ValidateString("   ", "Field", true, 2, 100)
		assert.Error(t, err)
	})
}

func TestValidateFloat(t *testing.T) {
	t.Run("Valid float", func(t *testing.T) {
		err := ValidateFloat(10.5, "Field", true, 0, 100)
		assert.NoError(t, err)
	})

	t.Run("Valid float at minimum", func(t *testing.T) {
		err := ValidateFloat(0.0, "Field", true, 0, 100)
		assert.NoError(t, err)
	})

	t.Run("Valid float at maximum", func(t *testing.T) {
		err := ValidateFloat(100.0, "Field", true, 0, 100)
		assert.NoError(t, err)
	})

	t.Run("Required float is zero", func(t *testing.T) {
		err := ValidateFloat(0.0, "Field", true, 1, 100)
		assert.Error(t, err)
	})

	t.Run("Optional float is zero", func(t *testing.T) {
		err := ValidateFloat(0.0, "Field", false, 1, 100)
		// Should fail because 0 < min (1)
		assert.Error(t, err)
	})

	t.Run("Float below minimum", func(t *testing.T) {
		err := ValidateFloat(5.0, "Field", true, 10, 100)
		assert.Error(t, err)
	})

	t.Run("Float above maximum", func(t *testing.T) {
		err := ValidateFloat(150.0, "Field", true, 0, 100)
		assert.Error(t, err)
	})

	t.Run("Float with no maximum", func(t *testing.T) {
		err := ValidateFloat(1000.0, "Field", true, 0, 0)
		assert.NoError(t, err)
	})

	t.Run("Negative float", func(t *testing.T) {
		err := ValidateFloat(-10.0, "Field", true, 0, 100)
		assert.Error(t, err)
	})
}

func TestValidateInt(t *testing.T) {
	t.Run("Valid int", func(t *testing.T) {
		err := ValidateInt(10, "Field", true, 0, 100)
		assert.NoError(t, err)
	})

	t.Run("Valid int at minimum", func(t *testing.T) {
		err := ValidateInt(0, "Field", true, 0, 100)
		assert.NoError(t, err)
	})

	t.Run("Valid int at maximum", func(t *testing.T) {
		err := ValidateInt(100, "Field", true, 0, 100)
		assert.NoError(t, err)
	})

	t.Run("Required int is zero", func(t *testing.T) {
		err := ValidateInt(0, "Field", true, 1, 100)
		assert.Error(t, err)
	})

	t.Run("Int below minimum", func(t *testing.T) {
		err := ValidateInt(5, "Field", true, 10, 100)
		assert.Error(t, err)
	})

	t.Run("Int above maximum", func(t *testing.T) {
		err := ValidateInt(150, "Field", true, 0, 100)
		assert.Error(t, err)
	})

	t.Run("Int with no maximum", func(t *testing.T) {
		err := ValidateInt(1000, "Field", true, 0, 0)
		assert.NoError(t, err)
	})

	t.Run("Negative int", func(t *testing.T) {
		err := ValidateInt(-10, "Field", true, 0, 100)
		assert.Error(t, err)
	})
}

func TestValidateObjectID(t *testing.T) {
	t.Run("Valid ObjectID", func(t *testing.T) {
		validID := primitive.NewObjectID().Hex()
		err := ValidateObjectID(validID, "Field")
		assert.NoError(t, err)
	})

	t.Run("Empty ObjectID", func(t *testing.T) {
		err := ValidateObjectID("", "Field")
		assert.Error(t, err)
	})

	t.Run("Invalid ObjectID format", func(t *testing.T) {
		err := ValidateObjectID("invalid-id", "Field")
		assert.Error(t, err)
	})

	t.Run("ObjectID too short", func(t *testing.T) {
		err := ValidateObjectID("1234567890123456789012", "Field") // 23 chars, need 24
		assert.Error(t, err)
	})

	t.Run("ObjectID too long", func(t *testing.T) {
		err := ValidateObjectID("1234567890123456789012345", "Field") // 25 chars
		assert.Error(t, err)
	})

	t.Run("ObjectID with invalid characters", func(t *testing.T) {
		err := ValidateObjectID("12345678901234567890123g", "Field") // 'g' is invalid hex
		assert.Error(t, err)
	})
}

func TestValidateRole(t *testing.T) {
	t.Run("Valid Admin role", func(t *testing.T) {
		err := ValidateRole("Admin")
		assert.NoError(t, err)
	})

	t.Run("Valid User role", func(t *testing.T) {
		err := ValidateRole("User")
		assert.NoError(t, err)
	})

	t.Run("Invalid role", func(t *testing.T) {
		err := ValidateRole("InvalidRole")
		assert.Error(t, err)
	})

	t.Run("Empty role", func(t *testing.T) {
		err := ValidateRole("")
		assert.Error(t, err)
	})

	t.Run("Lowercase role", func(t *testing.T) {
		err := ValidateRole("admin")
		assert.Error(t, err)
	})
}

func TestValidateRapportType(t *testing.T) {
	t.Run("Valid entree type", func(t *testing.T) {
		err := ValidateRapportType("entree")
		assert.NoError(t, err)
	})

	t.Run("Valid sortie type", func(t *testing.T) {
		err := ValidateRapportType("sortie")
		assert.NoError(t, err)
	})

	t.Run("Valid uppercase type", func(t *testing.T) {
		err := ValidateRapportType("ENTREE")
		assert.NoError(t, err)
	})

	t.Run("Invalid type", func(t *testing.T) {
		err := ValidateRapportType("invalid")
		assert.Error(t, err)
	})

	t.Run("Empty type", func(t *testing.T) {
		err := ValidateRapportType("")
		assert.Error(t, err)
	})
}

func TestValidateCurrency(t *testing.T) {
	t.Run("Valid USD", func(t *testing.T) {
		err := ValidateCurrency("USD")
		assert.NoError(t, err)
	})

	t.Run("Valid EUR", func(t *testing.T) {
		err := ValidateCurrency("EUR")
		assert.NoError(t, err)
	})

	t.Run("Valid CDF", func(t *testing.T) {
		err := ValidateCurrency("CDF")
		assert.NoError(t, err)
	})

	t.Run("Valid lowercase currency", func(t *testing.T) {
		err := ValidateCurrency("usd")
		assert.NoError(t, err) // Should be converted to uppercase
	})

	t.Run("Invalid currency", func(t *testing.T) {
		err := ValidateCurrency("INVALID")
		assert.Error(t, err)
	})

	t.Run("Empty currency", func(t *testing.T) {
		err := ValidateCurrency("")
		assert.Error(t, err)
	})
}

func TestValidateDate(t *testing.T) {
	t.Run("Valid RFC3339 date", func(t *testing.T) {
		err := ValidateDate("2024-01-01T00:00:00Z", "Date")
		assert.NoError(t, err)
	})

	t.Run("Valid date only format", func(t *testing.T) {
		err := ValidateDate("2024-01-01", "Date")
		assert.NoError(t, err)
	})

	t.Run("Valid date with timezone", func(t *testing.T) {
		err := ValidateDate("2024-01-01T00:00:00+02:00", "Date")
		assert.NoError(t, err)
	})

	t.Run("Empty date", func(t *testing.T) {
		err := ValidateDate("", "Date")
		assert.Error(t, err)
	})

	t.Run("Invalid date format", func(t *testing.T) {
		err := ValidateDate("01-01-2024", "Date")
		assert.Error(t, err)
	})

	t.Run("Invalid date string", func(t *testing.T) {
		err := ValidateDate("not-a-date", "Date")
		assert.Error(t, err)
	})
}

func TestValidateProductPrices(t *testing.T) {
	t.Run("Valid prices - vente > achat", func(t *testing.T) {
		err := ValidateProductPrices(100.0, 50.0)
		assert.NoError(t, err)
	})

	t.Run("Valid prices - vente == achat", func(t *testing.T) {
		err := ValidateProductPrices(100.0, 100.0)
		assert.NoError(t, err)
	})

	t.Run("Invalid prices - vente < achat", func(t *testing.T) {
		err := ValidateProductPrices(50.0, 100.0)
		assert.Error(t, err)
	})

	t.Run("Zero prices", func(t *testing.T) {
		err := ValidateProductPrices(0.0, 0.0)
		assert.NoError(t, err) // Both zero is technically valid
	})
}

func TestValidateFactureProducts(t *testing.T) {
	t.Run("Valid products array", func(t *testing.T) {
		products := make([]interface{}, 5)
		err := ValidateFactureProducts(products)
		assert.NoError(t, err)
	})

	t.Run("Empty products array", func(t *testing.T) {
		products := []interface{}{}
		err := ValidateFactureProducts(products)
		assert.Error(t, err)
	})

	t.Run("Too many products", func(t *testing.T) {
		products := make([]interface{}, 101)
		err := ValidateFactureProducts(products)
		assert.Error(t, err)
	})

	t.Run("Exactly 100 products", func(t *testing.T) {
		products := make([]interface{}, 100)
		err := ValidateFactureProducts(products)
		assert.NoError(t, err)
	})
}

func TestSanitizeString(t *testing.T) {
	t.Run("Trim whitespace", func(t *testing.T) {
		result := SanitizeString("  test  ", 100)
		assert.Equal(t, "test", result)
	})

	t.Run("Limit length", func(t *testing.T) {
		longStr := "a" + string(make([]byte, 100))
		result := SanitizeString(longStr, 10)
		assert.Len(t, result, 10)
	})

	t.Run("No limit", func(t *testing.T) {
		str := "test string"
		result := SanitizeString(str, 0)
		assert.Equal(t, "test string", result)
	})
}

func TestSanitizeEmail(t *testing.T) {
	t.Run("Lowercase email", func(t *testing.T) {
		result := SanitizeEmail("TEST@EXAMPLE.COM")
		assert.Equal(t, "test@example.com", result)
	})

	t.Run("Trim and lowercase", func(t *testing.T) {
		result := SanitizeEmail("  TEST@EXAMPLE.COM  ")
		assert.Equal(t, "test@example.com", result)
	})

	t.Run("Limit length", func(t *testing.T) {
		longEmail := "a" + string(make([]byte, 300)) + "@example.com"
		result := SanitizeEmail(longEmail)
		assert.LessOrEqual(t, len(result), 255)
	})
}

func TestValidateNotEmpty(t *testing.T) {
	t.Run("Valid string slice", func(t *testing.T) {
		slice := []string{"item1", "item2"}
		err := ValidateNotEmpty(slice, "Field")
		assert.NoError(t, err)
	})

	t.Run("Empty string slice", func(t *testing.T) {
		slice := []string{}
		err := ValidateNotEmpty(slice, "Field")
		assert.Error(t, err)
	})

	t.Run("Valid interface slice", func(t *testing.T) {
		slice := []interface{}{"item1", "item2"}
		err := ValidateNotEmpty(slice, "Field")
		assert.NoError(t, err)
	})

	t.Run("Empty interface slice", func(t *testing.T) {
		slice := []interface{}{}
		err := ValidateNotEmpty(slice, "Field")
		assert.Error(t, err)
	})
}



