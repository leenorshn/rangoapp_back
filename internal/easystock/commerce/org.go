package commerce

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"rangoapp/internal/easystock/access"
	"rangoapp/internal/easystock/catalog"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/phone"
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

// Store is a boutique the caller can work in, with its principal cash register.
type Store struct {
	ID           uuid.UUID
	Name         string
	Timezone     string
	RegisterID   uuid.UUID
	RegisterName string
}

func (s *Service) CreateStore(ctx context.Context, p tenant.Principal, name, timezone string) (Store, error) {
	var created Store
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, nil, access.StoresManage); err != nil {
			return err
		}
		name = strings.TrimSpace(name)
		timezone = strings.TrimSpace(timezone)
		if name == "" {
			return domain.Invalid("store", "Indiquez le nom de la boutique.")
		}
		if timezone == "" {
			timezone = "Africa/Lubumbashi"
		}
		if len(timezone) > 64 {
			return domain.Invalid("timezone", "Le fuseau horaire est trop long.")
		}
		if _, err := time.LoadLocation(timezone); err != nil {
			return domain.Invalid("timezone", "Le fuseau horaire n'est pas reconnu.")
		}
		id := uuid.New()
		registerID := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO stores (id, company_id, name, timezone) VALUES ($1,$2,$3,$4)
		`, id, p.CompanyID, name, timezone); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO cash_registers (id, company_id, store_id, name) VALUES ($1,$2,$3,'Caisse principale')
		`, registerID, p.CompanyID, id); err != nil {
			return err
		}
		created = Store{ID: id, Name: name, Timezone: timezone, RegisterID: registerID, RegisterName: "Caisse principale"}
		return audit(ctx, tx, p, &id, "store.create", "store", id, map[string]string{"name": name})
	})
	if err != nil {
		return Store{}, err
	}
	return created, nil
}

// Stores lists the boutiques of the caller's company. The owner and the
// organization admin see every open boutique. Everyone else sees only the
// boutiques they are assigned to.
func (s *Service) Stores(ctx context.Context, p tenant.Principal) ([]Store, error) {
	var out []Store
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		var role, status string
		err := tx.QueryRow(ctx, `
			SELECT org_role, status FROM organization_memberships WHERE user_id = $1
		`, p.UserID).Scan(&role, &status)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && status != "active") {
			return domain.PermissionDenied("membership", "Vous n'êtes pas membre actif de cette entreprise.")
		}
		if err != nil {
			return err
		}
		query := `
			SELECT s.id, s.name, s.timezone, r.id, r.name
			FROM stores s
			LEFT JOIN LATERAL (
				SELECT id, name
				FROM cash_registers
				WHERE store_id = s.id AND archived_at IS NULL
				ORDER BY created_at, id
				LIMIT 1
			) r ON true
			WHERE s.archived_at IS NULL
		`
		args := []any{}
		if role != "owner" && role != "organization_admin" {
			query += ` AND EXISTS (
				SELECT 1 FROM store_assignments a
				WHERE a.store_id = s.id AND a.user_id = $1
			)`
			args = append(args, p.UserID)
		}
		query += ` ORDER BY s.name, s.id`
		rows, err := tx.Query(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item Store
			var registerID *uuid.UUID
			var registerName *string
			if err := rows.Scan(&item.ID, &item.Name, &item.Timezone, &registerID, &registerName); err != nil {
				return err
			}
			if registerID != nil {
				item.RegisterID = *registerID
			}
			if registerName != nil {
				item.RegisterName = *registerName
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	return out, err
}

func (s *Service) CreateProduct(ctx context.Context, p tenant.Principal, in CreateProductInput) (ProductCreated, error) {
	product, err := s.CreateCatalogProduct(ctx, p, catalog.CreateInput{
		SKU: in.SKU, Name: in.Name, CategoryName: in.Category, BaseUnit: in.BaseUnit,
		Barcode: in.Barcode, AllowFractional: in.AllowFractional, AlertThreshold: in.AlertThreshold,
	})
	if err != nil {
		return ProductCreated{}, err
	}
	created := ProductCreated{ProductID: product.ID}
	for _, unit := range product.Units {
		if unit.IsBase {
			created.UnitID = unit.ID
		}
	}
	return created, nil
}

func (s *Service) AddProductUnit(ctx context.Context, p tenant.Principal, productID uuid.UUID, name string, factor decimal.Decimal) (uuid.UUID, error) {
	unit, err := s.CreateCatalogUnit(ctx, p, productID, catalog.UnitInput{Name: name, Factor: factor, HasFactor: true})
	if err != nil {
		return uuid.Nil, err
	}
	return unit.ID, nil
}

func (s *Service) SetPrice(ctx context.Context, p tenant.Principal, storeID, productID uuid.UUID, unitName, currency string, unitPrice decimal.Decimal) error {
	amount := unitPrice
	_, err := s.UpsertCatalogPrice(ctx, p, catalog.PriceInput{
		StoreID: storeID, ProductID: productID, UnitName: unitName, Currency: currency, Amount: &amount,
	})
	return err
}

type CustomerInput struct {
	StoreID       uuid.UUID
	Name          string
	Phone         string
	WhatsappPhone string
	Address       string
	Notes         string
	CreditLimit   *decimal.Decimal
}

// CustomerCard is a customer created during a sale. The phone numbers are E.164.
type CustomerCard struct {
	ID            uuid.UUID
	StoreID       uuid.UUID
	Name          string
	Phone         string
	WhatsappPhone string
	Address       string
	Notes         string
	CreatedAt     time.Time
}

func (s *Service) CreateCustomer(ctx context.Context, p tenant.Principal, in CustomerInput) (CustomerCard, error) {
	var out CustomerCard
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &in.StoreID, access.SalesCreate); err != nil {
			return err
		}
		card, err := insertCustomer(ctx, tx, p, in)
		if err != nil {
			return err
		}
		out = card
		return audit(ctx, tx, p, &in.StoreID, "customer.create", "customer", card.ID, map[string]string{"name": card.Name})
	})
	return out, err
}

