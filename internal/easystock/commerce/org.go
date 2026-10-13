package commerce

import (
	"context"
	"encoding/json"

	"rangoapp/internal/easystock/access"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

type CreateProductInput struct {
	SKU             string
	Name            string
	Category        string
	BaseUnit        string
	Barcode         string
	AllowFractional bool
	AlertThreshold  decimal.Decimal
}

type ProductCreated struct {
	ProductID uuid.UUID
	UnitID    uuid.UUID
}

func (s *Service) CreateStore(ctx context.Context, p tenant.Principal, name, timezone string) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		decision, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, nil, access.StoresManage)
		if err != nil {
			return err
		}
		if name == "" {
			return domain.Invalid("store", "Indiquez le nom de la boutique.")
		}
		if timezone == "" {
			timezone = "Africa/Lubumbashi"
		}
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM stores WHERE archived_at IS NULL`).Scan(&count); err != nil {
			return err
		}
		if count >= decision.MaxStores {
			return domain.FailedPrecondition("quota_stores", "Cette offre ne permet pas d'ajouter une boutique. Les boutiques existantes sont conservées.")
		}
		id = uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO stores (id, company_id, name, timezone) VALUES ($1,$2,$3,$4)
		`, id, p.CompanyID, name, timezone); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO cash_registers (id, company_id, store_id, name) VALUES ($1,$2,$3,'Caisse principale')
		`, uuid.New(), p.CompanyID, id); err != nil {
			return err
		}
		return audit(ctx, tx, p, &id, "store.create", "store", id, map[string]string{"name": name})
	})
	return id, err
}

func (s *Service) CreateProduct(ctx context.Context, p tenant.Principal, in CreateProductInput) (ProductCreated, error) {
	var created ProductCreated
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		decision, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, nil, access.CatalogManage)
		if err != nil {
			return err
		}
		if decision.Role == "store_manager" {
			var n int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM store_assignments WHERE user_id = $1`, p.UserID).Scan(&n); err != nil || n == 0 {
				return domain.PermissionDenied("store", "Affectez d'abord le responsable à une boutique.")
			}
		}
		if in.SKU == "" || in.Name == "" || in.BaseUnit == "" {
			return domain.Invalid("product", "Indiquez le nom, le SKU et l'unité de base.")
		}
		created.ProductID = uuid.New()
		created.UnitID = uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO products (
				id, company_id, sku, name, category, base_unit, barcode, allow_fractional, alert_threshold, status
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'active')
		`, created.ProductID, p.CompanyID, in.SKU, in.Name, blankNull(in.Category), in.BaseUnit, blankNull(in.Barcode), in.AllowFractional, money.Num(in.AlertThreshold)); err != nil {
			return domain.AlreadyExists("Ce SKU existe déjà.")
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO product_units (id, company_id, product_id, name, factor, is_base)
			VALUES ($1,$2,$3,$4,1,true)
		`, created.UnitID, p.CompanyID, created.ProductID, in.BaseUnit); err != nil {
			return err
		}
		return audit(ctx, tx, p, nil, "catalog.product", "product", created.ProductID, map[string]string{"sku": in.SKU})
	})
	return created, err
}

func (s *Service) AddProductUnit(ctx context.Context, p tenant.Principal, productID uuid.UUID, name string, factor decimal.Decimal) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, nil, access.CatalogManage); err != nil {
			return err
		}
		if name == "" || !factor.IsPositive() {
			return domain.Invalid("unit", "Indiquez le nom du conditionnement et un facteur supérieur à zéro.")
		}
		id = uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO product_units (id, company_id, product_id, name, factor, is_base)
			VALUES ($1,$2,$3,$4,$5,false)
		`, id, p.CompanyID, productID, name, money.Num(factor)); err != nil {
			return domain.AlreadyExists("Ce conditionnement existe déjà.")
		}
		return nil
	})
	return id, err
}

func (s *Service) SetPrice(ctx context.Context, p tenant.Principal, storeID, productID uuid.UUID, unitName, currency string, unitPrice decimal.Decimal) error {
	return s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &storeID, access.CatalogManage); err != nil {
			return err
		}
		if !validCurrency(currency) || unitPrice.IsNegative() {
			return domain.Invalid("price", "Indiquez une devise et un prix positif ou nul.")
		}
		var unitID uuid.UUID
		if err := tx.QueryRow(ctx, `
			SELECT id FROM product_units WHERE product_id = $1 AND name = $2
		`, productID, unitName).Scan(&unitID); err != nil {
			return domain.NotFound("Conditionnement introuvable.")
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO product_prices (id, company_id, store_id, product_id, product_unit_id, currency, unit_price)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (company_id, store_id, product_id, product_unit_id, currency)
			DO UPDATE SET unit_price = EXCLUDED.unit_price
		`, uuid.New(), p.CompanyID, storeID, productID, unitID, currency, money.Num(money.Round(unitPrice))); err != nil {
			return err
		}
		return audit(ctx, tx, p, &storeID, "catalog.price", "product", productID, map[string]string{"currency": currency, "unitPrice": unitPrice.String()})
	})
}

