package commerce

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"rangoapp/internal/easystock/access"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

type RefundLineInput struct {
	SaleItemID uuid.UUID
	Quantity   decimal.Decimal
	Restock    bool
}

type CreateRefundInput struct {
	IdempotencyKey string
	SaleID         uuid.UUID
	Lines          []RefundLineInput
	Reason         string
	CashSessionID  *uuid.UUID
}

type RefundResult struct {
	RefundID   string `json:"refundId"`
	Amount     string `json:"amount"`
	CashAmount string `json:"cashAmount"`
	DebtAmount string `json:"debtAmount"`
}

// AllocateRefund splits a refund between the remaining debt and the net cash already collected.
func AllocateRefund(due, netCollected, amount decimal.Decimal) (debtPart, cashPart decimal.Decimal, err error) {
	if !amount.IsPositive() {
		return decimal.Zero, decimal.Zero, domain.Invalid("refund", "Le montant du retour doit être supérieur à zéro.")
	}
	if amount.GreaterThan(due.Add(netCollected)) {
		return decimal.Zero, decimal.Zero, domain.FailedPrecondition("refund_exceeds", "Le remboursement dépasse le montant encore disponible sur cette vente.")
	}
	if amount.LessThanOrEqual(due) {
		return amount, decimal.Zero, nil
	}
	return due, amount.Sub(due), nil
}

func (s *Service) CreateRefund(ctx context.Context, p tenant.Principal, in CreateRefundInput) (RefundResult, error) {
	var out RefundResult
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		hash, err := hashPayload(in)
		if err != nil {
			return err
		}
		replay, err := s.beginIdempotency(ctx, tx, p.CompanyID, "create_refund", in.IdempotencyKey, hash)
		if err != nil {
			return err
		}
		if replay != nil {
			return json.Unmarshal(replay, &out)
		}
		out, err = createRefund(ctx, tx, p, in)
		if err != nil {
			return err
		}
		id, err := uuid.Parse(out.RefundID)
		if err != nil {
			return err
		}
		return finishIdempotency(ctx, tx, p.CompanyID, "create_refund", in.IdempotencyKey, id, out)
	})
	return out, err
}

