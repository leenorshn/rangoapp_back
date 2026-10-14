package authn

import (
	"context"
	"errors"
	"strings"
	"time"

	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SessionResult is a new access token and, for mobile, the refresh token.
type SessionResult struct {
	AccessToken        string
	RefreshToken       string
	ExpiresAt          time.Time
	SessionID          uuid.UUID
	UserID             uuid.UUID
	CompanyID          uuid.UUID
	Role               string
	Scope              string
	MustChangePassword bool
	CSRFToken          string
}

// SessionInfo is safe to show to the session owner.
type SessionInfo struct {
	ID            uuid.UUID
	CompanyID     uuid.UUID
	Scope         string
	CreatedAt     time.Time
	LastRefreshAt time.Time
	ExpiresAt     time.Time
	Current       bool
	UserAgent     string
}

func (s *Service) openSession(ctx context.Context, tx pgx.Tx, userID, companyID uuid.UUID, scope string, req Request) (SessionResult, error) {
	now := s.now()
	sessionID := uuid.New()
	refresh, err := randomToken()
	if err != nil {
		return SessionResult{}, domain.Internal("La session n'a pas pu être ouverte.")
	}
	csrf, err := randomToken()
	if err != nil {
		return SessionResult{}, domain.Internal("La session n'a pas pu être ouverte.")
	}
	absolute := now.Add(s.cfg.AbsoluteTTL)
	var company any
	if companyID != uuid.Nil {
		company = companyID
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO sessions (
			id, user_id, company_id, scope, created_at, last_refresh_at, absolute_expires_at, user_agent, ip_hash
		) VALUES ($1,$2,$3,$4,$5,$5,$6,$7,$8)
	`, sessionID, userID, company, scope, now, absolute, clipAgent(req.UserAgent), hashIP(s.cfg.JWTSecret, req.IP)); err != nil {
		return SessionResult{}, err
	}
	family := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO refresh_tokens (session_id, family_id, token_hash, expires_at, created_at)
		VALUES ($1,$2,$3,$4,$5)
	`, sessionID, family, digest(refresh), absolute, now); err != nil {
		return SessionResult{}, err
	}
	access, err := s.signAccess(sessionID, userID, scope, now)
	if err != nil {
		return SessionResult{}, err
	}
	var role string
	if companyID != uuid.Nil {
		_ = tx.QueryRow(ctx, `
			SELECT org_role FROM organization_memberships
			WHERE company_id = $1 AND user_id = $2 AND status = 'active'
		`, companyID, userID).Scan(&role)
	}
	var must bool
	_ = tx.QueryRow(ctx, `SELECT must_change_password FROM users WHERE id = $1`, userID).Scan(&must)
	return SessionResult{
		AccessToken: access, RefreshToken: refresh, ExpiresAt: now.Add(s.cfg.AccessTTL),
		SessionID: sessionID, UserID: userID, CompanyID: companyID, Role: role, Scope: scope,
		MustChangePassword: must, CSRFToken: csrf,
	}, nil
}

// Principal loads the server session behind an access token.
// A still-unexpired JWT is refused when the session is revoked, idle or past its absolute limit.
func (s *Service) Principal(ctx context.Context, raw string) (tenant.Principal, error) {
	claims, err := s.parseAccess(strings.TrimSpace(raw), s.now())
	if err != nil {
		return tenant.Principal{}, err
	}
	sessionID, err := uuid.Parse(claims.SessionID)
	if err != nil {
		return tenant.Principal{}, domain.Unauthenticated("Session refusée.")
	}
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return tenant.Principal{}, domain.Unauthenticated("Session refusée.")
	}
	return s.loadPrincipal(ctx, sessionID, userID)
}

