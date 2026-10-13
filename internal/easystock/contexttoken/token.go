package contexttoken

import (
	"errors"
	"time"

	"rangoapp/internal/easystock/tenant"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const audience = "easystock-commerce"

type claims struct {
	UserID    string `json:"uid"`
	CompanyID string `json:"cid"`
	Actor     string `json:"act,omitempty"`
	RequestID string `json:"rid,omitempty"`
	jwt.RegisteredClaims
}

// Sign builds the short-lived context the API sends to Commerce.
// A company id outside this token is not trusted.
func Sign(secret string, p tenant.Principal, ttl time.Duration) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		UserID:    p.UserID.String(),
		CompanyID: p.CompanyID.String(),
		Actor:     p.ActorKind(),
		RequestID: p.RequestID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "easystock-api",
			Audience:  jwt.ClaimStrings{audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	})
	return token.SignedString([]byte(secret))
}

func Verify(secret, raw string) (tenant.Principal, error) {
	parsed, err := jwt.ParseWithClaims(raw, &claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	}, jwt.WithAudience(audience), jwt.WithIssuer("easystock-api"))
	if err != nil {
		return tenant.Principal{}, err
	}
	body, ok := parsed.Claims.(*claims)
	if !ok || !parsed.Valid {
		return tenant.Principal{}, errors.New("invalid context")
	}
	userID, err := uuid.Parse(body.UserID)
	if err != nil {
		return tenant.Principal{}, err
	}
	companyID, err := uuid.Parse(body.CompanyID)
	if err != nil {
		return tenant.Principal{}, err
	}
	return tenant.Principal{
		UserID:    userID,
		CompanyID: companyID,
		RequestID: body.RequestID,
		ActorType: body.Actor,
	}, nil
}
