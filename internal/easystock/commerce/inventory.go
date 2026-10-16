package commerce

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"rangoapp/internal/easystock/access"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

// StockInventory is one physical count for a store. Counting does not change stock.
type StockInventory struct {
	ID          uuid.UUID
	CompanyID   uuid.UUID
	StoreID     uuid.UUID
	CreatedBy   uuid.UUID
	Status      string
	Note        string
	StartedAt   time.Time
	CompletedAt *time.Time
	Lines       []StockInventoryLine
}

// StockInventoryLine is one packaging count plus the product rollup in the base unit.
type StockInventoryLine struct {
	ID                  uuid.UUID
	ProductID           uuid.UUID
	ProductName         string
	ProductUnitID       uuid.UUID
	UnitName            string
	Factor              string
	CountedQuantity     string
	BaseQuantity        string
	TheoreticalQuantity string
	CountedBaseTotal    string
	Variance            string
	VarianceValue       string
	VarianceCurrency    string
	CostIncomplete      bool
	Reason              string
}

type StartInventoryInput struct {
	StoreID uuid.UUID
	Note    string
}

type RecordCountInput struct {
	InventoryID     uuid.UUID
	ProductID       uuid.UUID
	ProductUnitID   uuid.UUID
	CountedQuantity decimal.Decimal
	Reason          string
}

type FinishInventoryInput struct {
	InventoryID uuid.UUID
}

func (s *Service) StartStockInventory(ctx context.Context, p tenant.Principal, in StartInventoryInput) (StockInventory, error) {
	var out StockInventory
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		note := strings.TrimSpace(in.Note)
		if len(note) > 500 {
			return domain.Invalid("note", "La note est trop longue.")
		}
		if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &in.StoreID, access.StockInventoryStart); err != nil {
			return err
		}
		var storeOK bool
		if err := tx.QueryRow(ctx, `SELECT true FROM stores WHERE id = $1 AND archived_at IS NULL`, in.StoreID).Scan(&storeOK); err != nil || !storeOK {
			return domain.NotFound("Boutique introuvable.")
		}
		id := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO inventory_sessions (id, company_id, store_id, status, created_by, note)
			VALUES ($1,$2,$3,'counting',$4,$5)
		`, id, p.CompanyID, in.StoreID, p.UserID, blankNull(note)); err != nil {
			return err
		}
		if err := audit(ctx, tx, p, &in.StoreID, "stock.inventory.start", "inventory", id, map[string]string{"note": note}); err != nil {
			return err
		}
		var err error
		out, err = assembleInventory(ctx, tx, p, id)
		return err
	})
	return out, err
}

func (s *Service) RecordInventoryCount(ctx context.Context, p tenant.Principal, in RecordCountInput) (StockInventoryLine, error) {
	var out StockInventoryLine
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		reason := strings.TrimSpace(in.Reason)
		if len(reason) > 500 {
			return domain.Invalid("reason", "Le motif est trop long.")
		}
		if in.CountedQuantity.IsNegative() {
			return domain.Invalid("quantity", "La quantité comptée ne peut pas être négative.")
		}
		session, err := lockInventory(ctx, tx, in.InventoryID)
		if err != nil {
			return err
		}
		if session.status != "counting" && session.status != "draft" {
			return inventoryClosed(session.status)
		}
		if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &session.storeID, access.StockInventoryCount); err != nil {
			return err
		}
		var productStatus string
		if err := tx.QueryRow(ctx, `SELECT status FROM products WHERE id = $1`, in.ProductID).Scan(&productStatus); err != nil {
			return domain.NotFound("Article introuvable.")
		}
		if productStatus != "active" {
			return domain.FailedPrecondition("product_archived", "Cet article est archivé.")
		}
		var factor pgtype.Numeric
		var active, unitFractional bool
		var unitName, dimension string
		err = tx.QueryRow(ctx, `
			SELECT name, factor, active, allow_fractional, dimension
			FROM product_units WHERE id = $1 AND product_id = $2
		`, in.ProductUnitID, in.ProductID).Scan(&unitName, &factor, &active, &unitFractional, &dimension)
		if err != nil {
			return domain.NotFound("Conditionnement introuvable.")
		}
		if !active {
			return domain.FailedPrecondition("unit_archived", "Ce conditionnement est archivé.")
		}
		if err := soldQuantityAllowed(unitFractional, in.CountedQuantity); err != nil {
			return err
		}
		factorDec, err := money.FromNum(factor)
		if err != nil {
			return err
		}
		conv, err := loadConversion(ctx, tx, in.ProductID)
		if err != nil {
			return err
		}
		base, _, err := convertQuantity(in.CountedQuantity, factorDec, dimension, conv)
		if err != nil {
			return err
		}
		var qty pgtype.Numeric
		err = tx.QueryRow(ctx, `
			SELECT quantity FROM stock_balances WHERE store_id = $1 AND product_id = $2
		`, session.storeID, in.ProductID).Scan(&qty)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		have := decimal.Zero
		if err == nil {
			have, err = money.FromNum(qty)
			if err != nil {
				return err
			}
		}
		if session.status == "draft" {
			if _, err := tx.Exec(ctx, `UPDATE inventory_sessions SET status = 'counting' WHERE id = $1`, in.InventoryID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO inventory_count_lines (
				id, company_id, inventory_id, product_id, product_unit_id,
				counted_quantity, factor, base_quantity, reason, balance_at_count, counted_at, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,now(),now())
			ON CONFLICT (inventory_id, product_unit_id) DO UPDATE SET
				counted_quantity = EXCLUDED.counted_quantity,
				factor = EXCLUDED.factor,
				base_quantity = EXCLUDED.base_quantity,
				reason = EXCLUDED.reason,
				balance_at_count = EXCLUDED.balance_at_count,
				counted_at = now(),
				updated_at = now()
		`, uuid.New(), p.CompanyID, in.InventoryID, in.ProductID, in.ProductUnitID, money.Num(in.CountedQuantity), money.Num(factorDec), money.Num(base), blankNull(reason), money.Num(have)); err != nil {
			return err
		}
		if err := audit(ctx, tx, p, &session.storeID, "stock.inventory.count", "inventory", in.InventoryID, map[string]string{"productId": in.ProductID.String()}); err != nil {
			return err
		}
		view, err := assembleInventory(ctx, tx, p, in.InventoryID)
		if err != nil {
			return err
		}
		for _, line := range view.Lines {
			if line.ProductUnitID == in.ProductUnitID {
				out = line
				return nil
			}
		}
		return domain.Internal("Le comptage n'a pas pu être relu.")
	})
	return out, err
}

