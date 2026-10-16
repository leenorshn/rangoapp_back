package commerce

import (
	"context"

	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

type balanceSnap struct {
	have     decimal.Decimal
	avg      decimal.Decimal
	avgValid bool
	currency string
	complete bool
}

func (s balanceSnap) valuedExit(base decimal.Decimal) (unitCost decimal.Decimal, value decimal.Decimal, currency string, ok bool) {
	if !s.complete || !s.avgValid || !validCurrency(s.currency) {
		return decimal.Zero, decimal.Zero, "", false
	}
	return s.avg, money.Round(base.Mul(s.avg)), s.currency, true
}

func lockProductBalance(ctx context.Context, tx pgx.Tx, companyID, storeID, productID uuid.UUID) (balanceSnap, error) {
	if _, err := tx.Exec(ctx, `
		INSERT INTO stock_balances (company_id, store_id, product_id, quantity, cost_complete)
		VALUES ($1,$2,$3,0,false)
		ON CONFLICT (company_id, store_id, product_id) DO NOTHING
	`, companyID, storeID, productID); err != nil {
		return balanceSnap{}, err
	}
	var qty, avg pgtype.Numeric
	var currency *string
	var complete bool
	if err := tx.QueryRow(ctx, `
		SELECT quantity, avg_cost, cost_currency, cost_complete
		FROM stock_balances
		WHERE store_id = $1 AND product_id = $2
		FOR UPDATE
	`, storeID, productID).Scan(&qty, &avg, &currency, &complete); err != nil {
		return balanceSnap{}, err
	}
	have, err := money.FromNum(qty)
	if err != nil {
		return balanceSnap{}, err
	}
	average, err := money.FromNum(avg)
	if err != nil {
		return balanceSnap{}, err
	}
	snap := balanceSnap{have: have, avg: average, avgValid: avg.Valid, complete: complete && avg.Valid}
	if currency != nil {
		snap.currency = *currency
	}
	return snap, nil
}

func consumeLocked(ctx context.Context, tx pgx.Tx, companyID, storeID, productID uuid.UUID, base decimal.Decimal) error {
	tag, err := tx.Exec(ctx, `
		UPDATE stock_balances
		SET quantity = quantity - $4
		WHERE company_id = $1 AND store_id = $2 AND product_id = $3 AND quantity >= $4
	`, companyID, storeID, productID, money.Num(base))
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return domain.Conflict("stock_conflict", "Stock insuffisant pour cet article. Réduisez la quantité ou approvisionnez la boutique.")
	}
	return nil
}

func addUncostedQuantity(ctx context.Context, tx pgx.Tx, companyID, storeID, productID uuid.UUID, base decimal.Decimal) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO stock_balances (company_id, store_id, product_id, quantity, cost_complete)
		VALUES ($1,$2,$3,0,false)
		ON CONFLICT (company_id, store_id, product_id) DO NOTHING
	`, companyID, storeID, productID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		SELECT 1 FROM stock_balances WHERE store_id = $1 AND product_id = $2 FOR UPDATE
	`, storeID, productID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		UPDATE stock_balances
		SET quantity = quantity + $4, cost_complete = false
		WHERE company_id = $1 AND store_id = $2 AND product_id = $3
	`, companyID, storeID, productID, money.Num(base))
	return err
}

func postStockMovement(ctx context.Context, tx pgx.Tx, p tenant.Principal, storeID, productID uuid.UUID, delta decimal.Decimal, movementType, reason, refType string, refID uuid.UUID, unit stockUnit, originQty decimal.Decimal, exitValue *decimal.Decimal, exitCurrency string) error {
	var value any
	var currency any
	if exitValue != nil && validCurrency(exitCurrency) {
		value = money.Num(*exitValue)
		currency = exitCurrency
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO stock_movements (
			id, company_id, store_id, product_id, quantity_delta, movement_type, reason,
			ref_type, ref_id, actor_user_id, origin_unit_id, origin_unit_name, origin_quantity, origin_factor,
			exit_value, exit_currency
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	`, uuid.New(), p.CompanyID, storeID, productID, money.Num(delta), movementType, blankNull(reason),
		refType, refID, p.UserID, unit.id, unit.name, money.Num(originQty), money.Num(unit.factor), value, currency)
	return err
}
