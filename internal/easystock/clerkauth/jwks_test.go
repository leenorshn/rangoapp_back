package clerkauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestFetchKeyfuncVerifiesToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{
		"keys": []map[string]string{{
			"kty": "RSA",
			"kid": "test-key",
			"alg": "RS256",
			"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()
	keyfn, err := FetchKeyfunc(context.Background(), server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	signed := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": "user_1", "iss": "https://clerk.example", "azp": "https://app.example",
		"exp": now.Add(time.Minute).Unix(), "org_id": "org_1",
	})
	signed.Header["kid"] = "test-key"
	raw, err := signed.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Verify(raw, keyfn, "https://clerk.example", map[string]struct{}{"https://app.example": {}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Subject != "user_1" || got.OrgID != "org_1" {
		t.Fatalf("%+v", got)
	}
}