func (s *Service) loadPrincipal(ctx context.Context, sessionID, userID uuid.UUID) (tenant.Principal, error) {
	var p tenant.Principal
	var scope, userStatus string
	var company *uuid.UUID
	var created, lastRefresh, absolute time.Time
	var revoked *time.Time
	var must, admin bool
	var role, memberStatus *string
	err := s.pool.QueryRow(ctx, `
		SELECT s.user_id, s.company_id, s.scope, s.created_at, s.last_refresh_at, s.absolute_expires_at, s.revoked_at,
		       u.status, u.must_change_password, u.is_platform_admin, m.org_role, m.status
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		LEFT JOIN organization_memberships m ON m.user_id = s.user_id AND m.company_id = s.company_id
		WHERE s.id = $1 AND s.user_id = $2
	`, sessionID, userID).Scan(
		&p.UserID, &company, &scope, &created, &lastRefresh, &absolute, &revoked,
		&userStatus, &must, &admin, &role, &memberStatus,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return tenant.Principal{}, domain.Unauthenticated("Session refusée.")
	}
	if err != nil {
		return tenant.Principal{}, domain.Internal("La session n'a pas pu être vérifiée.")
	}
	now := s.now()
	if revoked != nil || userStatus != "active" || !now.Before(absolute) || !now.Before(lastRefresh.Add(s.cfg.IdleTTL)) {
		return tenant.Principal{}, domain.Unauthenticated("Session refusée.")
	}
	p.SessionID = sessionID
	p.Scope = scope
	p.MustChangePassword = must
	p.PlatformAdmin = admin
	if must {
		p.Scope = ScopePassword
	}
	if company != nil {
		p.CompanyID = *company
	}
	if role != nil && memberStatus != nil && *memberStatus == "active" {
		p.OrgRole = *role
	}
	return p, nil
}

// Refresh rotates the refresh token. Reuse of an old token revokes the whole family.
// Concurrent calls with the same token are treated as reuse: the client must serialize them.
func (s *Service) Refresh(ctx context.Context, refreshToken string, req Request) (SessionResult, error) {
	if refreshToken == "" {
		return SessionResult{}, domain.Unauthenticated("Session refusée.")
	}
	now := s.now()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SessionResult{}, err
	}
	defer tx.Rollback(ctx)
	var tokenID, sessionID, familyID uuid.UUID
	var used, revoked *time.Time
	var tokenExpires time.Time
	err = tx.QueryRow(ctx, `
		SELECT id, session_id, family_id, used_at, revoked_at, expires_at
		FROM refresh_tokens
		WHERE token_hash = $1
		FOR UPDATE
	`, digest(refreshToken)).Scan(&tokenID, &sessionID, &familyID, &used, &revoked, &tokenExpires)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionResult{}, domain.Unauthenticated("Session refusée.")
	}
	if err != nil {
		return SessionResult{}, domain.Internal("La session n'a pas pu être renouvelée.")
	}
	if used != nil || revoked != nil {
		if err := revokeFamily(ctx, tx, familyID, sessionID, now); err != nil {
			return SessionResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return SessionResult{}, err
		}
		return SessionResult{}, domain.Unauthenticated("Session refusée.")
	}
	var userID uuid.UUID
	var company *uuid.UUID
	var scope, userStatus string
	var lastRefresh, absolute time.Time
	var sessionRevoked *time.Time
	var must bool
	err = tx.QueryRow(ctx, `
		SELECT s.user_id, s.company_id, s.scope, s.last_refresh_at, s.absolute_expires_at, s.revoked_at,
		       u.status, u.must_change_password
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.id = $1
		FOR UPDATE
	`, sessionID).Scan(&userID, &company, &scope, &lastRefresh, &absolute, &sessionRevoked, &userStatus, &must)
	if err != nil {
		return SessionResult{}, domain.Unauthenticated("Session refusée.")
	}
	if sessionRevoked != nil || userStatus != "active" || !now.Before(absolute) || !now.Before(lastRefresh.Add(s.cfg.IdleTTL)) || !now.Before(tokenExpires) {
		_, _ = tx.Exec(ctx, `UPDATE sessions SET revoked_at = COALESCE(revoked_at, $2) WHERE id = $1`, sessionID, now)
		if err := tx.Commit(ctx); err != nil {
			return SessionResult{}, err
		}
		return SessionResult{}, domain.Unauthenticated("Session refusée.")
	}
	if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET used_at = $2 WHERE id = $1`, tokenID, now); err != nil {
		return SessionResult{}, err
	}
	next, err := randomToken()
	if err != nil {
		return SessionResult{}, domain.Internal("La session n'a pas pu être renouvelée.")
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO refresh_tokens (session_id, family_id, token_hash, expires_at, created_at)
		VALUES ($1,$2,$3,$4,$5)
	`, sessionID, familyID, digest(next), absolute, now); err != nil {
		return SessionResult{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE sessions
		SET last_refresh_at = $2, user_agent = $3, ip_hash = $4
		WHERE id = $1
	`, sessionID, now, clipAgent(req.UserAgent), hashIP(s.cfg.JWTSecret, req.IP)); err != nil {
		return SessionResult{}, err
	}
	if must {
		scope = ScopePassword
	}
	access, err := s.signAccess(sessionID, userID, scope, now)
	if err != nil {
		return SessionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SessionResult{}, err
	}
	var companyID uuid.UUID
	if company != nil {
		companyID = *company
	}
	csrf, _ := randomToken()
	return SessionResult{
		AccessToken: access, RefreshToken: next, ExpiresAt: now.Add(s.cfg.AccessTTL),
		SessionID: sessionID, UserID: userID, CompanyID: companyID, Scope: scope,
		MustChangePassword: must, CSRFToken: csrf,
	}, nil
}

func revokeFamily(ctx context.Context, tx pgx.Tx, familyID, sessionID uuid.UUID, now time.Time) error {
	if _, err := tx.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, $2) WHERE family_id = $1
	`, familyID, now); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		UPDATE sessions SET revoked_at = COALESCE(revoked_at, $2) WHERE id = $1
	`, sessionID, now)
	return err
}

func (s *Service) Logout(ctx context.Context, sessionID, userID uuid.UUID) error {
	now := s.now()
	tag, err := s.pool.Exec(ctx, `
		UPDATE sessions SET revoked_at = COALESCE(revoked_at, $3)
		WHERE id = $1 AND user_id = $2
	`, sessionID, userID, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.Unauthenticated("Session refusée.")
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, $2) WHERE session_id = $1
	`, sessionID, now)
	return err
}

