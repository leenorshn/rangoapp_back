package authn

import (
	"context"
	"errors"
	"fmt"
	"time"

	"rangoapp/internal/easystock/domain"

	"github.com/jackc/pgx/v5"
)

// RequestOTP sends a code for signup or password reset.
// The public result does not say whether an account exists.
func (s *Service) RequestOTP(ctx context.Context, purpose, rawPhone string, req Request) error {
	if purpose != PurposeSignup && purpose != PurposeReset {
		return domain.Invalid("purpose", "Cette vérification n'est pas disponible.")
	}
	number, err := s.normalize(rawPhone)
	if err != nil {
		return err
	}
	if err := s.allow(ctx, "otp:phone:"+number, s.cfg.OTPPerPhone, s.cfg.OTPWindow); err != nil {
		return err
	}
	if req.IP != "" {
		if err := s.allow(ctx, "otp:ip:"+req.IP, s.cfg.OTPPerIP, s.cfg.OTPWindow); err != nil {
			return err
		}
	}
	if purpose == PurposeReset {
		var exists bool
		err := s.pool.QueryRow(ctx, `SELECT true FROM users WHERE phone = $1 AND status = 'active'`, number).Scan(&exists)
		if errors.Is(err, pgx.ErrNoRows) || !exists {
			return nil
		}
		if err != nil {
			return domain.Internal("La demande n'a pas pu être enregistrée.")
		}
	}
	now := s.now()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var sentAt time.Time
	err = tx.QueryRow(ctx, `
		SELECT sent_at FROM phone_verification_challenges
		WHERE phone = $1 AND purpose = $2 AND consumed_at IS NULL
		ORDER BY sent_at DESC LIMIT 1
	`, number, purpose).Scan(&sentAt)
	if err == nil && now.Sub(sentAt) < s.cfg.OTPResend {
		return domain.FailedPrecondition("OTP_RESEND", "Attendez avant de demander un nouveau code.")
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return domain.Internal("La demande n'a pas pu être enregistrée.")
	}
	if _, err := tx.Exec(ctx, `
		UPDATE phone_verification_challenges
		SET consumed_at = $3
		WHERE phone = $1 AND purpose = $2 AND consumed_at IS NULL
	`, number, purpose, now); err != nil {
		return err
	}
	code, err := s.issueCode(number)
	if err != nil {
		return domain.Internal("La demande n'a pas pu être enregistrée.")
	}
	var challengeID string
	err = tx.QueryRow(ctx, `
		INSERT INTO phone_verification_challenges (
			purpose, phone, code_hash, expires_at, max_attempts, sent_at, ip_hash
		) VALUES ($1,$2,'pending',$3,$4,$5,$6)
		RETURNING id::text
	`, purpose, number, now.Add(s.cfg.OTPTTL), s.cfg.OTPMaxAttempts, now, hashIP(s.cfg.OTPSecret, req.IP)).Scan(&challengeID)
	if err != nil {
		return err
	}
	mac := codeMAC(s.cfg.OTPSecret, purpose, number, challengeID, code)
	if _, err := tx.Exec(ctx, `
		UPDATE phone_verification_challenges SET code_hash = $2 WHERE id = $1::uuid
	`, challengeID, mac); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if _, seeded := s.cfg.seededCode(number); seeded {
		return nil
	}
	if s.sms == nil {
		_ = s.consumeChallenge(ctx, challengeID, now)
		return domain.Internal("L'envoi du code est indisponible.")
	}
	body := fmt.Sprintf("EasyStock: code %s. Valable 5 min.", code)
	if _, err := s.sms.Send(ctx, number, body, "otp:"+challengeID); err != nil {
		_ = s.consumeChallenge(ctx, challengeID, now)
		return domain.Internal("Le code n'a pas pu être envoyé.")
	}
	return nil
}

func (s *Service) issueCode(number string) (string, error) {
	if code, ok := s.cfg.seededCode(number); ok {
		return code, nil
	}
	return numericCode()
}