func (s *Service) CompleteStockInventory(ctx context.Context, p tenant.Principal, in FinishInventoryInput) (StockInventory, error) {
	var out StockInventory
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		session, err := lockInventory(ctx, tx, in.InventoryID)
		if err != nil {
			return err
		}
		if session.status == "completed" {
			return domain.FailedPrecondition("inventory_completed", "Cet inventaire est déjà validé.")
		}
		if session.status != "counting" && session.status != "draft" {
			return inventoryClosed(session.status)
		}
		if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &session.storeID, access.StockInventoryValidate); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			SELECT l.product_id, l.product_unit_id, l.counted_quantity, l.factor, l.base_quantity,
			       l.balance_at_count, l.counted_at, l.reason, u.factor, p.base_unit, p.status, p.name
			FROM inventory_count_lines l
			JOIN product_units u ON u.id = l.product_unit_id
			JOIN products p ON p.id = l.product_id
			WHERE l.inventory_id = $1
			ORDER BY l.product_id, l.product_unit_id
		`, in.InventoryID)
		if err != nil {
			return err
		}
		type countLine struct {
			product, unit                  uuid.UUID
			counted, factor, base, atCount decimal.Decimal
			currentFactor                  decimal.Decimal
			countedAt                      time.Time
			reason, baseUnit, status, name string
		}
		lines := make([]countLine, 0)
		for rows.Next() {
			var item countLine
			var counted, factor, base, atCount, current pgtype.Numeric
			var reason *string
			if err := rows.Scan(&item.product, &item.unit, &counted, &factor, &base, &atCount, &item.countedAt, &reason, &current, &item.baseUnit, &item.status, &item.name); err != nil {
				rows.Close()
				return err
			}
			item.counted, err = money.FromNum(counted)
			if err != nil {
				rows.Close()
				return err
			}
			item.factor, err = money.FromNum(factor)
			if err != nil {
				rows.Close()
				return err
			}
			item.base, err = money.FromNum(base)
			if err != nil {
				rows.Close()
				return err
			}
			item.atCount, err = money.FromNum(atCount)
			if err != nil {
				rows.Close()
				return err
			}
			item.currentFactor, err = money.FromNum(current)
			if err != nil {
				rows.Close()
				return err
			}
			if reason != nil {
				item.reason = *reason
			}
			lines = append(lines, item)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		if len(lines) == 0 {
			return domain.Invalid("inventory", "Comptez au moins un article avant de valider.")
		}
		grouped := map[uuid.UUID][]countLine{}
		order := make([]uuid.UUID, 0)
		for _, line := range lines {
			if _, ok := grouped[line.product]; !ok {
				order = append(order, line.product)
			}
			grouped[line.product] = append(grouped[line.product], line)
		}
		sort.Slice(order, func(i, j int) bool { return order[i].String() < order[j].String() })
		for _, productID := range order {
			group := grouped[productID]
			if group[0].status != "active" {
				return domain.FailedPrecondition("product_archived", "Cet article est archivé.")
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO stock_balances (company_id, store_id, product_id, quantity, cost_complete)
				VALUES ($1,$2,$3,0,false)
				ON CONFLICT (company_id, store_id, product_id) DO NOTHING
			`, p.CompanyID, session.storeID, productID); err != nil {
				return err
			}
			var qty, avg pgtype.Numeric
			var costCurrency *string
			var complete bool
			if err := tx.QueryRow(ctx, `
				SELECT quantity, avg_cost, cost_currency, cost_complete
				FROM stock_balances
				WHERE store_id = $1 AND product_id = $2
				FOR UPDATE
			`, session.storeID, productID).Scan(&qty, &avg, &costCurrency, &complete); err != nil {
				return err
			}
			have, err := money.FromNum(qty)
			if err != nil {
				return err
			}
			countedBase := decimal.Zero
			latest := group[0].countedAt
			reason := ""
			for _, line := range group {
				if !line.factor.Equal(line.currentFactor) || !line.atCount.Equal(have) {
					return inventoryStale()
				}
				if line.countedAt.After(latest) {
					latest = line.countedAt
				}
				countedBase = countedBase.Add(line.base)
				if reason == "" {
					reason = line.reason
				}
			}
			var moved bool
			if err := tx.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1 FROM stock_movements
					WHERE store_id = $1 AND product_id = $2 AND created_at > $3
				)
			`, session.storeID, productID, latest).Scan(&moved); err != nil {
				return err
			}
			if moved {
				return inventoryStale()
			}
			delta := countedBase.Sub(have)
			var value any
			var valueCurrency any
			costIncomplete := !complete || !avg.Valid || costCurrency == nil
			if !costIncomplete {
				cost, err := money.FromNum(avg)
				if err != nil {
					return err
				}
				value = money.Num(money.Round(delta.Mul(cost)))
				valueCurrency = *costCurrency
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO inventory_counts (
					id, company_id, inventory_id, product_id, counted_quantity, reason,
					theoretical_quantity, variance_quantity, variance_value, variance_currency
				) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			`, uuid.New(), p.CompanyID, in.InventoryID, productID, money.Num(countedBase), blankNull(reason), money.Num(have), money.Num(delta), value, valueCurrency); err != nil {
				return err
			}
			if delta.IsZero() {
				continue
			}
			tag, err := tx.Exec(ctx, `
				UPDATE stock_balances
				SET quantity = quantity + $4
				WHERE company_id = $1 AND store_id = $2 AND product_id = $3 AND quantity = $5
			`, p.CompanyID, session.storeID, productID, money.Num(delta), money.Num(have))
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return inventoryStale()
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO stock_movements (
					id, company_id, store_id, product_id, quantity_delta, movement_type, reason,
					ref_type, ref_id, actor_user_id, origin_unit_name, origin_quantity, origin_provenance
				) VALUES ($1,$2,$3,$4,$5,'inventory_adjustment',$6,'inventory',$7,$8,$9,$10,'recorded')
			`, uuid.New(), p.CompanyID, session.storeID, productID, money.Num(delta), blankNull(reason), in.InventoryID, p.UserID, group[0].baseUnit, money.Num(delta)); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE inventory_sessions
			SET status = 'completed', completed_at = now(), completed_by = $2
			WHERE id = $1
		`, in.InventoryID, p.UserID); err != nil {
			return err
		}
		if err := audit(ctx, tx, p, &session.storeID, "stock.inventory.validate", "inventory", in.InventoryID, map[string]int{"lines": len(lines)}); err != nil {
			return err
		}
		out, err = assembleInventory(ctx, tx, p, in.InventoryID)
		return err
	})
	return out, err
}

