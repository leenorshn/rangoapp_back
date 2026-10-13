package commerce

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"

	"rangoapp/internal/easystock/access"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const invitationTTL = 7 * 24 * time.Hour

// Invitation is a pending or historical request to join the company.
// Token is set only when the invitation is created.
type Invitation struct {
	ID        uuid.UUID
	Email     string
	Role      string
	StoreID   *uuid.UUID
	Status    string
	ExpiresAt time.Time
	Token     string
}

// InviteInput chooses the role and, for a shop role, the boutique.
type InviteInput struct {
	Email   string
	Role    string
	StoreID *uuid.UUID
}

var inviteRoles = map[string]struct{}{
	"organization_admin": {},
	"store_manager":      {},
	"cashier":            {},
	"stockkeeper":        {},
	"accountant":         {},
}

// InviteMember records an invitation. The token is returned once and is the only way to accept.
func (s *Service) InviteMember(ctx context.Context, p tenant.Principal, in InviteInput) (Invitation, error) {
	var out Invitation
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		email, err := normalizeEmail(in.Email)
		if err != nil {
			return err
		}
		if _, ok := inviteRoles[in.Role]; !ok {
			return domain.Invalid("role", "Choisissez un rôle : administrateur, responsable, vendeur, magasinier ou comptable.")
		}
		if shopRole(in.Role) && in.StoreID == nil {
			return domain.Invalid("store", "Indiquez la boutique pour ce rôle.")
		}
		decision, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, in.StoreID, access.MembersManage)
		if err != nil {
			return err
		}
		if err := access.CheckRoleChange(decision.Role, p.UserID, uuid.Nil, "", in.Role); err != nil {
			return err
		}
		if access.IsOperational(in.Role) && !decision.AllowOperational {
			return domain.FailedPrecondition("quota_roles", "Cette offre ne comprend pas les rôles opérationnels. Passez à l'offre Pro.")
		}
		if in.StoreID != nil {
			var ok bool
			if err := tx.QueryRow(ctx, `SELECT true FROM stores WHERE id = $1 AND archived_at IS NULL`, *in.StoreID).Scan(&ok); err != nil || !ok {
				return domain.NotFound("Boutique introuvable.")
			}
		}
		var active, pending int
		if err := tx.QueryRow(ctx, `
			SELECT
				(SELECT count(*) FROM organization_memberships WHERE status = 'active'),
				(SELECT count(*) FROM invitations WHERE status = 'pending' AND expires_at > now())
		`).Scan(&active, &pending); err != nil {
			return err
		}
		if active+pending >= decision.MaxMembers {
			return domain.FailedPrecondition("quota_members", "Le nombre de collaborateurs autorisés par l'offre est atteint, invitations en attente comprises.")
		}
		plain, hash, err := newInvitationToken()
		if err != nil {
			return err
		}
		out = Invitation{
			ID: uuid.New(), Email: email, Role: in.Role, StoreID: in.StoreID,
			Status: "pending", ExpiresAt: time.Now().UTC().Add(invitationTTL), Token: plain,
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO invitations (
				id, company_id, email, org_role, store_id, status, token_hash, invited_by, expires_at
			) VALUES ($1,$2,$3,$4,$5,'pending',$6,$7,$8)
		`, out.ID, p.CompanyID, email, in.Role, in.StoreID, hash, p.UserID, out.ExpiresAt); err != nil {
			if strings.Contains(err.Error(), "invitations_pending_email") {
				return domain.AlreadyExists("Une invitation est déjà en attente pour cet e-mail.")
			}
			return err
		}
		return audit(ctx, tx, p, in.StoreID, "member.invite", "invitation", out.ID, map[string]string{"role": in.Role})
	})
	return out, err
}

// ListInvitations returns invitations of the company. A store manager sees only their boutique.
func (s *Service) ListInvitations(ctx context.Context, p tenant.Principal) ([]Invitation, error) {
	var out []Invitation
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		decision, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, nil, access.MembersManage)
		if err != nil {
			return err
		}
		query := `
			SELECT id, email, org_role, store_id, status, expires_at
			FROM invitations
			WHERE status = 'pending'
		`
		args := []any{}
		if decision.Role == "store_manager" {
			query += ` AND store_id IN (SELECT store_id FROM store_assignments WHERE user_id = $1)`
			args = append(args, p.UserID)
		}
		query += ` ORDER BY created_at DESC`
		rows, err := tx.Query(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item Invitation
			if err := rows.Scan(&item.ID, &item.Email, &item.Role, &item.StoreID, &item.Status, &item.ExpiresAt); err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	return out, err
}

// RevokeInvitation cancels a pending invitation.
func (s *Service) RevokeInvitation(ctx context.Context, p tenant.Principal, id uuid.UUID) error {
	return s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		decision, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, nil, access.MembersManage)
		if err != nil {
			return err
		}
		var status string
		var storeID *uuid.UUID
		err = tx.QueryRow(ctx, `
			SELECT status, store_id FROM invitations WHERE id = $1 FOR UPDATE
		`, id).Scan(&status, &storeID)
		if err != nil {
			return domain.NotFound("Invitation introuvable.")
		}
		if decision.Role == "store_manager" {
			if storeID == nil {
				return domain.PermissionDenied("invitation", "Vous ne pouvez annuler que les invitations de votre boutique.")
			}
			if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, storeID, access.MembersManage); err != nil {
				return err
			}
		}
		if status != "pending" {
			return domain.FailedPrecondition("invitation_status", "Seule une invitation en attente peut être annulée.")
		}
		if _, err := tx.Exec(ctx, `UPDATE invitations SET status = 'revoked' WHERE id = $1`, id); err != nil {
			return err
		}
		return audit(ctx, tx, p, storeID, "member.revoke_invite", "invitation", id, map[string]string{})
	})
}

// RecordMembershipAccepted writes the audit row after a token is accepted.
func (s *Service) RecordMembershipAccepted(ctx context.Context, p tenant.Principal, role string) error {
	return s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		var status string
		if err := tx.QueryRow(ctx, `
			SELECT status FROM organization_memberships WHERE user_id = $1
		`, p.UserID).Scan(&status); err != nil || status != "active" {
			return domain.PermissionDenied("membership", "Vous n'êtes pas membre actif de cette entreprise.")
		}
		return audit(ctx, tx, p, nil, "member.accept", "membership", p.UserID, map[string]string{"role": role})
	})
}

func shopRole(role string) bool {
	switch role {
	case "store_manager", "cashier", "stockkeeper":
		return true
	default:
		return false
	}
}

func normalizeEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) < 3 || len(value) > 200 || strings.ContainsAny(value, " \t") || !strings.Contains(value, "@") {
		return "", domain.Invalid("email", "Indiquez une adresse e-mail valide.")
	}
	return value, nil
}

func newInvitationToken() (plain, hash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	plain = base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(plain))
	return plain, hex.EncodeToString(sum[:]), nil
}
