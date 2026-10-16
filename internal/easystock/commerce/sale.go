package commerce

import (
	"context"
	"encoding/json"
	"sort"

	"rangoapp/internal/easystock/access"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

type SaleLineInput struct {
	ProductID         uuid.UUID
	UnitName          string
	Quantity          decimal.Decimal
	ExpectedUnitPrice decimal.Decimal
}

type PaymentInput struct {
	Method            string
	Amount            decimal.Decimal
	ExternalReference string
}

type ConfirmSaleInput struct {
	IdempotencyKey string
	StoreID        uuid.UUID
	CustomerID     *uuid.UUID
	Currency       string
	Lines          []SaleLineInput
	Payments       []PaymentInput
	CashSessionID  *uuid.UUID
	DiscountAmount decimal.Decimal
	ClientDraftID  string
}

type SaleResult struct {
	SaleID   uuid.UUID  `json:"saleId"`
	Currency string     `json:"currency"`
	Net      string     `json:"net"`
	Paid     string     `json:"paid"`
	Due      string     `json:"due"`
	DebtID   *uuid.UUID `json:"debtId,omitempty"`
}

type resolvedLine struct {
	ProductID    uuid.UUID
	UnitID       uuid.UUID
	Quantity     decimal.Decimal
	BaseQty      decimal.Decimal
	UnitPrice    decimal.Decimal
	LineGross    decimal.Decimal
	LineNet      decimal.Decimal
	UnitCost     decimal.Decimal
	CostComplete bool
	UnitName     string
}

func (s *Service) ConfirmSale(ctx context.Context, p tenant.Principal, in ConfirmSaleInput) (SaleResult, error) {
	var out SaleResult
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		hash, err := hashPayload(in)
		if err != nil {
			return err
		}
		replay, err := s.beginIdempotency(ctx, tx, p.CompanyID, "confirm_sale", in.IdempotencyKey, hash)
		if err != nil {
			return err
		}
		if replay != nil {
			return json.Unmarshal(replay, &out)
		}
		out, err = confirmSale(ctx, tx, p, in)
		if err != nil {
			return err
		}
		return finishIdempotency(ctx, tx, p.CompanyID, "confirm_sale", in.IdempotencyKey, out.SaleID, out)
	})
	return out, err
}

