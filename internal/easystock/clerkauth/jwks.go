package clerkauth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type jwksDocument struct {
	Keys []struct {
		Kty string `json:"kty"`
		Kid string `json:"kid"`
		Alg string `json:"alg"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

// FetchKeyfunc loads RSA keys from a JWKS URL. The caller caches the result.
func FetchKeyfunc(ctx context.Context, rawURL string, client *http.Client) (jwt.Keyfunc, error) {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks status %d", resp.StatusCode)
	}
	var doc jwksDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, err
	}
	keys := map[string]*rsa.PublicKey{}
	for _, key := range doc.Keys {
		if key.Kty != "RSA" || key.Kid == "" {
			continue
		}
		pub, err := rsaPublic(key.N, key.E)
		if err != nil {
			return nil, err
		}
		keys[key.Kid] = pub
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("jwks has no rsa keys")
	}
	return func(token *jwt.Token) (any, error) {
		kid, _ := token.Header["kid"].(string)
		pub := keys[kid]
		if pub == nil {
			return nil, fmt.Errorf("unknown signing key")
		}
		return pub, nil
	}, nil
}

func rsaPublic(n, e string) (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(n)
	if err != nil {
		return nil, err
	}
	eb, err := base64.RawURLEncoding.DecodeString(e)
	if err != nil {
		return nil, err
	}
	modulus := new(big.Int).SetBytes(nb)
	exponent := new(big.Int).SetBytes(eb)
	if !exponent.IsInt64() || exponent.Int64() <= 0 {
		return nil, fmt.Errorf("invalid exponent")
	}
	return &rsa.PublicKey{N: modulus, E: int(exponent.Int64())}, nil
}
