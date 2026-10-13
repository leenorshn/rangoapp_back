package identity

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

type Event struct {
	ID         string
	Type       string
	OccurredAt time.Time
	Data       json.RawMessage
}

func (s *Service) Apply(ctx context.Context, event Event) error {
	if event.ID == "" || event.Type == "" || event.OccurredAt.IsZero() {
		return domain.Invalid("webhook", "Événement incomplet.")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	sum := sha256.Sum256(event.Data)
	tag, err := tx.Exec(ctx, `
		INSERT INTO webhook_events (provider, event_id, event_type, occurred_at, payload_hash, status)
		VALUES ('clerk', $1, $2, $3, $4, 'applied')
		ON CONFLICT (provider, event_id) DO NOTHING
	`, event.ID, event.Type, event.OccurredAt, hex.EncodeToString(sum[:]))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return tx.Commit(ctx)
	}
	fresh, err := s.applyBody(ctx, tx, event)
	if err != nil {
		return err
	}
	if !fresh {
		if _, err := tx.Exec(ctx, `
			UPDATE webhook_events SET status = 'ignored_stale'
			WHERE provider = 'clerk' AND event_id = $1
		`, event.ID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Service) applyBody(ctx context.Context, tx pgx.Tx, event Event) (bool, error) {
	switch event.Type {
	case "user.created", "user.updated":
		var data struct {
			ID        string `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		}
		if err := json.Unmarshal(event.Data, &data); err != nil || data.ID == "" {
			return false, domain.Invalid("webhook", "Utilisateur illisible.")
		}
		ok, err := claim(ctx, tx, "user", data.ID, event.OccurredAt)
		if err != nil || !ok {
			return ok, err
		}
		name := strings.TrimSpace(data.FirstName + " " + data.LastName)
		if name == "" {
			name = data.ID
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO users (id, clerk_user_id, display_name)
			VALUES ($1, $2, $3)
			ON CONFLICT (clerk_user_id) DO UPDATE SET display_name = EXCLUDED.display_name
		`, uuid.New(), data.ID, name)
		return true, err
	case "organization.created", "organization.updated":
		var data struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			CreatedBy string `json:"created_by"`
		}
		if err := json.Unmarshal(event.Data, &data); err != nil || data.ID == "" {
			return false, domain.Invalid("webhook", "Organisation illisible.")
		}
		ok, err := claim(ctx, tx, "org", data.ID, event.OccurredAt)
		if err != nil || !ok {
			return ok, err
		}
		return true, linkOrganization(ctx, tx, data.ID, data.Name, data.CreatedBy, event.OccurredAt)
	case "organizationMembership.created", "organizationMembership.updated", "organizationMembership.deleted":
		var data struct {
			ID           string `json:"id"`
			Role         string `json:"role"`
			Organization struct {
				ID string `json:"id"`
			} `json:"organization"`
			PublicUser struct {
				UserID string `json:"user_id"`
			} `json:"public_user_data"`
		}
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return false, domain.Invalid("webhook", "Adhésion illisible.")
		}
		key := data.Organization.ID + ":" + data.PublicUser.UserID
		ok, err := claim(ctx, tx, "membership", key, event.OccurredAt)
		if err != nil || !ok {
			return ok, err
		}
		var companyID, userID uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT id FROM companies WHERE clerk_org_id = $1`, data.Organization.ID).Scan(&companyID); err != nil {
			return false, domain.FailedPrecondition("webhook_order", "L'organisation n'est pas encore projetée.")
		}
		if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE clerk_user_id = $1`, data.PublicUser.UserID).Scan(&userID); err != nil {
			return false, domain.FailedPrecondition("webhook_order", "L'utilisateur n'est pas encore projeté.")
		}
		role := mapRole(data.Role)
		status := "active"
		if event.Type == "organizationMembership.deleted" {
			status = "revoked"
		}
		var owners int
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FROM organization_memberships
			WHERE company_id = $1 AND org_role = 'owner' AND status = 'active'
		`, companyID).Scan(&owners); err != nil {
			return false, err
		}
		if owners == 0 && status == "active" {
			role = "owner"
		}
		var currentRole string
		err = tx.QueryRow(ctx, `
			SELECT org_role FROM organization_memberships
			WHERE company_id = $1 AND user_id = $2
		`, companyID, userID).Scan(&currentRole)
		if errors.Is(err, pgx.ErrNoRows) {
			currentRole = ""
			err = nil
		}
		if err != nil {
			return false, err
		}
		if currentRole == "owner" && (role != "owner" || status != "active") {
			var others int
			if err := tx.QueryRow(ctx, `
				SELECT count(*) FROM organization_memberships
				WHERE company_id = $1 AND org_role = 'owner' AND status = 'active' AND user_id <> $2
			`, companyID, userID).Scan(&others); err != nil {
				return false, err
			}
			if others == 0 {
				role = "owner"
				status = "active"
			}
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO organization_memberships (id, company_id, user_id, org_role, status, clerk_membership_id)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (company_id, user_id) DO UPDATE
			SET org_role = EXCLUDED.org_role, status = EXCLUDED.status, clerk_membership_id = EXCLUDED.clerk_membership_id
		`, uuid.New(), companyID, userID, role, status, blank(data.ID))
		return true, err
	case "billing.subscription.updated":
		var data struct {
			ID             string `json:"id"`
			OrganizationID string `json:"organization_id"`
			Status         string `json:"status"`
			PeriodEnd      int64  `json:"period_end"`
		}
		if err := json.Unmarshal(event.Data, &data); err != nil || data.OrganizationID == "" {
			return false, domain.Invalid("webhook", "Abonnement illisible.")
		}
		ok, err := claim(ctx, tx, "billing", data.OrganizationID, event.OccurredAt)
		if err != nil || !ok {
			return ok, err
		}
		var period *time.Time
		if data.PeriodEnd > 0 {
			t := time.Unix(data.PeriodEnd, 0).UTC()
			period = &t
		}
		_, err = tx.Exec(ctx, `
			UPDATE subscription_snapshots
			SET status = $2, source = 'clerk', external_id = $3, current_period_end = $4
			WHERE company_id = (SELECT id FROM companies WHERE clerk_org_id = $1)
		`, data.OrganizationID, mapBilling(data.Status), data.ID, period)
		return true, err
	default:
		_, err := tx.Exec(ctx, `
			UPDATE webhook_events SET status = 'ignored_unknown'
			WHERE provider = 'clerk' AND event_id = $1
		`, event.ID)
		return true, err
	}
}

