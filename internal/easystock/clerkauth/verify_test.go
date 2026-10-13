package clerkauth

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestVerifySessionToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)
	signed := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": "user_1",
		"iss": "https://clerk.example",
		"azp": "https://app.example",
		"exp": now.Add(time.Minute).Unix(),
		"o":   map[string]any{"id": "org_1", "rol": "admin"},
	})
	raw, err := signed.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	keyfn := func(token *jwt.Token) (any, error) { return &key.PublicKey, nil }
	parties := map[string]struct{}{"https://app.example": {}}
	got, err := Verify(raw, keyfn, "https://clerk.example", parties, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Subject != "user_1" || got.OrgID != "org_1" || got.OrgRole != "admin" {
		t.Fatalf("%+v", got)
	}
	if _, err := Verify(raw, keyfn, "https://other.example", parties, now); err == nil {
		t.Fatal("wrong issuer was accepted")
	}
	if _, err := Verify(raw, keyfn, "https://clerk.example", parties, now.Add(2*time.Minute)); err == nil {
		t.Fatal("expired token was accepted")
	}
}
