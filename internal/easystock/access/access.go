package access

import (
	"context"

	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/money"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

const (
	SalesCreate     = "sales.create"
	SalesRefund     = "sales.refund"
	SalesDiscount   = "sales.discount"
	CreditAuthorize = "credit.authorize"
	StockReceive    = "stock.receive"
	StockAdjust     = "stock.adjust"
	DebtsCollect    = "debts.collect"
	CashOpen        = "cash.open"
	CashClose       = "cash.close"
	ExpensesCreate  = "expenses.create"
	ReportsRead     = "reports.read"
	ReportsExport   = "reports.export"
	MarginsRead     = "margins.read"
	MembersManage   = "members.manage"
	StoresManage    = "stores.manage"
	BillingManage   = "billing.manage"
	CatalogManage   = "catalog.manage"
	RatesManage     = "rates.manage"
)

// Decision is the result of a successful authorization check.
type Decision struct {
	Role               string
	UnlimitedDiscount  bool
	HasDiscountCeiling bool
	MaxDiscountRate    decimal.Decimal
	UnlimitedCredit    bool
	HasCreditCeiling   bool
	MaxCredit          decimal.Decimal
	PlanCode           string
	SubscriptionStatus string
	AllowExport        bool
	AllowOperational   bool
	AllowMultistore    bool
	MaxStores          int
	MaxMembers         int
}

var rolePerms = map[string]map[string]struct{}{
	"owner":              permSet(SalesCreate, SalesRefund, SalesDiscount, CreditAuthorize, StockReceive, StockAdjust, DebtsCollect, CashOpen, CashClose, ExpensesCreate, ReportsRead, ReportsExport, MarginsRead, MembersManage, StoresManage, BillingManage, CatalogManage, RatesManage),
	"organization_admin": permSet(SalesCreate, SalesRefund, SalesDiscount, CreditAuthorize, StockReceive, StockAdjust, DebtsCollect, CashOpen, CashClose, ExpensesCreate, ReportsRead, ReportsExport, MarginsRead, MembersManage, StoresManage, CatalogManage, RatesManage),
	"store_manager":      permSet(SalesCreate, SalesRefund, SalesDiscount, CreditAuthorize, StockReceive, StockAdjust, DebtsCollect, CashOpen, CashClose, ExpensesCreate, ReportsRead, ReportsExport, MembersManage, CatalogManage),
	"cashier":            permSet(SalesCreate, CashOpen, CashClose, DebtsCollect),
	"stockkeeper":        permSet(StockReceive, StockAdjust, ReportsRead),
	"accountant":         permSet(ReportsRead, ReportsExport, MarginsRead),
}

func permSet(perms ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(perms))
	for _, perm := range perms {
		out[perm] = struct{}{}
	}
	return out
}

func RoleAllows(role, perm string) bool {
	_, ok := rolePerms[role][perm]
	return ok
}

func IsOperational(role string) bool {
	switch role {
	case "store_manager", "cashier", "stockkeeper", "accountant":
		return true
	default:
		return false
	}
}

func IsCommercial(perm string) bool {
	switch perm {
	case SalesCreate, SalesRefund, SalesDiscount, CreditAuthorize, StockReceive, StockAdjust,
		DebtsCollect, CashOpen, CashClose, ExpensesCreate, CatalogManage, StoresManage:
		return true
	default:
		return false
	}
}

func CommercialOpen(status string) bool {
	switch status {
	case "trialing", "active", "grace":
		return true
	default:
		return false
	}
}

// Authorize checks membership, permission, store assignment and subscription.
// storeID is required for store-scoped permissions and ignored when nil.
// A commercial permission is refused when the subscription is expired.
func Authorize(ctx context.Context, tx pgx.Tx, companyID, userID uuid.UUID, storeID *uuid.UUID, perm string) (Decision, error) {
	return authorize(ctx, tx, companyID, userID, storeID, perm, true)
}

// AuthorizeRead is Authorize for a consultation. An expired subscription still
// allows reading the caller's own operations; it only blocks new writes.
func AuthorizeRead(ctx context.Context, tx pgx.Tx, companyID, userID uuid.UUID, storeID *uuid.UUID, perm string) (Decision, error) {
	return authorize(ctx, tx, companyID, userID, storeID, perm, false)
}