func (s *Service) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	now := s.now()
	if _, err := s.pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, $2)
		WHERE session_id IN (SELECT id FROM sessions WHERE user_id = $1)
	`, userID, now); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE sessions SET revoked_at = COALESCE(revoked_at, $2) WHERE user_id = $1
	`, userID, now)
	return err
}

func (s *Service) Sessions(ctx context.Context, userID, current uuid.UUID) ([]SessionInfo, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, company_id, scope, created_at, last_refresh_at, absolute_expires_at, user_agent
		FROM sessions
		WHERE user_id = $1 AND revoked_at IS NULL
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SessionInfo
	for rows.Next() {
		var item SessionInfo
		var company *uuid.UUID
		var agent *string
		if err := rows.Scan(&item.ID, &company, &item.Scope, &item.CreatedAt, &item.LastRefreshAt, &item.ExpiresAt, &agent); err != nil {
			return nil, err
		}
		if company != nil {
			item.CompanyID = *company
		}
		if agent != nil {
			item.UserAgent = *agent
		}
		item.Current = item.ID == current
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) Revoke(ctx context.Context, userID, sessionID uuid.UUID) error {
	var owner uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT user_id FROM sessions WHERE id = $1`, sessionID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) || owner != userID {
		return domain.NotFound("Session introuvable.")
	}
	if err != nil {
		return err
	}
	return s.Logout(ctx, sessionID, userID)
}

// SessionFromRefresh resolves a refresh token without rotating it. Logout uses it.
func (s *Service) SessionFromRefresh(ctx context.Context, refreshToken string) (uuid.UUID, uuid.UUID, error) {
	var sessionID, userID uuid.UUID
	var used, revoked *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT t.session_id, s.user_id, t.used_at, t.revoked_at
		FROM refresh_tokens t
		JOIN sessions s ON s.id = t.session_id
		WHERE t.token_hash = $1 AND s.revoked_at IS NULL
	`, digest(refreshToken)).Scan(&sessionID, &userID, &used, &revoked)
	if err != nil || used != nil || revoked != nil {
		return uuid.Nil, uuid.Nil, domain.Unauthenticated("Session refusée.")
	}
	return sessionID, userID, nil
}