func createRefund(ctx context.Context, tx pgx.Tx, p tenant.Principal, in CreateRefundInput) (RefundResult, error) {
	if in.Reason == "" || len(in.Lines) == 0 {
		return RefundResult{}, domain.Invalid("refund", "Indiquez un motif et au moins un article à retourner.")
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1::text), hashtext('sale'))`, in.SaleID.String()); err != nil {
		return RefundResult{}, err
	}
	var storeID uuid.UUID
	var currency string
	var paid pgtype.Numeric
	err := tx.QueryRow(ctx, `
		SELECT store_id, currency, paid_amount FROM sales WHERE id = $1
	`, in.SaleID).Scan(&storeID, &currency, &paid)
	if errors.Is(err, pgx.ErrNoRows) {
		return RefundResult{}, domain.NotFound("Vente introuvable.")
	}
	if err != nil {
		return RefundResult{}, err
	}
	decision, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &storeID, access.SalesRefund)
	if err != nil {
		return RefundResult{}, err
	}
	total := decimal.Zero
	type item struct {
		id         uuid.UUID
		product    uuid.UUID
		unit       uuid.UUID
		unitName   string
		qty        decimal.Decimal
		base       decimal.Decimal
		factor     *decimal.Decimal
		density    *decimal.Decimal
		provenance string
		restock    bool
		amount     decimal.Decimal
	}
	items := make([]item, 0, len(in.Lines))
	for _, line := range in.Lines {
		if !line.Quantity.IsPositive() {
			return RefundResult{}, domain.Invalid("quantity", "La quantité retournée doit être supérieure à zéro.")
		}
		var soldQty, baseQty, lineNet pgtype.Numeric
		var productID, unitID uuid.UUID
		var unitName string
		if err := tx.QueryRow(ctx, `
			SELECT s.product_id, s.product_unit_id, u.name, s.quantity, s.base_quantity, s.line_net
			FROM sale_items s
			JOIN product_units u ON u.id = s.product_unit_id
			WHERE s.id = $1 AND s.sale_id = $2
		`, line.SaleItemID, in.SaleID).Scan(&productID, &unitID, &unitName, &soldQty, &baseQty, &lineNet); err != nil {
			return RefundResult{}, domain.NotFound("Ligne de vente introuvable.")
		}
		sold, _ := money.FromNum(soldQty)
		base, _ := money.FromNum(baseQty)
		netLine, _ := money.FromNum(lineNet)
		var alreadyQty, alreadyAmount, alreadyBase pgtype.Numeric
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(quantity), 0), COALESCE(SUM(amount), 0), COALESCE(SUM(base_quantity), 0)
			FROM refund_items WHERE sale_item_id = $1
		`, line.SaleItemID).Scan(&alreadyQty, &alreadyAmount, &alreadyBase); err != nil {
			return RefundResult{}, err
		}
		prevQty, _ := money.FromNum(alreadyQty)
		prevAmount, _ := money.FromNum(alreadyAmount)
		prevBase, _ := money.FromNum(alreadyBase)
		if prevQty.Add(line.Quantity).GreaterThan(sold) {
			return RefundResult{}, domain.FailedPrecondition("refund_quantity", "La quantité retournée dépasse la quantité encore remboursable.")
		}
		var amount decimal.Decimal
		if prevQty.Add(line.Quantity).Equal(sold) {
			amount = netLine.Sub(prevAmount)
		} else {
			amount = money.Round(netLine.Mul(line.Quantity).Div(sold))
		}
		baseBack := refundBase(sold, base, prevQty, prevBase, line.Quantity)
		if !baseBack.IsPositive() {
			return RefundResult{}, domain.Invalid("quantity", "La quantité convertie est trop petite pour être stockée.")
		}
		traceName, factor, density, err := loadSaleTrace(ctx, tx, p.CompanyID, storeID, in.SaleID, productID, unitID, sold, base)
		if err != nil {
			return RefundResult{}, err
		}
		if traceName != "" {
			unitName = traceName
		}
		provenance := "recovered"
		if conversionExplained(line.Quantity, baseBack, factor, density) {
			provenance = "recorded"
		}
		items = append(items, item{
			id: line.SaleItemID, product: productID, unit: unitID, unitName: unitName,
			qty: line.Quantity, base: baseBack, factor: factor, density: density, provenance: provenance,
			restock: line.Restock, amount: amount,
		})
		total = total.Add(amount)
	}
	total = money.Round(total)
	paidDec, _ := money.FromNum(paid)
	var refundedCash pgtype.Numeric
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(cash_amount), 0) FROM refunds WHERE sale_id = $1
	`, in.SaleID).Scan(&refundedCash); err != nil {
		return RefundResult{}, err
	}
	alreadyCash, _ := money.FromNum(refundedCash)
	netCollected := paidDec.Sub(alreadyCash)
	var dueDec decimal.Decimal
	var debtID *uuid.UUID
	var debtUUID uuid.UUID
	var offset, due pgtype.Numeric
	err = tx.QueryRow(ctx, `
		SELECT id, offset_amount, due_amount FROM debts WHERE sale_id = $1 FOR UPDATE
	`, in.SaleID).Scan(&debtUUID, &offset, &due)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return RefundResult{}, err
	}
	if err == nil {
		debtID = &debtUUID
		dueDec, _ = money.FromNum(due)
	}
	debtPart, cashPart, err := AllocateRefund(dueDec, netCollected, total)
	if err != nil {
		return RefundResult{}, err
	}
	if cashPart.IsPositive() {
		if err := lockCashSession(ctx, tx, p, decision.Role, storeID, in.CashSessionID, currency); err != nil {
			return RefundResult{}, err
		}
	}
	refundID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO refunds (
			id, company_id, store_id, sale_id, currency, amount, cash_amount, debt_amount, reason, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, refundID, p.CompanyID, storeID, in.SaleID, currency, money.Num(total), money.Num(cashPart), money.Num(debtPart), in.Reason, p.UserID); err != nil {
		return RefundResult{}, err
	}
	for _, line := range items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO refund_items (
				id, company_id, refund_id, sale_item_id, quantity, base_quantity, restock, amount
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		`, uuid.New(), p.CompanyID, refundID, line.id, money.Num(line.qty), money.Num(line.base), line.restock, money.Num(line.amount)); err != nil {
			return RefundResult{}, err
		}
		if !line.restock {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO stock_balances (company_id, store_id, product_id, quantity, cost_complete)
			VALUES ($1,$2,$3,0,false)
			ON CONFLICT (company_id, store_id, product_id) DO NOTHING
		`, p.CompanyID, storeID, line.product); err != nil {
			return RefundResult{}, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE stock_balances SET quantity = quantity + $4
			WHERE company_id = $1 AND store_id = $2 AND product_id = $3
		`, p.CompanyID, storeID, line.product, money.Num(line.base)); err != nil {
			return RefundResult{}, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO stock_movements (
				id, company_id, store_id, product_id, quantity_delta, movement_type, reason, ref_type, ref_id, actor_user_id,
				origin_unit_id, origin_unit_name, origin_quantity, origin_factor, density_kg_per_liter, origin_provenance
			) VALUES ($1,$2,$3,$4,$5,'refund',$6,'refund',$7,$8,$9,$10,$11,$12,$13,$14)
		`, uuid.New(), p.CompanyID, storeID, line.product, money.Num(line.base), in.Reason, refundID, p.UserID, line.unit, line.unitName, money.Num(line.qty), factorArg(line.factor), densityArg(line.density), line.provenance); err != nil {
			return RefundResult{}, err
		}
	}
	if debtPart.IsPositive() && debtID != nil {
		offsetDec, _ := money.FromNum(offset)
		newOffset := offsetDec.Add(debtPart)
		newDue := dueDec.Sub(debtPart)
		status := "partial"
		if newDue.IsZero() {
			status = "settled"
		}
		if _, err := tx.Exec(ctx, `
			UPDATE debts SET offset_amount = $2, due_amount = $3, status = $4 WHERE id = $1
		`, *debtID, money.Num(newOffset), money.Num(newDue), status); err != nil {
			return RefundResult{}, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO debt_payments (id, company_id, debt_id, kind, amount, refund_id, created_by)
			VALUES ($1,$2,$3,'refund_offset',$4,$5,$6)
		`, uuid.New(), p.CompanyID, *debtID, money.Num(debtPart), refundID, p.UserID); err != nil {
			return RefundResult{}, err
		}
	}
	if cashPart.IsPositive() {
		if _, err := tx.Exec(ctx, `
			INSERT INTO cash_transactions (
				id, company_id, session_id, currency, direction, kind, amount, ref_type, ref_id, created_by
			) VALUES ($1,$2,$3,$4,'out','refund',$5,'refund',$6,$7)
		`, uuid.New(), p.CompanyID, *in.CashSessionID, currency, money.Num(cashPart), refundID, p.UserID); err != nil {
			return RefundResult{}, err
		}
	}
	if err := audit(ctx, tx, p, &storeID, "sale.refund", "refund", refundID, map[string]string{"amount": total.String()}); err != nil {
		return RefundResult{}, err
	}
	return RefundResult{RefundID: refundID.String(), Amount: total.String(), CashAmount: cashPart.String(), DebtAmount: debtPart.String()}, nil
}