func confirmSale(ctx context.Context, tx pgx.Tx, p tenant.Principal, in ConfirmSaleInput) (SaleResult, error) {
	if !validCurrency(in.Currency) {
		return SaleResult{}, domain.Invalid("currency", "Indiquez une devise sur trois lettres, par exemple USD ou CDF.")
	}
	if len(in.Lines) == 0 {
		return SaleResult{}, domain.Invalid("lines", "Ajoutez au moins un article à la vente.")
	}
	decision, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &in.StoreID, access.SalesCreate)
	if err != nil {
		return SaleResult{}, err
	}
	var storeOK bool
	if err := tx.QueryRow(ctx, `SELECT true FROM stores WHERE id = $1 AND archived_at IS NULL`, in.StoreID).Scan(&storeOK); err != nil || !storeOK {
		return SaleResult{}, domain.NotFound("Boutique introuvable.")
	}

	lines := append([]SaleLineInput(nil), in.Lines...)
	sort.Slice(lines, func(i, j int) bool {
		if lines[i].ProductID == lines[j].ProductID {
			return lines[i].UnitName < lines[j].UnitName
		}
		return lines[i].ProductID.String() < lines[j].ProductID.String()
	})

	resolved := make([]resolvedLine, 0, len(lines))
	grossParts := make([]decimal.Decimal, 0, len(lines))
	for _, line := range lines {
		item, err := resolveLine(ctx, tx, in.StoreID, in.Currency, line)
		if err != nil {
			return SaleResult{}, err
		}
		resolved = append(resolved, item)
		grossParts = append(grossParts, item.LineGross)
	}
	gross := decimal.Zero
	for _, part := range grossParts {
		gross = gross.Add(part)
	}
	discount := money.Round(in.DiscountAmount)
	if discount.IsNegative() || discount.GreaterThan(gross) {
		return SaleResult{}, domain.Invalid("discount", "La remise ne peut pas dépasser le montant des articles.")
	}
	if discount.IsPositive() {
		discountDecision, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &in.StoreID, access.SalesDiscount)
		if err != nil {
			return SaleResult{}, err
		}
		if !discountDecision.UnlimitedDiscount && (gross.IsZero() || discount.Div(gross).GreaterThan(discountDecision.MaxDiscountRate)) {
			return SaleResult{}, domain.PermissionDenied("discount_ceiling", "La remise dépasse le plafond autorisé.")
		}
	}
	nets := allocateNets(grossParts, discount)
	net := gross.Sub(discount)
	for i := range resolved {
		resolved[i].LineNet = nets[i]
	}

	paid := decimal.Zero
	cashPaid := decimal.Zero
	for _, payment := range in.Payments {
		amount := money.Round(payment.Amount)
		if !amount.IsPositive() {
			return SaleResult{}, domain.Invalid("payment", "Chaque paiement doit être supérieur à zéro.")
		}
		if payment.Method != "cash" && payment.Method != "external" {
			return SaleResult{}, domain.Invalid("payment", "Le mode de paiement doit être espèces ou paiement externe déclaré.")
		}
		paid = paid.Add(amount)
		if payment.Method == "cash" {
			cashPaid = cashPaid.Add(amount)
		}
	}
	paid = money.Round(paid)
	if paid.GreaterThan(net) {
		return SaleResult{}, domain.Invalid("payment", "Le montant reçu dépasse le montant à payer. Ajustez les paiements.")
	}
	due := net.Sub(paid)
	if due.IsPositive() {
		if in.CustomerID == nil {
			return SaleResult{}, domain.Invalid("customer", "Choisissez un client pour une vente qui n'est pas entièrement payée.")
		}
		credit, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &in.StoreID, access.CreditAuthorize)
		if err != nil {
			return SaleResult{}, err
		}
		if !credit.UnlimitedCredit && due.GreaterThan(credit.MaxCredit) {
			return SaleResult{}, domain.PermissionDenied("credit_ceiling", "Le crédit dépasse le plafond autorisé.")
		}
	}
	if cashPaid.IsPositive() {
		if err := lockCashSession(ctx, tx, p, decision.Role, in.StoreID, in.CashSessionID, in.Currency); err != nil {
			return SaleResult{}, err
		}
	}

	if err := lockAndPriceStock(ctx, tx, in.StoreID, resolved); err != nil {
		return SaleResult{}, err
	}

	var creditLimit pgtype.Numeric
	if in.CustomerID != nil {
		if err := tx.QueryRow(ctx, `
			SELECT credit_limit FROM customers WHERE id = $1 AND store_id = $2 AND archived_at IS NULL
		`, *in.CustomerID, in.StoreID).Scan(&creditLimit); err != nil {
			return SaleResult{}, domain.NotFound("Client introuvable dans cette boutique.")
		}
		if creditLimit.Valid && due.IsPositive() {
			limit, err := money.FromNum(creditLimit)
			if err != nil {
				return SaleResult{}, err
			}
			var outstanding pgtype.Numeric
			if err := tx.QueryRow(ctx, `
				SELECT COALESCE(SUM(due_amount), 0)
				FROM debts
				WHERE customer_id = $1 AND currency = $2 AND due_amount > 0
			`, *in.CustomerID, in.Currency).Scan(&outstanding); err != nil {
				return SaleResult{}, err
			}
			open, err := money.FromNum(outstanding)
			if err != nil {
				return SaleResult{}, err
			}
			if open.Add(due).GreaterThan(limit) {
				return SaleResult{}, domain.FailedPrecondition("credit_limit", "Ce client a atteint sa limite de crédit.")
			}
		}
	}

	saleID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO sales (
			id, company_id, store_id, customer_id, operator_user_id, currency,
			gross_amount, discount_amount, net_amount, paid_amount, due_amount
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, saleID, p.CompanyID, in.StoreID, in.CustomerID, p.UserID, in.Currency,
		money.Num(gross), money.Num(discount), money.Num(net), money.Num(paid), money.Num(due)); err != nil {
		return SaleResult{}, err
	}

	totals := map[uuid.UUID]decimal.Decimal{}
	for _, line := range resolved {
		itemID := uuid.New()
		var unitCost any
		if line.CostComplete {
			unitCost = money.Num(line.UnitCost)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO sale_items (
				id, company_id, sale_id, product_id, product_unit_id, quantity, base_quantity,
				unit_price, line_gross, line_net, unit_cost, cost_complete
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		`, itemID, p.CompanyID, saleID, line.ProductID, line.UnitID,
			money.Num(line.Quantity), money.Num(line.BaseQty), money.Num(line.UnitPrice),
			money.Num(line.LineGross), money.Num(line.LineNet), unitCost, line.CostComplete); err != nil {
			return SaleResult{}, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO stock_movements (
				id, company_id, store_id, product_id, quantity_delta, movement_type, ref_type, ref_id, actor_user_id,
				origin_unit_id, origin_unit_name, origin_quantity
			) VALUES ($1,$2,$3,$4,$5,'sale','sale',$6,$7,$8,$9,$10)
		`, uuid.New(), p.CompanyID, in.StoreID, line.ProductID, money.Num(line.BaseQty.Neg()), saleID, p.UserID, line.UnitID, line.UnitName, money.Num(line.Quantity)); err != nil {
			return SaleResult{}, err
		}
		totals[line.ProductID] = totals[line.ProductID].Add(line.BaseQty)
	}
	for productID, qty := range totals {
		tag, err := tx.Exec(ctx, `
			UPDATE stock_balances
			SET quantity = quantity - $4
			WHERE company_id = $1 AND store_id = $2 AND product_id = $3 AND quantity >= $4
		`, p.CompanyID, in.StoreID, productID, money.Num(qty))
		if err != nil {
			return SaleResult{}, err
		}
		if tag.RowsAffected() != 1 {
			return SaleResult{}, domain.Conflict("stock_conflict", "Stock insuffisant pour cet article. Réduisez la quantité ou approvisionnez la boutique.")
		}
	}

	for _, payment := range in.Payments {
		paymentID := uuid.New()
		var session any
		if payment.Method == "cash" {
			session = *in.CashSessionID
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO payments (
				id, company_id, store_id, sale_id, method, currency, amount, external_reference, cash_session_id, created_by
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		`, paymentID, p.CompanyID, in.StoreID, saleID, payment.Method, in.Currency, money.Num(money.Round(payment.Amount)),
			blankNull(payment.ExternalReference), session, p.UserID); err != nil {
			return SaleResult{}, err
		}
		if payment.Method == "cash" {
			if _, err := tx.Exec(ctx, `
				INSERT INTO cash_transactions (
					id, company_id, session_id, currency, direction, kind, amount, ref_type, ref_id, created_by
				) VALUES ($1,$2,$3,$4,'in','sale',$5,'sale',$6,$7)
			`, uuid.New(), p.CompanyID, *in.CashSessionID, in.Currency, money.Num(money.Round(payment.Amount)), saleID, p.UserID); err != nil {
				return SaleResult{}, err
			}
		}
	}

	var debtID *uuid.UUID
	if due.IsPositive() {
		id := uuid.New()
		debtID = &id
		status := "open"
		if paid.IsPositive() {
			status = "partial"
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO debts (
				id, company_id, store_id, customer_id, sale_id, currency,
				original_amount, collected_amount, offset_amount, due_amount, status
			) VALUES ($1,$2,$3,$4,$5,$6,$7,0,0,$7,$8)
		`, id, p.CompanyID, in.StoreID, *in.CustomerID, saleID, in.Currency, money.Num(due), status); err != nil {
			return SaleResult{}, err
		}
	}
	if in.ClientDraftID != "" {
		if _, err := tx.Exec(ctx, `
			UPDATE sale_drafts SET status = 'confirmed'
			WHERE company_id = $1 AND client_draft_id = $2 AND status = 'open'
		`, p.CompanyID, in.ClientDraftID); err != nil {
			return SaleResult{}, err
		}
	}
	if err := audit(ctx, tx, p, &in.StoreID, "sale.confirm", "sale", saleID, map[string]string{
		"currency": in.Currency,
		"net":      net.String(),
		"paid":     paid.String(),
		"due":      due.String(),
	}); err != nil {
		return SaleResult{}, err
	}
	return SaleResult{
		SaleID:   saleID,
		Currency: in.Currency,
		Net:      net.String(),
		Paid:     paid.String(),
		Due:      due.String(),
		DebtID:   debtID,
	}, nil
}

func resolveLine(ctx context.Context, tx pgx.Tx, storeID uuid.UUID, currency string, line SaleLineInput) (resolvedLine, error) {
	if !line.Quantity.IsPositive() {
		return resolvedLine{}, domain.Invalid("quantity", "La quantité doit être supérieure à zéro.")
	}
	var status string
	if err := tx.QueryRow(ctx, `
		SELECT status FROM products WHERE id = $1
	`, line.ProductID).Scan(&status); err != nil {
		return resolvedLine{}, domain.NotFound("Article introuvable.")
	}
	if status != "active" {
		return resolvedLine{}, domain.FailedPrecondition("product_archived", "Cet article est archivé et ne peut plus être vendu.")
	}
	var unitID uuid.UUID
	var factor pgtype.Numeric
	var active, allowSale, unitFractional bool
	if err := tx.QueryRow(ctx, `
		SELECT id, factor, active, allow_sale, allow_fractional
		FROM product_units WHERE product_id = $1 AND name = $2
	`, line.ProductID, line.UnitName).Scan(&unitID, &factor, &active, &allowSale, &unitFractional); err != nil {
		return resolvedLine{}, domain.NotFound("Conditionnement introuvable pour cet article.")
	}
	if !active {
		return resolvedLine{}, domain.FailedPrecondition("unit_archived", "Ce conditionnement est archivé et ne peut plus être vendu.")
	}
	if !allowSale {
		return resolvedLine{}, domain.FailedPrecondition("unit_not_for_sale", "Ce conditionnement n'est pas autorisé à la vente.")
	}
	if err := soldQuantityAllowed(unitFractional, line.Quantity); err != nil {
		return resolvedLine{}, err
	}
	factorDec, err := money.FromNum(factor)
	if err != nil {
		return resolvedLine{}, err
	}
	var price pgtype.Numeric
	if err := tx.QueryRow(ctx, `
		SELECT unit_price FROM product_prices
		WHERE store_id = $1 AND product_id = $2 AND product_unit_id = $3 AND currency = $4
		  AND archived_at IS NULL
		FOR UPDATE
	`, storeID, line.ProductID, unitID, currency).Scan(&price); err != nil {
		return resolvedLine{}, domain.NotFound("Aucun prix n'est défini pour cet article dans cette boutique et cette devise.")
	}
	unitPrice, err := money.FromNum(price)
	if err != nil {
		return resolvedLine{}, err
	}
	unitPrice = money.Round(unitPrice)
	if !unitPrice.Equal(money.Round(line.ExpectedUnitPrice)) {
		return resolvedLine{}, domain.Conflict("price_changed", "Le prix a changé depuis la préparation de la vente. Vérifiez le nouveau prix avant de confirmer.")
	}
	baseQty := line.Quantity.Mul(factorDec)
	return resolvedLine{
		ProductID: line.ProductID,
		UnitID:    unitID,
		UnitName:  line.UnitName,
		Quantity:  line.Quantity,
		BaseQty:   baseQty,
		UnitPrice: unitPrice,
		LineGross: money.Round(line.Quantity.Mul(unitPrice)),
	}, nil
}

func lockAndPriceStock(ctx context.Context, tx pgx.Tx, storeID uuid.UUID, lines []resolvedLine) error {
	seen := map[uuid.UUID]decimal.Decimal{}
	order := make([]uuid.UUID, 0)
	for _, line := range lines {
		if _, ok := seen[line.ProductID]; !ok {
			order = append(order, line.ProductID)
		}
		seen[line.ProductID] = seen[line.ProductID].Add(line.BaseQty)
	}
	sort.Slice(order, func(i, j int) bool { return order[i].String() < order[j].String() })
	costs := map[uuid.UUID]struct {
		cost     decimal.Decimal
		complete bool
	}{}
	for _, productID := range order {
		if _, err := tx.Exec(ctx, `
			INSERT INTO stock_balances (company_id, store_id, product_id, quantity, cost_complete)
			VALUES (app_setting_uuid('app.company_id'), $1, $2, 0, false)
			ON CONFLICT (company_id, store_id, product_id) DO NOTHING
		`, storeID, productID); err != nil {
			return err
		}
		var qty, avg pgtype.Numeric
		var complete bool
		if err := tx.QueryRow(ctx, `
			SELECT quantity, avg_cost, cost_complete
			FROM stock_balances
			WHERE store_id = $1 AND product_id = $2
			FOR UPDATE
		`, storeID, productID).Scan(&qty, &avg, &complete); err != nil {
			return err
		}
		have, err := money.FromNum(qty)
		if err != nil {
			return err
		}
		if have.LessThan(seen[productID]) {
			return domain.Conflict("stock_conflict", "Stock insuffisant pour cet article. Réduisez la quantité ou approvisionnez la boutique.")
		}
		cost, err := money.FromNum(avg)
		if err != nil {
			return err
		}
		costs[productID] = struct {
			cost     decimal.Decimal
			complete bool
		}{cost: cost, complete: complete && avg.Valid}
	}
	for i := range lines {
		info := costs[lines[i].ProductID]
		lines[i].UnitCost = info.cost
		lines[i].CostComplete = info.complete
	}
	return nil
}

func lockCashSession(ctx context.Context, tx pgx.Tx, p tenant.Principal, role string, storeID uuid.UUID, sessionID *uuid.UUID, currency string) error {
	if sessionID == nil {
		return domain.FailedPrecondition("cash_session", "Ouvrez une session de caisse avant d'encaisser des espèces.")
	}
	_ = currency
	var gotStore uuid.UUID
	var status string
	var openedBy uuid.UUID
	if err := tx.QueryRow(ctx, `
		SELECT store_id, status, opened_by FROM cash_sessions WHERE id = $1 FOR UPDATE
	`, *sessionID).Scan(&gotStore, &status, &openedBy); err != nil {
		return domain.NotFound("Session de caisse introuvable.")
	}
	if gotStore != storeID || status != "open" {
		return domain.FailedPrecondition("cash_session", "La session de caisse n'est pas ouverte dans cette boutique.")
	}
	return denyForeignDrawer(role, openedBy, p.UserID, "Vous ne pouvez encaisser que sur la session que vous avez ouverte.")
}

func allocateNets(grosses []decimal.Decimal, discount decimal.Decimal) []decimal.Decimal {
	nets := make([]decimal.Decimal, len(grosses))
	gross := decimal.Zero
	for _, value := range grosses {
		gross = gross.Add(value)
	}
	netTotal := gross.Sub(discount)
	if len(grosses) == 0 {
		return nets
	}
	used := decimal.Zero
	for i := 0; i < len(grosses)-1; i++ {
		share := decimal.Zero
		if gross.IsPositive() {
			share = money.Round(discount.Mul(grosses[i]).Div(gross))
		}
		nets[i] = grosses[i].Sub(share)
		used = used.Add(nets[i])
	}
	nets[len(nets)-1] = netTotal.Sub(used)
	return nets
}

func blankNull(value string) any {
	if value == "" {
		return nil
	}
	return value
}