type CustomerInput struct {
	StoreID     uuid.UUID
	Name        string
	Phone       string
	Notes       string
	CreditLimit *decimal.Decimal
}

func (s *Service) CreateCustomer(ctx context.Context, p tenant.Principal, in CustomerInput) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &in.StoreID, access.SalesCreate); err != nil {
			return err
		}
		if in.Name == "" {
			return domain.Invalid("customer", "Indiquez le nom du client.")
		}
		id = uuid.New()
		var limit any
		if in.CreditLimit != nil {
			limit = money.Num(*in.CreditLimit)
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO customers (id, company_id, store_id, name, phone, notes, credit_limit)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
		`, id, p.CompanyID, in.StoreID, in.Name, blankNull(in.Phone), blankNull(in.Notes), limit)
		return err
	})
	return id, err
}

func (s *Service) SaveSaleDraft(ctx context.Context, p tenant.Principal, storeID uuid.UUID, clientDraftID string, payload json.RawMessage) error {
	return s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &storeID, access.SalesCreate); err != nil {
			return err
		}
		if clientDraftID == "" || len(payload) == 0 {
			return domain.Invalid("draft", "Le brouillon doit avoir un identifiant et un contenu.")
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO sale_drafts (id, company_id, store_id, client_draft_id, payload, created_by)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (company_id, client_draft_id)
			DO UPDATE SET payload = EXCLUDED.payload, status = 'open', store_id = EXCLUDED.store_id
		`, uuid.New(), p.CompanyID, storeID, clientDraftID, []byte(payload), p.UserID)
		return err
	})
}

func (s *Service) SetMemberRole(ctx context.Context, p tenant.Principal, userID uuid.UUID, role string) error {
	return s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		decision, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, nil, access.MembersManage)
		if err != nil {
			return err
		}
		var current, status string
		if err := tx.QueryRow(ctx, `
			SELECT org_role, status FROM organization_memberships WHERE user_id = $1 FOR UPDATE
		`, userID).Scan(&current, &status); err != nil {
			return domain.NotFound("Membre introuvable.")
		}
		if err := access.CheckRoleChange(decision.Role, p.UserID, userID, current, role); err != nil {
			return err
		}
		if access.IsOperational(role) && !decision.AllowOperational {
			return domain.FailedPrecondition("quota_roles", "Cette offre ne comprend pas les rôles opérationnels. Passez à l'offre Pro.")
		}
		if status != "active" {
			var active int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM organization_memberships WHERE status = 'active'`).Scan(&active); err != nil {
				return err
			}
			if active >= decision.MaxMembers {
				return domain.FailedPrecondition("quota_members", "Le nombre de collaborateurs actifs autorisés par l'offre est atteint.")
			}
		}
		if current == "owner" && (role != "owner" || status == "active") && role != "owner" {
			var others int
			if err := tx.QueryRow(ctx, `
				SELECT count(*) FROM organization_memberships
				WHERE org_role = 'owner' AND status = 'active' AND user_id <> $1
			`, userID).Scan(&others); err != nil {
				return err
			}
			if others == 0 {
				return domain.FailedPrecondition("last_owner", "Le dernier propriétaire ne peut pas être retiré.")
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE organization_memberships SET org_role = $2, status = 'active' WHERE user_id = $1
		`, userID, role); err != nil {
			return err
		}
		return audit(ctx, tx, p, nil, "member.role", "membership", userID, map[string]string{"role": role})
	})
}

func (s *Service) AssignStore(ctx context.Context, p tenant.Principal, userID, storeID uuid.UUID) error {
	return s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		decision, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, nil, access.MembersManage)
		if err != nil {
			return err
		}
		if decision.Role == "store_manager" {
			if userID == p.UserID {
				return domain.PermissionDenied("assignment", "Un responsable ne peut pas étendre ses propres affectations.")
			}
			var ok bool
			if err := tx.QueryRow(ctx, `
				SELECT true FROM store_assignments WHERE user_id = $1 AND store_id = $2
			`, p.UserID, storeID).Scan(&ok); err != nil || !ok {
				return domain.PermissionDenied("assignment", "Vous ne pouvez affecter du personnel que dans votre boutique.")
			}
		}
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM store_assignments WHERE user_id = $1`, userID).Scan(&n); err != nil {
			return err
		}
		var already bool
		_ = tx.QueryRow(ctx, `SELECT true FROM store_assignments WHERE user_id = $1 AND store_id = $2`, userID, storeID).Scan(&already)
		if already {
			return nil
		}
		if n >= 1 && !decision.AllowMultistore {
			return domain.FailedPrecondition("quota_multistore", "Cette offre ne permet pas d'affecter une personne à plusieurs boutiques.")
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO store_assignments (company_id, store_id, user_id) VALUES ($1,$2,$3)
		`, p.CompanyID, storeID, userID); err != nil {
			return err
		}
		return audit(ctx, tx, p, &storeID, "member.assign", "store_assignment", userID, map[string]string{"storeId": storeID.String()})
	})
}