func (s *Service) consumeChallenge(ctx context.Context, id string, now time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE phone_verification_challenges SET consumed_at = $2 WHERE id = $1::uuid AND consumed_at IS NULL
	`, id, now)
	return err
}

// VerifyOTP checks the code and returns a one-time grant bound to that phone and purpose.
func (s *Service) VerifyOTP(ctx context.Context, purpose, rawPhone, code string) (string, error) {
	if purpose != PurposeSignup && purpose != PurposeReset {
		return "", domain.Invalid("purpose", "Cette vérification n'est pas disponible.")
	}
	number, err := s.normalize(rawPhone)
	if err != nil {
		return "", err
	}
	if len(code) != 6 {
		return "", domain.Invalid("OTP_INVALID", "Code incorrect ou expiré.")
	}
	now := s.now()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var id string
	var stored string
	var expires time.Time
	var attempts, max int
	err = tx.QueryRow(ctx, `
		SELECT id::text, code_hash, expires_at, attempts, max_attempts
		FROM phone_verification_challenges
		WHERE phone = $1 AND purpose = $2 AND consumed_at IS NULL
		ORDER BY sent_at DESC
		LIMIT 1
		FOR UPDATE
	`, number, purpose).Scan(&id, &stored, &expires, &attempts, &max)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.Invalid("OTP_INVALID", "Code incorrect ou expiré.")
	}
	if err != nil {
		return "", domain.Internal("La vérification n'a pas pu être enregistrée.")
	}
	if !now.Before(expires) {
		if _, err := tx.Exec(ctx, `UPDATE phone_verification_challenges SET consumed_at = $2 WHERE id = $1::uuid`, id, now); err != nil {
			return "", err
		}
		if err := tx.Commit(ctx); err != nil {
			return "", err
		}
		return "", domain.FailedPrecondition("OTP_EXPIRED", "Ce code a expiré. Demandez-en un nouveau.")
	}
	if attempts >= max {
		return "", domain.FailedPrecondition("OTP_ATTEMPTS", "Ce code n'est plus valable. Demandez-en un nouveau.")
	}
	attempts++
	if _, err := tx.Exec(ctx, `
		UPDATE phone_verification_challenges SET attempts = $2 WHERE id = $1::uuid
	`, id, attempts); err != nil {
		return "", err
	}
	if !hmacEqual(stored, codeMAC(s.cfg.OTPSecret, purpose, number, id, code)) {
		if err := tx.Commit(ctx); err != nil {
			return "", err
		}
		if attempts >= max {
			return "", domain.FailedPrecondition("OTP_ATTEMPTS", "Ce code n'est plus valable. Demandez-en un nouveau.")
		}
		return "", domain.Invalid("OTP_INVALID", "Code incorrect ou expiré.")
	}
	if _, err := tx.Exec(ctx, `
		UPDATE phone_verification_challenges SET consumed_at = $2 WHERE id = $1::uuid
	`, id, now); err != nil {
		return "", err
	}
	if purpose == PurposeSignup {
		var exists bool
		err := tx.QueryRow(ctx, `SELECT true FROM users WHERE phone = $1`, number).Scan(&exists)
		if err == nil && exists {
			if err := tx.Commit(ctx); err != nil {
				return "", err
			}
			return "", domain.FailedPrecondition("ACCOUNT_EXISTS", "Ce numéro est déjà enregistré. Connectez-vous ou récupérez votre mot de passe.")
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return "", err
		}
	}
	token, err := randomToken()
	if err != nil {
		return "", domain.Internal("La vérification n'a pas pu être enregistrée.")
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO auth_grants (purpose, phone, token_hash, expires_at)
		VALUES ($1,$2,$3,$4)
	`, purpose, number, digest(token), now.Add(s.cfg.GrantTTL)); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return token, nil
}

func hmacEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range len(a) {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
