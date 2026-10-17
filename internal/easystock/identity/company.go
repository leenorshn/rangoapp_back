package identity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/subscription"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CompanyInput is the commercial part of signup. The Clerk organization id
// comes from the verified session, never from this payload.
type CompanyInput struct {
	Name            string
	PrimaryCurrency string
	Timezone        string
}

// CompanyResult is the company the caller can continue onboarding with.
type CompanyResult struct {
	CompanyID       uuid.UUID
	UserID          uuid.UUID
	Role            string
	Name            string
	PrimaryCurrency string
	TrialEndsAt     time.Time
	Inserted        bool
	ClerkLinked     bool
}

// AcceptedInvite is a membership opened by an invitation token.
type AcceptedInvite struct {
	CompanyID uuid.UUID
	UserID    uuid.UUID
	Role      string
	StoreID   *uuid.UUID
	InviterID uuid.UUID
}

// EnsureUser projects the Clerk user. The display name is kept once a webhook has set it.
func (s *Service) EnsureUser(ctx context.Context, clerkUserID, displayName string) (uuid.UUID, error) {
	if clerkUserID == "" {
		return uuid.Nil, domain.Unauthenticated("Session refusée.")
	}
	if strings.TrimSpace(displayName) == "" {
		displayName = clerkUserID
	}
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (id, clerk_user_id, display_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (clerk_user_id) DO NOTHING
		RETURNING id
	`, uuid.New(), clerkUserID, displayName).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.pool.QueryRow(ctx, `SELECT id FROM users WHERE clerk_user_id = $1`, clerkUserID).Scan(&id)
	}
	return id, err
}

// ResolveSession maps a verified Clerk session to a local principal.
// A company header is accepted only for a company that is not linked to a Clerk
// organization yet. Once clerk_org_id is set, the organization in the token is required.
func (s *Service) ResolveSession(ctx context.Context, clerkUserID, clerkOrgID, companyHeader string) (tenant.Principal, error) {
	userID, err := s.EnsureUser(ctx, clerkUserID, clerkUserID)
	if err != nil {
		return tenant.Principal{}, err
	}
	if clerkOrgID != "" {
		principal, err := s.LookupMember(ctx, clerkUserID, clerkOrgID)
		if err == nil {
			return principal, nil
		}
		if de, ok := domain.As(err); ok && de.Code == domain.CodeUnauthenticated {
			return tenant.Principal{UserID: userID}, nil
		}
		return tenant.Principal{}, err
	}
	if strings.TrimSpace(companyHeader) == "" {
		return tenant.Principal{UserID: userID}, nil
	}
	companyID, err := uuid.Parse(companyHeader)
	if err != nil {
		return tenant.Principal{}, domain.Unauthenticated("Session refusée.")
	}
	var principal tenant.Principal
	err = s.pool.QueryRow(ctx, `
		SELECT u.id, c.id, m.org_role
		FROM users u
		JOIN organization_memberships m ON m.user_id = u.id AND m.status = 'active'
		JOIN companies c ON c.id = m.company_id
		WHERE u.clerk_user_id = $1 AND c.id = $2 AND c.clerk_org_id IS NULL
	`, clerkUserID, companyID).Scan(&principal.UserID, &principal.CompanyID, &principal.OrgRole)
	if errors.Is(err, pgx.ErrNoRows) {
		return tenant.Principal{}, domain.Unauthenticated("Session refusée.")
	}
	return principal, err
}

// CreateCompany opens a company, makes the caller the owner and starts the 14-day trial.
// The same Clerk user keeps a single company until it is linked to an organization.
// Calling it again returns that company.
func (s *Service) CreateCompany(ctx context.Context, userID uuid.UUID, clerkOrgID string, in CompanyInput) (CompanyResult, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.PrimaryCurrency = strings.ToUpper(strings.TrimSpace(in.PrimaryCurrency))
	in.Timezone = strings.TrimSpace(in.Timezone)
	if in.Name == "" || len(in.Name) > 160 {
		return CompanyResult{}, domain.Invalid("company", "Indiquez le nom de l'entreprise.")
	}
	if !currencyOK(in.PrimaryCurrency) {
		return CompanyResult{}, domain.Invalid("currency", "Indiquez une devise sur trois lettres, par exemple USD ou CDF.")
	}
	if in.Timezone == "" {
		in.Timezone = "Africa/Lubumbashi"
	}
	if len(in.Timezone) > 64 {
		return CompanyResult{}, domain.Invalid("timezone", "Le fuseau horaire est trop long.")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CompanyResult{}, err
	}
	defer tx.Rollback(ctx)
	var userOK bool
	if err := tx.QueryRow(ctx, `SELECT true FROM users WHERE id = $1`, userID).Scan(&userOK); err != nil || !userOK {
		return CompanyResult{}, domain.Unauthenticated("Session refusée.")
	}
	result := CompanyResult{UserID: userID, Name: in.Name, PrimaryCurrency: in.PrimaryCurrency}
	if clerkOrgID != "" {
		err = tx.QueryRow(ctx, `
			SELECT id, name, primary_currency FROM companies WHERE clerk_org_id = $1
		`, clerkOrgID).Scan(&result.CompanyID, &result.Name, &result.PrimaryCurrency)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `
				UPDATE companies
				SET clerk_org_id = $2
				WHERE id = (
					SELECT id FROM companies
					WHERE created_by = $1 AND clerk_org_id IS NULL
				)
				AND (SELECT count(*) FROM companies WHERE created_by = $1 AND clerk_org_id IS NULL) = 1
				RETURNING id, name, primary_currency
			`, userID, clerkOrgID).Scan(&result.CompanyID, &result.Name, &result.PrimaryCurrency)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `
				INSERT INTO companies (id, clerk_org_id, name, primary_currency, timezone, created_by)
				VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (clerk_org_id) DO NOTHING
				RETURNING id
			`, uuid.New(), clerkOrgID, in.Name, in.PrimaryCurrency, in.Timezone, userID).Scan(&result.CompanyID)
			if err == nil {
				result.Inserted = true
			} else if errors.Is(err, pgx.ErrNoRows) {
				err = tx.QueryRow(ctx, `
					SELECT id, name, primary_currency FROM companies WHERE clerk_org_id = $1
				`, clerkOrgID).Scan(&result.CompanyID, &result.Name, &result.PrimaryCurrency)
			}
		}
	} else {
		err = tx.QueryRow(ctx, `
			INSERT INTO companies (id, name, primary_currency, timezone, created_by)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (created_by) WHERE clerk_org_id IS NULL AND created_by IS NOT NULL DO NOTHING
			RETURNING id
		`, uuid.New(), in.Name, in.PrimaryCurrency, in.Timezone, userID).Scan(&result.CompanyID)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `
				SELECT id, name, primary_currency FROM companies
				WHERE created_by = $1 AND clerk_org_id IS NULL
			`, userID).Scan(&result.CompanyID, &result.Name, &result.PrimaryCurrency)
		} else if err == nil {
			result.Inserted = true
		}
	}
	if err != nil {
		return CompanyResult{}, err
	}
	if err := s.ensureOwner(ctx, tx, &result); err != nil {
		return CompanyResult{}, err
	}
	var trial *time.Time
	if err := tx.QueryRow(ctx, `
		SELECT trial_ends_at FROM subscription_snapshots WHERE company_id = $1
	`, result.CompanyID).Scan(&trial); err != nil {
		return CompanyResult{}, err
	}
	if trial == nil {
		return CompanyResult{}, domain.FailedPrecondition("subscription", "Aucun abonnement n'est enregistré pour cette entreprise.")
	}
	result.TrialEndsAt = *trial
	if err := tx.Commit(ctx); err != nil {
		return CompanyResult{}, err
	}
	return result, nil
}

func (s *Service) ensureOwner(ctx context.Context, tx pgx.Tx, result *CompanyResult) error {
	var role, status string
	err := tx.QueryRow(ctx, `
		SELECT org_role, status FROM organization_memberships
		WHERE company_id = $1 AND user_id = $2
	`, result.CompanyID, result.UserID).Scan(&role, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		var owners int
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FROM organization_memberships
			WHERE company_id = $1 AND org_role = 'owner' AND status = 'active'
		`, result.CompanyID).Scan(&owners); err != nil {
			return err
		}
		if owners > 0 {
			return domain.FailedPrecondition("company_owner", "Cette entreprise a déjà un propriétaire. Demandez une invitation.")
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO organization_memberships (id, company_id, user_id, org_role, status)
			VALUES ($1, $2, $3, 'owner', 'active')
		`, uuid.New(), result.CompanyID, result.UserID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO subscription_snapshots (company_id, plan_code, status, source, trial_ends_at)
			VALUES ($1, 'essentiel', 'trialing', 'trial', $2)
			ON CONFLICT (company_id) DO NOTHING
		`, result.CompanyID, time.Now().UTC().AddDate(0, 0, subscription.TrialDays))
		result.Role = "owner"
		return err
	}
	if err != nil {
		return err
	}
	if status != "active" {
		return domain.PermissionDenied("membership", "Votre accès à cette entreprise n'est pas actif.")
	}
	result.Role = role
	return nil
}

