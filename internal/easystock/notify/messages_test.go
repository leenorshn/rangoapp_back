package notify

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestOTPText(t *testing.T) {
	signup := OTPText("signup", "123456", 5*time.Minute)
	want := "Voici le code OTP de EasyStock 123456. Utilisez-le avant 5 min et ne le partagez pas."
	if signup != want {
		t.Fatalf("signup %q", signup)
	}
	if utf8.RuneCountInString(signup) > uzaMaxMessageRunes {
		t.Fatalf("signup length %d", utf8.RuneCountInString(signup))
	}
	reset := OTPText("password_reset", "654321", 5*time.Minute)
	if !strings.Contains(reset, "654321") || !strings.Contains(reset, "mot de passe") {
		t.Fatalf("reset %q", reset)
	}
	if strings.Contains(reset, "OTP") {
		t.Fatalf("reset reused the signup wording: %q", reset)
	}
}

func TestReminderTextFits(t *testing.T) {
	long := strings.Repeat("Entreprise ", 30)
	for _, kind := range []string{"j1", "expired", "morning", "evening"} {
		text := ReminderText(kind, long)
		if n := utf8.RuneCountInString(text); n > uzaMaxMessageRunes {
			t.Fatalf("%s length %d: %s", kind, n, text)
		}
		if !strings.Contains(text, "EasyStock") {
			t.Fatalf("%s %q", kind, text)
		}
	}
	j1 := ReminderText("j1", "Beni")
	if j1 != "EasyStock: l'abonnement Beni expire demain. Renouvelez dans l'application." {
		t.Fatalf("j1 %q", j1)
	}
}
