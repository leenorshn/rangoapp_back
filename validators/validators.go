package validators

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Common validation patterns
var (
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	phoneRegex   = regexp.MustCompile(`^\+?[1-9]\d{1,14}$|^[0-9]{8,15}$`) // International or local format
	passwordRegex = regexp.MustCompile(`^.{8,}$`)                        // At least 8 characters
)

// ValidateEmail validates an email address
func ValidateEmail(email string) error {
	if email == "" {
		return nil // Email is optional in some cases
	}
	if !emailRegex.MatchString(email) {
		return gqlerror.Errorf("Invalid email format")
	}
	if len(email) > 255 {
		return gqlerror.Errorf("Email is too long (max 255 characters)")
	}
	return nil
}

// ValidatePhone validates a phone number
func ValidatePhone(phone string) error {
	if phone == "" {
		return gqlerror.Errorf("Phone number is required")
	}
	phone = strings.TrimSpace(phone)
	if len(phone) < 8 || len(phone) > 20 {
		return gqlerror.Errorf("Phone number must be between 8 and 20 characters")
	}
	if !phoneRegex.MatchString(phone) {
		return gqlerror.Errorf("Invalid phone number format")
	}
	return nil
}

// ValidatePassword validates a password
func ValidatePassword(password string) error {
	if password == "" {
		return gqlerror.Errorf("Password is required")
	}
	if len(password) < 8 {
		return gqlerror.Errorf("Password must be at least 8 characters long")
	}
	if len(password) > 128 {
		return gqlerror.Errorf("Password is too long (max 128 characters)")
	}
	return nil
}

// ValidateString validates a string field
func ValidateString(value, fieldName string, required bool, minLen, maxLen int) error {
	if required && strings.TrimSpace(value) == "" {
		return gqlerror.Errorf("%s is required", fieldName)
	}
	if value != "" {
		trimmed := strings.TrimSpace(value)
		if len(trimmed) < minLen {
			return gqlerror.Errorf("%s must be at least %d characters long", fieldName, minLen)
		}
		if len(trimmed) > maxLen {
			return gqlerror.Errorf("%s must be at most %d characters long", fieldName, maxLen)
		}
	}
	return nil
}

// ValidateFloat validates a float value
func ValidateFloat(value float64, fieldName string, required bool, min, max float64) error {
	if required && value == 0 {
		return gqlerror.Errorf("%s is required", fieldName)
	}
	if value < min {
		return gqlerror.Errorf("%s must be at least %.2f", fieldName, min)
	}
	if max > 0 && value > max {
		return gqlerror.Errorf("%s must be at most %.2f", fieldName, max)
	}
	return nil
}

// ValidateInt validates an integer value
func ValidateInt(value int, fieldName string, required bool, min, max int) error {
	if required && value == 0 {
		return gqlerror.Errorf("%s is required", fieldName)
	}
	if value < min {
		return gqlerror.Errorf("%s must be at least %d", fieldName, min)
	}
	if max > 0 && value > max {
		return gqlerror.Errorf("%s must be at most %d", fieldName, max)
	}
	return nil
}

// ValidateObjectID validates a MongoDB ObjectID string
func ValidateObjectID(id, fieldName string) error {
	if id == "" {
		return gqlerror.Errorf("%s is required", fieldName)
	}
	if !primitive.IsValidObjectID(id) {
		return gqlerror.Errorf("Invalid %s format", fieldName)
	}
	return nil
}

// ValidateRole validates a user role
func ValidateRole(role string) error {
	validRoles := map[string]bool{
		"Admin": true,
		"User":  true,
	}
	if !validRoles[role] {
		return gqlerror.Errorf("Invalid role. Must be 'Admin' or 'User'")
	}
	return nil
}

// ValidateRapportType validates a rapport store type
func ValidateRapportType(rapportType string) error {
	validTypes := map[string]bool{
		"entree": true,
		"sortie": true,
	}
	if !validTypes[strings.ToLower(rapportType)] {
		return gqlerror.Errorf("Invalid rapport type. Must be 'entree' or 'sortie'")
	}
	return nil
}

// ValidateCurrency validates a currency code
// Only USD, EUR, and CDF are supported
func ValidateCurrency(currency string) error {
	if currency == "" {
		return gqlerror.Errorf("Currency is required")
	}
	validCurrencies := map[string]bool{
		"USD": true,
		"EUR": true,
		"CDF": true,
	}
	currency = strings.ToUpper(currency)
	if !validCurrencies[currency] {
		return gqlerror.Errorf("Invalid currency code. Supported: USD, EUR, CDF")
	}
	return nil
}

// ValidateDate validates a date string (RFC3339 format or YYYY-MM-DD format)
func ValidateDate(dateStr, fieldName string) error {
	if dateStr == "" {
		return gqlerror.Errorf("%s is required", fieldName)
	}
	// Accept both RFC3339 format (2024-01-01T00:00:00Z) and HTML date input format (2024-01-01)
	rfc3339Regex := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(Z|[+-]\d{2}:\d{2})$`)
	dateOnlyRegex := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	if !rfc3339Regex.MatchString(dateStr) && !dateOnlyRegex.MatchString(dateStr) {
		return gqlerror.Errorf("Invalid %s format. Expected RFC3339 format (e.g., 2024-01-01T00:00:00Z) or date format (e.g., 2024-01-01)", fieldName)
	}
	return nil
}

// ValidateProductPrices validates that priceVente >= priceAchat
func ValidateProductPrices(priceVente, priceAchat float64) error {
	if priceVente < priceAchat {
		return gqlerror.Errorf("Price de vente must be greater than or equal to price d'achat")
	}
	return nil
}

// ValidateFactureProducts validates facture products array
func ValidateFactureProducts(products []interface{}) error {
	if len(products) == 0 {
		return gqlerror.Errorf("At least one product is required")
	}
	if len(products) > 100 {
		return gqlerror.Errorf("Maximum 100 products allowed per facture")
	}
	return nil
}

// SanitizeString removes leading/trailing whitespace and limits length
func SanitizeString(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if maxLen > 0 && len(s) > maxLen {
		s = s[:maxLen]
	}
	return s
}

// SanitizeEmail sanitizes an email address
func SanitizeEmail(email string) string {
	email = strings.TrimSpace(strings.ToLower(email))
	if len(email) > 255 {
		email = email[:255]
	}
	return email
}

// ValidateNotEmpty validates that a slice is not empty
func ValidateNotEmpty(slice interface{}, fieldName string) error {
	switch v := slice.(type) {
	case []string:
		if len(v) == 0 {
			return gqlerror.Errorf("%s cannot be empty", fieldName)
		}
	case []interface{}:
		if len(v) == 0 {
			return gqlerror.Errorf("%s cannot be empty", fieldName)
		}
	default:
		return fmt.Errorf("unsupported type for ValidateNotEmpty")
	}
	return nil
}