// SetCompanyDefaults updates the signup choices while the company still has no boutique.
// The caller must already have checked that no store exists.
func (s *Service) SetCompanyDefaults(ctx context.Context, userID, companyID uuid.UUID, name, currency, timezone string) error {
	name = strings.TrimSpace(name)
	currency = strings.ToUpper(strings.TrimSpace(currency))
	timezone = strings.TrimSpace(timezone)
	if name == "" || !currencyOK(currency) || timezone == "" || len(timezone) > 64 {
		return domain.Invalid("company", "Indiquez le nom, la devise principale et le fuseau horaire.")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var role string
	err = tx.QueryRow(ctx, `
		SELECT org_role FROM organization_memberships
		WHERE company_id = $1 AND user_id = $2 AND status = 'active'
	`, companyID, userID).Scan(&role)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) || role != "owner" {
		return domain.PermissionDenied("company", "Seul le propriétaire choisit la devise principale.")
	}
	if _, err := tx.Exec(ctx, `
		UPDATE companies
		SET name = $2, primary_currency = $3, timezone = $4
		WHERE id = $1
	`, companyID, name, currency, timezone); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CompanyProfile loads the company of an active member.
func (s *Service) CompanyProfile(ctx context.Context, userID, companyID uuid.UUID) (CompanyResult, error) {
	var result CompanyResult
	var trial *time.Time
	var clerkOrg *string
	err := s.pool.QueryRow(ctx, `
		SELECT c.id, c.name, c.primary_currency, m.org_role, c.clerk_org_id, s.trial_ends_at
		FROM companies c
		JOIN organization_memberships m ON m.company_id = c.id AND m.user_id = $2 AND m.status = 'active'
		LEFT JOIN subscription_snapshots s ON s.company_id = c.id
		WHERE c.id = $1
	`, companyID, userID).Scan(&result.CompanyID, &result.Name, &result.PrimaryCurrency, &result.Role, &clerkOrg, &trial)
	if errors.Is(err, pgx.ErrNoRows) {
		return CompanyResult{}, domain.PermissionDenied("membership", "Vous n'êtes pas membre actif de cette entreprise.")
	}
	if err != nil {
		return CompanyResult{}, err
	}
	result.UserID = userID
	if trial != nil {
		result.TrialEndsAt = *trial
	}
	result.ClerkLinked = clerkOrg != nil && *clerkOrg != ""
	return result, nil
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// AcceptInvitation opens a membership for the authenticated user who presents the token.
// The same user can present the token again. Someone else cannot reuse it.
func (s *Service) AcceptInvitation(ctx context.Context, userID uuid.UUID, token string) (AcceptedInvite, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return AcceptedInvite{}, domain.Invalid("invitation", "Indiquez le jeton d'invitation.")
	}
	sum := sha256Hex(token)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AcceptedInvite{}, err
	}
	defer tx.Rollback(ctx)
	var userOK bool
	if err := tx.QueryRow(ctx, `SELECT true FROM users WHERE id = $1`, userID).Scan(&userOK); err != nil || !userOK {
		return AcceptedInvite{}, domain.Unauthenticated("Session refusée.")
	}
	var (
		inviteID, companyID, inviter uuid.UUID
		storeID                      *uuid.UUID
		acceptedBy                   *uuid.UUID
		role, status                 string
		expires                      time.Time
	)
	err = tx.QueryRow(ctx, `
		SELECT id, company_id, org_role, store_id, status, invited_by, accepted_user_id, expires_at
		FROM invitations
		WHERE token_hash = $1
		FOR UPDATE
	`, sum).Scan(&inviteID, &companyID, &role, &storeID, &status, &inviter, &acceptedBy, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return AcceptedInvite{}, domain.NotFound("Invitation introuvable.")
	}
	if err != nil {
		return AcceptedInvite{}, err
	}
	out := AcceptedInvite{CompanyID: companyID, UserID: userID, Role: role, StoreID: storeID, InviterID: inviter}
	switch status {
	case "revoked":
		return AcceptedInvite{}, domain.FailedPrecondition("invitation_revoked", "Cette invitation a été annulée.")
	case "accepted":
		if acceptedBy != nil && *acceptedBy == userID {
			if err := tx.Commit(ctx); err != nil {
				return AcceptedInvite{}, err
			}
			return out, nil
		}
		return AcceptedInvite{}, domain.FailedPrecondition("invitation_used", "Cette invitation a déjà été utilisée.")
	}
	if time.Now().After(expires) {
		return AcceptedInvite{}, domain.FailedPrecondition("invitation_expired", "Cette invitation a expiré. Demandez-en une nouvelle.")
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO organization_memberships (id, company_id, user_id, org_role, status)
		VALUES ($1, $2, $3, $4, 'active')
		ON CONFLICT (company_id, user_id) DO UPDATE
		SET org_role = EXCLUDED.org_role, status = 'active'
		WHERE organization_memberships.status <> 'active'
		   OR organization_memberships.org_role = EXCLUDED.org_role
	`, uuid.New(), companyID, userID, role)
	if err != nil {
		if strings.Contains(err.Error(), "last owner") {
			return AcceptedInvite{}, domain.FailedPrecondition("last_owner", "Le dernier propriétaire ne peut pas être retiré.")
		}
		return AcceptedInvite{}, err
	}
	if tag.RowsAffected() == 0 {
		return AcceptedInvite{}, domain.FailedPrecondition("already_member", "Vous faites déjà partie de cette entreprise avec un autre rôle.")
	}
	if _, err := tx.Exec(ctx, `
		UPDATE invitations
		SET status = 'accepted', accepted_user_id = $2, accepted_at = now()
		WHERE id = $1
	`, inviteID, userID); err != nil {
		return AcceptedInvite{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AcceptedInvite{}, err
	}
	return out, nil
}

func linkOrganization(ctx context.Context, tx pgx.Tx, id, name, createdBy string, at time.Time) error {
	if name == "" {
		name = id
	}
	tag, err := tx.Exec(ctx, `UPDATE companies SET name = $2 WHERE clerk_org_id = $1`, id, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		return nil
	}
	if createdBy != "" {
		rows, err := tx.Query(ctx, `
			SELECT c.id
			FROM companies c
			JOIN organization_memberships m ON m.company_id = c.id AND m.org_role = 'owner' AND m.status = 'active'
			JOIN users u ON u.id = m.user_id AND u.clerk_user_id = $1
			WHERE c.clerk_org_id IS NULL
			FOR UPDATE OF c
		`, createdBy)
		if err != nil {
			return err
		}
		var ids []uuid.UUID
		for rows.Next() {
			var companyID uuid.UUID
			if err := rows.Scan(&companyID); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, companyID)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(ids) == 1 {
			tag, err = tx.Exec(ctx, `
				UPDATE companies SET clerk_org_id = $2, name = $3
				WHERE id = $1 AND clerk_org_id IS NULL
			`, ids[0], id, name)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 1 {
				return nil
			}
		}
	}
	var companyID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO companies (id, clerk_org_id, name)
		VALUES ($1, $2, $3)
		ON CONFLICT (clerk_org_id) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`, uuid.New(), id, name).Scan(&companyID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO subscription_snapshots (company_id, plan_code, status, source, trial_ends_at)
		VALUES ($1, 'essentiel', 'trialing', 'trial', $2)
		ON CONFLICT (company_id) DO NOTHING
	`, companyID, at.AddDate(0, 0, subscription.TrialDays))
	return err
}

func currencyOK(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, r := range value {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}
