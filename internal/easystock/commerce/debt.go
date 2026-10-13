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
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

type PayDebtInput struct {
	IdempotencyKey    string
	DebtID            uuid.UUID
	Amount            decimal.Decimal
	Method            string
	CashSessionID     *uuid.UUID
	ExternalReference string
}

type DebtResult struct {
	DebtID string `json:"debtId"`
	Due    string `json:"due"`
	Paid   string `json:"paid"`
}

func (s *Service) PayDebt(ctx context.Context, p tenant.Principal, in PayDebtInput) (DebtResult, error) {
	var out DebtResult
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		hash, err := hashPayload(in)
		if err != nil {
			return err
		}
		replay, err := s.beginIdempotency(ctx, tx, p.CompanyID, "pay_debt", in.IdempotencyKey, hash)
		if err != nil {
			return err
		}
		if replay != nil {
			return json.Unmarshal(replay, &out)
		}
		out, err = payDebt(ctx, tx, p, in)
		if err != nil {
			return err
		}
		return finishIdempotency(ctx, tx, p.CompanyID, "pay_debt", in.IdempotencyKey, in.DebtID, out)
	})
	return out, err
}

func payDebt(ctx context.Context, tx pgx.Tx, p tenant.Principal, in PayDebtInput) (DebtResult, error) {
	amount := money.Round(in.Amount)
	if !amount.IsPositive() {
		return DebtResult{}, domain.Invalid("amount", "Le montant du remboursement doit être supérieur à zéro.")
	}
	if in.Method != "cash" && in.Method != "external" {
		return DebtResult{}, domain.Invalid("payment", "Le mode de paiement doit être espèces ou paiement externe déclaré.")
	}
	var storeID uuid.UUID
	var currency string
	var due, collected pgtype.Numeric
	var saleID uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT store_id, currency, due_amount, collected_amount, sale_id
		FROM debts WHERE id = $1 FOR UPDATE
	`, in.DebtID).Scan(&storeID, &currency, &due, &collected, &saleID)
	if err != nil {
		return DebtResult{}, domain.NotFound("Dette introuvable.")
	}
	decision, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &storeID, access.DebtsCollect)
	if err != nil {
		return DebtResult{}, err
	}
	dueDec, err := money.FromNum(due)
	if err != nil {
		return DebtResult{}, err
	}
	if amount.GreaterThan(dueDec) {
		return DebtResult{}, domain.FailedPrecondition("debt_balance", "Le remboursement dépasse le solde restant. Saisissez un montant inférieur ou égal au restant dû.")
	}
	if in.Method == "cash" {
		if err := lockCashSession(ctx, tx, p, decision.Role, storeID, in.CashSessionID, currency); err != nil {
			return DebtResult{}, err
		}
	}
	newDue := dueDec.Sub(amount)
	collectedDec, err := money.FromNum(collected)
	if err != nil {
		return DebtResult{}, err
	}
	newCollected := collectedDec.Add(amount)
	status := "partial"
	if newDue.IsZero() {
		status = "settled"
	}
	paymentID := uuid.New()
	var session any
	if in.Method == "cash" {
		session = *in.CashSessionID
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO payments (
			id, company_id, store_id, sale_id, debt_id, method, currency, amount, external_reference, cash_session_id, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, paymentID, p.CompanyID, storeID, saleID, in.DebtID, in.Method, currency, money.Num(amount), blankNull(in.ExternalReference), session, p.UserID); err != nil {
		return DebtResult{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO debt_payments (id, company_id, debt_id, kind, amount, payment_id, created_by)
		VALUES ($1,$2,$3,'collection',$4,$5,$6)
	`, uuid.New(), p.CompanyID, in.DebtID, money.Num(amount), paymentID, p.UserID); err != nil {
		return DebtResult{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE debts
		SET collected_amount = $2, due_amount = $3, status = $4
		WHERE id = $1
	`, in.DebtID, money.Num(newCollected), money.Num(newDue), status); err != nil {
		return DebtResult{}, err
	}
	if in.Method == "cash" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO cash_transactions (
				id, company_id, session_id, currency, direction, kind, amount, ref_type, ref_id, created_by
			) VALUES ($1,$2,$3,$4,'in','debt_collection',$5,'debt',$6,$7)
		`, uuid.New(), p.CompanyID, *in.CashSessionID, currency, money.Num(amount), in.DebtID, p.UserID); err != nil {
			return DebtResult{}, err
		}
	}
	if err := audit(ctx, tx, p, &storeID, "debt.collect", "debt", in.DebtID, map[string]string{"amount": amount.String()}); err != nil {
		return DebtResult{}, err
	}
	return DebtResult{DebtID: in.DebtID.String(), Due: newDue.String(), Paid: amount.String()}, nil
}