func insertCustomer(ctx context.Context, tx pgx.Tx, p tenant.Principal, in CustomerInput) (CustomerCard, error) {
	name := strings.TrimSpace(in.Name)
	address := strings.TrimSpace(in.Address)
	notes := strings.TrimSpace(in.Notes)
	if name == "" || len(name) > 160 {
		return CustomerCard{}, domain.Invalid("customer", "Indiquez le nom complet du client.")
	}
	if address == "" || len(address) > 300 {
		return CustomerCard{}, domain.Invalid("address", "Indiquez l'adresse du client.")
	}
	if len(notes) > 500 {
		return CustomerCard{}, domain.Invalid("notes", "La note est trop longue.")
	}
	var storeOK bool
	if err := tx.QueryRow(ctx, `SELECT true FROM stores WHERE id = $1 AND archived_at IS NULL`, in.StoreID).Scan(&storeOK); err != nil || !storeOK {
		return CustomerCard{}, domain.NotFound("Boutique introuvable.")
	}
	normalized, err := normalizeContact(in.Phone, in.WhatsappPhone)
	if err != nil {
		return CustomerCard{}, err
	}
	numbers := uniqueStrings(normalized.phone, normalized.whatsapp)
	var taken bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM customers
			WHERE archived_at IS NULL
			  AND (phone = ANY($1::text[]) OR whatsapp_phone = ANY($1::text[]))
		)
	`, numbers).Scan(&taken); err != nil {
		return CustomerCard{}, err
	}
	if taken {
		return CustomerCard{}, domain.AlreadyExists("Un client de cette entreprise utilise déjà ce numéro.")
	}
	id := uuid.New()
	var limit any
	if in.CreditLimit != nil {
		limit = money.Num(*in.CreditLimit)
	}
	var createdAt time.Time
	if err := tx.QueryRow(ctx, `
		INSERT INTO customers (
			id, company_id, store_id, name, phone, whatsapp_phone, address, notes, credit_limit
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING created_at
	`, id, p.CompanyID, in.StoreID, name, blankNull(normalized.phone), blankNull(normalized.whatsapp), address, blankNull(notes), limit).Scan(&createdAt); err != nil {
		return CustomerCard{}, err
	}
	return CustomerCard{
		ID: id, StoreID: in.StoreID, Name: name, Phone: normalized.phone,
		WhatsappPhone: normalized.whatsapp, Address: address, Notes: notes, CreatedAt: createdAt,
	}, nil
}

type normalizedContact struct {
	phone    string
	whatsapp string
}

func normalizeContact(phoneRaw, whatsappRaw string) (normalizedContact, error) {
	phoneRaw = strings.TrimSpace(phoneRaw)
	whatsappRaw = strings.TrimSpace(whatsappRaw)
	if phoneRaw == "" && whatsappRaw == "" {
		return normalizedContact{}, domain.Invalid("phone", "Indiquez un numéro de téléphone ou WhatsApp.")
	}
	var out normalizedContact
	var err error
	if phoneRaw != "" {
		out.phone, err = phone.Normalize(phoneRaw, "CD")
		if err != nil {
			return normalizedContact{}, err
		}
	}
	if whatsappRaw != "" {
		out.whatsapp, err = phone.Normalize(whatsappRaw, "CD")
		if err != nil {
			return normalizedContact{}, domain.Invalid("whatsapp", "Indiquez un numéro WhatsApp valide.")
		}
	}
	return out, nil
}

func uniqueStrings(values ...string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
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
		var already bool
		_ = tx.QueryRow(ctx, `SELECT true FROM store_assignments WHERE user_id = $1 AND store_id = $2`, userID, storeID).Scan(&already)
		if already {
			return nil
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO store_assignments (company_id, store_id, user_id) VALUES ($1,$2,$3)
		`, p.CompanyID, storeID, userID); err != nil {
			return err
		}
		return audit(ctx, tx, p, &storeID, "member.assign", "store_assignment", userID, map[string]string{"storeId": storeID.String()})
	})
}
