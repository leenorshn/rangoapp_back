package authn

import (
	"unicode/utf8"

	"rangoapp/internal/easystock/domain"

	"golang.org/x/crypto/bcrypt"
)

// bcrypt rejects passwords longer than 72 bytes. Callers must fail instead of truncating.
const bcryptMaxBytes = 72

func hashPassword(password string, cost int) (string, error) {
	if err := validatePassword(password); err != nil {
		return "", err
	}
	if cost < bcrypt.MinCost {
		cost = bcrypt.DefaultCost
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", domain.Internal("Le mot de passe n'a pas pu être enregistré.")
	}
	return string(hashed), nil
}

func checkPassword(hash, password string) error {
	if len(password) > bcryptMaxBytes {
		return domain.Unauthenticated("Téléphone ou mot de passe incorrect.")
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return domain.Unauthenticated("Téléphone ou mot de passe incorrect.")
	}
	return nil
}

func validatePassword(password string) error {
	if utf8.RuneCountInString(password) < 8 {
		return domain.Invalid("password", "Le mot de passe doit contenir au moins 8 caractères.")
	}
	if len(password) > bcryptMaxBytes {
		return domain.Invalid("password", "Le mot de passe dépasse 72 octets et ne peut pas être accepté.")
	}
	return nil
}