func claim(ctx context.Context, tx pgx.Tx, entityType, entityID string, at time.Time) (bool, error) {
	var prev time.Time
	err := tx.QueryRow(ctx, `
		SELECT occurred_at FROM projection_versions
		WHERE entity_type = $1 AND entity_id = $2
		FOR UPDATE
	`, entityType, entityID).Scan(&prev)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = tx.Exec(ctx, `
			INSERT INTO projection_versions (entity_type, entity_id, occurred_at) VALUES ($1,$2,$3)
		`, entityType, entityID, at)
		return true, err
	}
	if err != nil {
		return false, err
	}
	if at.Before(prev) {
		return false, nil
	}
	_, err = tx.Exec(ctx, `
		UPDATE projection_versions SET occurred_at = $3
		WHERE entity_type = $1 AND entity_id = $2
	`, entityType, entityID, at)
	return true, err
}

func mapRole(role string) string {
	switch strings.TrimPrefix(role, "org:") {
	case "owner":
		return "owner"
	case "admin", "organization_admin":
		return "organization_admin"
	case "store_manager":
		return "store_manager"
	case "stockkeeper":
		return "stockkeeper"
	case "accountant":
		return "accountant"
	default:
		return "cashier"
	}
}

func mapBilling(status string) string {
	switch status {
	case "trialing":
		return "trialing"
	case "active":
		return "active"
	case "past_due":
		return "grace"
	case "canceled":
		return "canceled"
	default:
		return "expired"
	}
}

// ParseEvent reads a Clerk webhook body. The Svix id and timestamp come from headers.
func ParseEvent(id, timestamp string, body []byte) (Event, error) {
	var envelope struct {
		Type      string          `json:"type"`
		Timestamp int64           `json:"timestamp"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return Event{}, domain.Invalid("webhook", "Événement illisible.")
	}
	at := time.Time{}
	if ts, err := strconv.ParseInt(timestamp, 10, 64); err == nil {
		at = time.Unix(ts, 0).UTC()
	}
	if envelope.Timestamp > 1_000_000_000_000 {
		at = time.UnixMilli(envelope.Timestamp).UTC()
	} else if envelope.Timestamp > 0 {
		at = time.Unix(envelope.Timestamp, 0).UTC()
	}
	if envelope.Data == nil {
		envelope.Data = json.RawMessage(`{}`)
	}
	return Event{ID: id, Type: envelope.Type, OccurredAt: at, Data: envelope.Data}, nil
}

func blank(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// LookupMember resolves a verified Clerk session to the local user and company.
// Email is never used as a join key.
func (s *Service) LookupMember(ctx context.Context, clerkUserID, clerkOrgID string) (tenant.Principal, error) {
	if clerkUserID == "" || clerkOrgID == "" {
		return tenant.Principal{}, domain.Unauthenticated("Session refusée.")
	}
	var principal tenant.Principal
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, c.id, m.org_role
		FROM users u
		JOIN organization_memberships m ON m.user_id = u.id AND m.status = 'active'
		JOIN companies c ON c.id = m.company_id
		WHERE u.clerk_user_id = $1 AND c.clerk_org_id = $2
	`, clerkUserID, clerkOrgID).Scan(&principal.UserID, &principal.CompanyID, &principal.OrgRole)
	if errors.Is(err, pgx.ErrNoRows) {
		return tenant.Principal{}, domain.Unauthenticated("Session refusée.")
	}
	if err != nil {
		return tenant.Principal{}, err
	}
	return principal, nil
}

// VerifySvix checks a Clerk webhook signature against the raw body.
func VerifySvix(secret, id, timestamp, signature string, body []byte, now time.Time) error {
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return domain.Unauthenticated("Signature d'événement invalide.")
	}
	if now.Sub(time.Unix(ts, 0)) > 5*time.Minute || time.Unix(ts, 0).Sub(now) > 5*time.Minute {
		return domain.Unauthenticated("Signature d'événement expirée.")
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	if err != nil {
		return domain.Unauthenticated("Signature d'événement invalide.")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + timestamp + "."))
	mac.Write(body)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	for _, part := range strings.Fields(signature) {
		_, sig, ok := strings.Cut(part, ",")
		if ok && hmac.Equal([]byte(sig), []byte(expected)) {
			return nil
		}
	}
	return domain.Unauthenticated("Signature d'événement invalide.")
}
