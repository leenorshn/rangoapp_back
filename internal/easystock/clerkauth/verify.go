package clerkauth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is the local projection of a verified Clerk session token.
type Claims struct {
	Subject         string
	OrgID           string
	OrgRole         string
	AuthorizedParty string
}

// Verify checks signature, issuer, expiry, and authorized party. It does not trust email.
func Verify(raw string, keyfn jwt.Keyfunc, issuer string, parties map[string]struct{}, now time.Time) (Claims, error) {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	token, err := parser.Parse(raw, keyfn)
	if err != nil || !token.Valid {
		return Claims{}, fmt.Errorf("session token rejected")
	}
	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return Claims{}, fmt.Errorf("session token rejected")
	}
	out := Claims{Subject: stringClaim(mapClaims, "sub"), AuthorizedParty: stringClaim(mapClaims, "azp")}
	if out.Subject == "" {
		return Claims{}, fmt.Errorf("session token rejected")
	}
	if len(parties) > 0 {
		if _, ok := parties[out.AuthorizedParty]; !ok {
			return Claims{}, fmt.Errorf("session token rejected")
		}
	}
	if org, ok := mapClaims["o"].(map[string]any); ok {
		out.OrgID = stringClaim(org, "id")
		out.OrgRole = stringClaim(org, "rol")
	}
	if out.OrgID == "" {
		out.OrgID = stringClaim(mapClaims, "org_id")
	}
	return out, nil
}

func stringClaim(claims map[string]any, key string) string {
	value, _ := claims[key].(string)
	return value
}
