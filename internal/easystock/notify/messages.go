package notify

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// OTPText is the authentication SMS.
// Signup uses the OTP wording. Password reset names the password change.
// The code stays in the message, with the validity and a warning not to share it.
func OTPText(purpose, code string, validFor time.Duration) string {
	minutes := int(validFor / time.Minute)
	if minutes < 1 {
		minutes = 1
	}
	if purpose == "password_reset" {
		return clampSMS(fmt.Sprintf(
			"Voici le code EasyStock pour votre mot de passe %s. Utilisez-le avant %d min et ne le partagez pas.",
			code, minutes,
		))
	}
	return clampSMS(fmt.Sprintf(
		"Voici le code OTP de EasyStock %s. Utilisez-le avant %d min et ne le partagez pas.",
		code, minutes,
	))
}

// ReminderText is one subscription SMS. kind is j1, expired, morning or evening.
func ReminderText(kind, company string) string {
	company = strings.TrimSpace(company)
	if company == "" {
		company = "votre entreprise"
	}
	switch kind {
	case "j1":
		return withCompany("EasyStock: l'abonnement ", company, " expire demain. Renouvelez dans l'application.")
	case "expired":
		return withCompany("EasyStock: l'abonnement ", company, " a expiré. Renouvelez pour continuer.")
	case "morning":
		return withCompany("EasyStock: rappel, l'abonnement ", company, " a expiré. Renouvelez aujourd'hui.")
	case "evening":
		return withCompany("EasyStock: l'abonnement ", company, " est toujours expiré. Ouvrez l'application pour payer.")
	default:
		return withCompany("EasyStock: l'abonnement ", company, " doit être renouvelé.")
	}
}

func withCompany(prefix, company, suffix string) string {
	text := prefix + company + suffix
	if utf8.RuneCountInString(text) <= uzaMaxMessageRunes {
		return text
	}
	budget := uzaMaxMessageRunes - utf8.RuneCountInString(prefix) - utf8.RuneCountInString(suffix) - 1
	runes := []rune(company)
	if budget < 1 || len(runes) <= budget {
		return clampSMS(text)
	}
	return prefix + string(runes[:budget]) + "…" + suffix
}

func clampSMS(text string) string {
	runes := []rune(text)
	if len(runes) <= uzaMaxMessageRunes {
		return text
	}
	return string(runes[:uzaMaxMessageRunes-1]) + "…"
}
