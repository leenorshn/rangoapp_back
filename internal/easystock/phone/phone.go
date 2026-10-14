package phone

import (
	"strings"

	"rangoapp/internal/easystock/domain"

	"github.com/nyaruka/phonenumbers"
)

// Normalize converts a local or international number to E.164.
// region is the ISO 3166-1 alpha-2 country used when the number is not international.
// For the DRC, pass CD so that local numbers become +243.
func Normalize(raw, region string) (string, error) {
	raw = strings.TrimSpace(raw)
	region = strings.ToUpper(strings.TrimSpace(region))
	if raw == "" || region == "" {
		return "", domain.Invalid("phone", "Indiquez un numéro de téléphone valide.")
	}
	parsed, err := phonenumbers.Parse(raw, region)
	if err != nil || !phonenumbers.IsValidNumber(parsed) {
		return "", domain.Invalid("phone", "Indiquez un numéro de téléphone valide.")
	}
	return phonenumbers.Format(parsed, phonenumbers.E164), nil
}