func authorize(ctx context.Context, tx pgx.Tx, companyID, userID uuid.UUID, storeID *uuid.UUID, perm string, gateCommercial bool) (Decision, error) {
	var decision Decision
	var status string
	err := tx.QueryRow(ctx, `
		SELECT org_role, status
		FROM organization_memberships
		WHERE user_id = $1
	`, userID).Scan(&decision.Role, &status)
	if err != nil {
		return Decision{}, domain.PermissionDenied("membership", "Vous n'êtes pas membre actif de cette entreprise.")
	}
	if status != "active" {
		return Decision{}, domain.PermissionDenied("membership", "Votre accès à cette entreprise n'est pas actif.")
	}

	if storeID != nil && decision.Role != "owner" && decision.Role != "organization_admin" {
		var assigned bool
		if err := tx.QueryRow(ctx, `
			SELECT true FROM store_assignments WHERE user_id = $1 AND store_id = $2
		`, userID, *storeID).Scan(&assigned); err != nil || !assigned {
			return Decision{}, domain.PermissionDenied("store", "Vous n'êtes pas affecté à cette boutique.")
		}
	}

	var deny bool
	var allow bool
	var storeRate, companyRate, storeCredit, companyCredit pgtype.Numeric
	rows, err := tx.Query(ctx, `
		SELECT effect, max_discount_rate, max_credit, store_id IS NOT NULL
		FROM permission_overrides
		WHERE user_id = $1 AND permission = $2
		  AND (store_id IS NULL OR store_id = $3)
	`, userID, perm, nullableUUID(storeID))
	if err != nil {
		return Decision{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var effect string
		var rate, credit pgtype.Numeric
		var scoped bool
		if err := rows.Scan(&effect, &rate, &credit, &scoped); err != nil {
			return Decision{}, err
		}
		if effect == "deny" {
			deny = true
		}
		if effect == "allow" {
			allow = true
		}
		if scoped {
			storeRate, storeCredit = rate, credit
		} else {
			companyRate, companyCredit = rate, credit
		}
	}
	if err := rows.Err(); err != nil {
		return Decision{}, err
	}
	if deny {
		return Decision{}, domain.PermissionDenied(perm, "Cette action ne vous est pas autorisée.")
	}
	if !RoleAllows(decision.Role, perm) && !allow {
		return Decision{}, domain.PermissionDenied(perm, "Cette action ne vous est pas autorisée.")
	}

	var defaultRate, defaultCredit pgtype.Numeric
	if err := tx.QueryRow(ctx, `
		SELECT default_max_discount_rate, default_max_credit FROM companies WHERE id = $1
	`, companyID).Scan(&defaultRate, &defaultCredit); err != nil {
		return Decision{}, domain.NotFound("Entreprise introuvable.")
	}

	decision.UnlimitedDiscount, decision.HasDiscountCeiling, decision.MaxDiscountRate = ceiling(
		decision.Role, storeRate, companyRate, defaultRate, true,
	)
	decision.UnlimitedCredit, decision.HasCreditCeiling, decision.MaxCredit = ceiling(
		decision.Role, storeCredit, companyCredit, defaultCredit, false,
	)

	err = tx.QueryRow(ctx, `
		SELECT s.status, s.plan_code, p.allow_export, p.allow_operational_roles, p.allow_multistore,
		       p.max_stores, p.max_active_members
		FROM subscription_snapshots s
		JOIN subscription_plans p ON p.code = s.plan_code
		WHERE s.company_id = $1
	`, companyID).Scan(
		&decision.SubscriptionStatus, &decision.PlanCode, &decision.AllowExport, &decision.AllowOperational,
		&decision.AllowMultistore, &decision.MaxStores, &decision.MaxMembers,
	)
	if err != nil {
		return Decision{}, domain.FailedPrecondition("subscription", "Aucun abonnement n'est enregistré pour cette entreprise.")
	}
	if gateCommercial && IsCommercial(perm) && !CommercialOpen(decision.SubscriptionStatus) {
		return Decision{}, domain.FailedPrecondition("subscription_expired", "L'abonnement ne permet plus d'enregistrer de nouvelles opérations. Vous pouvez encore consulter et exporter les données autorisées.")
	}
	if (perm == ReportsExport || perm == MarginsRead) && !decision.AllowExport {
		return Decision{}, domain.PermissionDenied("plan", "Cette offre ne comprend pas les exports ni les marges. Passez à l'offre Pro pour les activer.")
	}
	return decision, nil
}

func ceiling(role string, storeValue, companyValue, fallback pgtype.Numeric, rate bool) (unlimited, has bool, max decimal.Decimal) {
	chosen := firstValid(storeValue, companyValue)
	if chosen.Valid {
		parsed, err := money.FromNum(chosen)
		if err != nil {
			return false, true, decimal.Zero
		}
		return false, true, parsed
	}
	if role == "owner" || role == "organization_admin" {
		return true, false, decimal.Zero
	}
	if fallback.Valid {
		parsed, err := money.FromNum(fallback)
		if err != nil {
			return false, true, decimal.Zero
		}
		return false, true, parsed
	}
	if rate {
		_ = rate
	}
	return false, true, decimal.Zero
}

func firstValid(values ...pgtype.Numeric) pgtype.Numeric {
	for _, value := range values {
		if value.Valid {
			return value
		}
	}
	return pgtype.Numeric{}
}

func nullableUUID(id *uuid.UUID) uuid.UUID {
	if id == nil {
		return uuid.Nil
	}
	return *id
}

// ActiveMember reports the role of an active membership in the current tenant.
// Conversion and file download use it: they are not a commercial write.
func ActiveMember(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (string, error) {
	var role, status string
	err := tx.QueryRow(ctx, `
		SELECT org_role, status FROM organization_memberships WHERE user_id = $1
	`, userID).Scan(&role, &status)
	if err != nil {
		return "", domain.PermissionDenied("membership", "Vous n'êtes pas membre actif de cette entreprise.")
	}
	if status != "active" {
		return "", domain.PermissionDenied("membership", "Votre accès à cette entreprise n'est pas actif.")
	}
	return role, nil
}

// CheckRoleChange blocks self-promotion and elevation beyond the actor's role.
func CheckRoleChange(actorRole string, actorID, targetID uuid.UUID, currentRole, nextRole string) error {
	if actorID == targetID {
		return domain.PermissionDenied("self_role", "Vous ne pouvez pas modifier votre propre rôle.")
	}
	switch actorRole {
	case "owner":
		return nil
	case "organization_admin":
		if currentRole == "owner" || nextRole == "owner" {
			return domain.PermissionDenied("owner_role", "Seul le propriétaire peut transférer la propriété.")
		}
		return nil
	case "store_manager":
		if currentRole != "cashier" && currentRole != "stockkeeper" && currentRole != "" {
			return domain.PermissionDenied("elevation", "Un responsable ne peut pas modifier ce rôle.")
		}
		if nextRole != "cashier" && nextRole != "stockkeeper" {
			return domain.PermissionDenied("elevation", "Un responsable ne peut pas attribuer ce rôle.")
		}
		return nil
	default:
		return domain.PermissionDenied("members", "Vous ne pouvez pas gérer les rôles.")
	}
}
