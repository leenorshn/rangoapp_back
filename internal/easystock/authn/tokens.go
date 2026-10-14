package authn

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"rangoapp/internal/easystock/domain"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type accessClaims struct {
	SessionID string `json:"sid"`
	UserID    string `json:"uid"`
	Scope     string `json:"scope"`
	jwt.RegisteredClaims
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func codeMAC(secret, purpose, phone, challengeID, code string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(purpose))
	mac.Write([]byte{0})
	mac.Write([]byte(phone))
	mac.Write([]byte{0})
	mac.Write([]byte(challengeID))
	mac.Write([]byte{0})
	mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

func hashIP(secret, ip string) string {
	if ip == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("ip"))
	mac.Write([]byte{0})
	mac.Write([]byte(ip))
	return hex.EncodeToString(mac.Sum(nil))
}

func numericCode() (string, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	n := int(buf[0])<<16 | int(buf[1])<<8 | int(buf[2])
	return fmt.Sprintf("%06d", n%1000000), nil
}

func (s *Service) signAccess(sessionID, userID uuid.UUID, scope string, now time.Time) (string, error) {
	claims := accessClaims{
		SessionID: sessionID.String(),
		UserID:    userID.String(),
		Scope:     scope,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.cfg.Issuer,
			Audience:  jwt.ClaimStrings{s.cfg.Audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.AccessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   userID.String(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return "", domain.Internal("La session n'a pas pu être ouverte.")
	}
	return signed, nil
}

func (s *Service) parseAccess(raw string, now time.Time) (accessClaims, error) {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(s.cfg.Issuer),
		jwt.WithAudience(s.cfg.Audience),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	parsed, err := parser.ParseWithClaims(raw, &accessClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("algorithme refusé")
		}
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil {
		return accessClaims{}, domain.Unauthenticated("Session refusée.")
	}
	claims, ok := parsed.Claims.(*accessClaims)
	if !ok || !parsed.Valid || claims.SessionID == "" || claims.UserID == "" {
		return accessClaims{}, domain.Unauthenticated("Session refusée.")
	}
	return *claims, nil
}