func (s *Service) GetStockInventory(ctx context.Context, p tenant.Principal, id uuid.UUID) (StockInventory, error) {
	var out StockInventory
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		var storeID uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT store_id FROM inventory_sessions WHERE id = $1`, id).Scan(&storeID); err != nil {
			return domain.NotFound("Inventaire introuvable.")
		}
		if _, err := access.AuthorizeRead(ctx, tx, p.CompanyID, p.UserID, &storeID, access.StockRead); err != nil {
			return err
		}
		var err error
		out, err = assembleInventory(ctx, tx, p, id)
		return err
	})
	return out, err
}

type lockedInventory struct {
	storeID uuid.UUID
	status  string
}

func lockInventory(ctx context.Context, tx pgx.Tx, id uuid.UUID) (lockedInventory, error) {
	var session lockedInventory
	err := tx.QueryRow(ctx, `
		SELECT store_id, status FROM inventory_sessions WHERE id = $1 FOR UPDATE
	`, id).Scan(&session.storeID, &session.status)
	if err != nil {
		return lockedInventory{}, domain.NotFound("Inventaire introuvable.")
	}
	return session, nil
}

func inventoryClosed(status string) error {
	if status == "completed" {
		return domain.FailedPrecondition("inventory_completed", "Cet inventaire est déjà validé.")
	}
	return domain.FailedPrecondition("inventory_closed", "Cet inventaire n'est plus ouvert.")
}

func inventoryStale() error {
	return domain.Conflict("inventory_stale", "Le stock a changé pendant le comptage. Recomptez les articles concernés avant de valider.")
}

func assembleInventory(ctx context.Context, tx pgx.Tx, p tenant.Principal, id uuid.UUID) (StockInventory, error) {
	var out StockInventory
	var note *string
	var completed pgtype.Timestamptz
	err := tx.QueryRow(ctx, `
		SELECT id, company_id, store_id, created_by, status, note, created_at, completed_at
		FROM inventory_sessions WHERE id = $1
	`, id).Scan(&out.ID, &out.CompanyID, &out.StoreID, &out.CreatedBy, &out.Status, &note, &out.StartedAt, &completed)
	if err != nil {
		return StockInventory{}, domain.NotFound("Inventaire introuvable.")
	}
	if note != nil {
		out.Note = *note
	}
	if completed.Valid {
		done := completed.Time
		out.CompletedAt = &done
	}
	rows, err := tx.Query(ctx, `
		SELECT l.id, l.product_id, p.name, l.product_unit_id, u.name, l.factor::text,
		       l.counted_quantity::text, l.base_quantity::text, l.balance_at_count::text, l.reason
		FROM inventory_count_lines l
		JOIN products p ON p.id = l.product_id
		JOIN product_units u ON u.id = l.product_unit_id
		WHERE l.inventory_id = $1
		ORDER BY p.name, u.name, l.id
	`, id)
	if err != nil {
		return StockInventory{}, err
	}
	defer rows.Close()
	type rawLine struct {
		line    StockInventoryLine
		product uuid.UUID
		base    decimal.Decimal
		atCount decimal.Decimal
	}
	raw := make([]rawLine, 0)
	for rows.Next() {
		var item rawLine
		var factor, counted, baseText, atText string
		var reason *string
		if err := rows.Scan(&item.line.ID, &item.product, &item.line.ProductName, &item.line.ProductUnitID, &item.line.UnitName, &factor, &counted, &baseText, &atText, &reason); err != nil {
			return StockInventory{}, err
		}
		item.line.ProductID = item.product
		item.line.Factor = plainDecimal(factor)
		item.line.CountedQuantity = plainDecimal(counted)
		item.line.BaseQuantity = plainDecimal(baseText)
		item.base, err = decimal.NewFromString(baseText)
		if err != nil {
			return StockInventory{}, err
		}
		item.atCount, err = decimal.NewFromString(atText)
		if err != nil {
			return StockInventory{}, err
		}
		if reason != nil {
			item.line.Reason = *reason
		}
		raw = append(raw, item)
	}
	if err := rows.Err(); err != nil {
		return StockInventory{}, err
	}
	type rollup struct {
		theoretical, counted, variance decimal.Decimal
		value                          string
		currency                       string
		incomplete                     bool
	}
	rolls := map[uuid.UUID]rollup{}
	if out.Status == "completed" {
		saved, err := tx.Query(ctx, `
			SELECT product_id, theoretical_quantity::text, counted_quantity::text, variance_quantity::text,
			       variance_value::text, variance_currency
			FROM inventory_counts WHERE inventory_id = $1
		`, id)
		if err != nil {
			return StockInventory{}, err
		}
		defer saved.Close()
		for saved.Next() {
			var productID uuid.UUID
			var theoretical, counted, variance string
			var value *string
			var currency *string
			if err := saved.Scan(&productID, &theoretical, &counted, &variance, &value, &currency); err != nil {
				return StockInventory{}, err
			}
			item := rollup{incomplete: value == nil}
			item.theoretical, err = decimal.NewFromString(derefNum(theoretical))
			if err != nil {
				return StockInventory{}, err
			}
			item.counted, err = decimal.NewFromString(derefNum(counted))
			if err != nil {
				return StockInventory{}, err
			}
			item.variance, err = decimal.NewFromString(derefNum(variance))
			if err != nil {
				return StockInventory{}, err
			}
			if value != nil {
				item.value = plainDecimal(*value)
			}
			if currency != nil {
				item.currency = strings.TrimSpace(*currency)
			}
			rolls[productID] = item
		}
		if err := saved.Err(); err != nil {
			return StockInventory{}, err
		}
	} else {
		seen := map[uuid.UUID]struct{}{}
		for _, line := range raw {
			seen[line.product] = struct{}{}
		}
		for productID := range seen {
			var qty, avg pgtype.Numeric
			var currency *string
			var complete bool
			err := tx.QueryRow(ctx, `
				SELECT quantity, avg_cost, cost_currency, cost_complete
				FROM stock_balances WHERE store_id = $1 AND product_id = $2
			`, out.StoreID, productID).Scan(&qty, &avg, &currency, &complete)
			have := decimal.Zero
			if err == nil {
				have, err = money.FromNum(qty)
				if err != nil {
					return StockInventory{}, err
				}
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return StockInventory{}, err
			}
			counted := decimal.Zero
			for _, line := range raw {
				if line.product == productID {
					counted = counted.Add(line.base)
				}
			}
			variance := counted.Sub(have)
			item := rollup{theoretical: have, counted: counted, variance: variance, incomplete: true}
			if err == nil && complete && avg.Valid && currency != nil {
				cost, err := money.FromNum(avg)
				if err != nil {
					return StockInventory{}, err
				}
				item.incomplete = false
				item.currency = strings.TrimSpace(*currency)
				item.value = money.Round(variance.Mul(cost)).String()
			}
			rolls[productID] = item
		}
	}
	out.Lines = make([]StockInventoryLine, 0, len(raw))
	for _, line := range raw {
		roll := rolls[line.product]
		line.line.TheoreticalQuantity = roll.theoretical.String()
		line.line.CountedBaseTotal = roll.counted.String()
		line.line.Variance = roll.variance.String()
		line.line.VarianceValue = roll.value
		line.line.VarianceCurrency = roll.currency
		line.line.CostIncomplete = roll.incomplete
		out.Lines = append(out.Lines, line.line)
	}
	visible, err := access.CanReadCost(ctx, tx, p.CompanyID, p.UserID, out.StoreID)
	if err != nil {
		return StockInventory{}, err
	}
	if !visible {
		for i := range out.Lines {
			out.Lines[i].VarianceValue = ""
			out.Lines[i].VarianceCurrency = ""
		}
	}
	return out, nil
}

func derefNum(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "0"
	}
	return value
}
