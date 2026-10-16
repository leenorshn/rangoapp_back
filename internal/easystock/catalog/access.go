package catalog

import (
	"context"

	"rangoapp/internal/easystock/access"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func requireMember(ctx context.Context, tx pgx.Tx, p tenant.Principal) (string, error) {
	var role, status string
	err := tx.QueryRow(ctx, `
		SELECT org_role, status FROM organization_memberships WHERE user_id = $1
	`, p.UserID).Scan(&role, &status)
	if err != nil || status != "active" {
		return "", domain.PermissionDenied("membership", "Vous n'êtes pas membre actif de cette entreprise.")
	}
	return role, nil
}

func requireRead(ctx context.Context, tx pgx.Tx, p tenant.Principal, storeID uuid.UUID) error {
	role, err := requireMember(ctx, tx, p)
	if err != nil {
		return err
	}
	if storeID == uuid.Nil || role == "owner" || role == "organization_admin" {
		return nil
	}
	var assigned bool
	if err := tx.QueryRow(ctx, `
		SELECT true FROM store_assignments WHERE user_id = $1 AND store_id = $2
	`, p.UserID, storeID).Scan(&assigned); err != nil || !assigned {
		return domain.PermissionDenied("store", "Vous n'êtes pas affecté à cette boutique.")
	}
	return nil
}

func requireManage(ctx context.Context, tx pgx.Tx, p tenant.Principal, storeID uuid.UUID) error {
	var scope *uuid.UUID
	if storeID != uuid.Nil {
		scope = &storeID
	}
	decision, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, scope, access.CatalogManage)
	if err != nil {
		return err
	}
	if storeID == uuid.Nil && decision.Role == "store_manager" {
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM store_assignments WHERE user_id = $1`, p.UserID).Scan(&n); err != nil || n == 0 {
			return domain.PermissionDenied("store", "Affectez d'abord le responsable à une boutique.")
		}
	}
	return nil
}

// CostVisible reports whether this caller may see purchase cost for the store.
// A refusal is not an error: the catalog stays usable and the cost stays absent.
func CostVisible(ctx context.Context, tx pgx.Tx, p tenant.Principal, storeID uuid.UUID) (bool, error) {
	if storeID == uuid.Nil {
		return false, nil
	}
	_, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &storeID, access.MarginsRead)
	if err == nil {
		return true, nil
	}
	if de, ok := domain.As(err); ok && (de.Code == domain.CodePermissionDenied || de.Code == domain.CodeFailedPrecondition) {
		return false, nil
	}
	return false, err
}