// loadSaleTrace copies the conversion stored on the unique sale movement.
// Zero or several matches return empty values. The current product density is never used.
func loadSaleTrace(ctx context.Context, tx pgx.Tx, companyID, storeID, saleID, productID, unitID uuid.UUID, soldQty, soldBase decimal.Decimal) (string, *decimal.Decimal, *decimal.Decimal, error) {
	rows, err := tx.Query(ctx, `
		SELECT origin_unit_name, origin_factor::text, density_kg_per_liter::text
		FROM stock_movements
		WHERE company_id = $1
		  AND store_id = $2
		  AND ref_type = 'sale'
		  AND ref_id = $3
		  AND product_id = $4
		  AND origin_unit_id = $5
		  AND origin_quantity = $6
		  AND quantity_delta = $7
	`, companyID, storeID, saleID, productID, unitID, money.Num(soldQty), money.Num(soldBase.Neg()))
	if err != nil {
		return "", nil, nil, err
	}
	defer rows.Close()
	var name *string
	var factorText, densityText *string
	matches := 0
	for rows.Next() {
		matches++
		if err := rows.Scan(&name, &factorText, &densityText); err != nil {
			return "", nil, nil, err
		}
	}
	if err := rows.Err(); err != nil {
		return "", nil, nil, err
	}
	if matches != 1 {
		return "", nil, nil, nil
	}
	factor, err := optionalDecimal(factorText)
	if err != nil {
		return "", nil, nil, err
	}
	density, err := optionalDecimal(densityText)
	if err != nil {
		return "", nil, nil, err
	}
	if name == nil {
		return "", factor, density, nil
	}
	return *name, factor, density, nil
}

func optionalDecimal(raw *string) (*decimal.Decimal, error) {
	if raw == nil {
		return nil, nil
	}
	text := strings.TrimSpace(*raw)
	if text == "" {
		return nil, nil
	}
	parsed, err := decimal.NewFromString(text)
	if err != nil {
		return nil, err
	}
	if !parsed.IsPositive() {
		return nil, nil
	}
	return &parsed, nil
}
