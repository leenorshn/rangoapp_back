package commerce

import (
	"context"

	"rangoapp/internal/easystock/access"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

type OpeningAmount struct {
	Currency string
	Amount   decimal.Decimal
}

type OpenCashInput struct {
	RegisterID uuid.UUID
	Openings   []OpeningAmount
}

type CloseCount struct {
	Currency string
	Counted  decimal.Decimal
	Reason   string
}

type CloseCashInput struct {
	SessionID uuid.UUID
	Counts    []CloseCount
}

type ExpenseInput struct {
	StoreID       uuid.UUID
	Category      string
	Amount        decimal.Decimal
	Currency      string
	PaidFromCash  bool
	CashSessionID *uuid.UUID
	MediaID       *uuid.UUID
	Note          string
}

type CashAdjustmentInput struct {
	SessionID          uuid.UUID
	Currency           string
	Direction          string
	Amount             decimal.Decimal
	Reason             string
	ReferenceSessionID uuid.UUID
}

func (s *Service) OpenCashSession(ctx context.Context, p tenant.Principal, in OpenCashInput) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		var storeID uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT store_id FROM cash_registers WHERE id = $1 AND archived_at IS NULL`, in.RegisterID).Scan(&storeID); err != nil {
			return domain.NotFound("Caisse introuvable.")
		}
		if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &storeID, access.CashOpen); err != nil {
			return err
		}
		id = uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO cash_sessions (id, company_id, store_id, register_id, opened_by, status)
			VALUES ($1,$2,$3,$4,$5,'open')
		`, id, p.CompanyID, storeID, in.RegisterID, p.UserID); err != nil {
			return err
		}
		for _, opening := range in.Openings {
			amount := money.Round(opening.Amount)
			if amount.IsZero() {
				continue
			}
			if !validCurrency(opening.Currency) || amount.IsNegative() {
				return domain.Invalid("opening", "Le fonds initial doit être positif et libellé dans une devise valide.")
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO cash_transactions (
					id, company_id, session_id, currency, direction, kind, amount, created_by
				) VALUES ($1,$2,$3,$4,'in','opening',$5,$6)
			`, uuid.New(), p.CompanyID, id, opening.Currency, money.Num(amount), p.UserID); err != nil {
				return err
			}
		}
		return audit(ctx, tx, p, &storeID, "cash.open", "cash_session", id, map[string]string{"registerId": in.RegisterID.String()})
	})
	return id, err
}

func (s *Service) CloseCashSession(ctx context.Context, p tenant.Principal, in CloseCashInput) error {
	return s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		var storeID uuid.UUID
		var status string
		var openedBy uuid.UUID
		if err := tx.QueryRow(ctx, `
			SELECT store_id, status, opened_by FROM cash_sessions WHERE id = $1 FOR UPDATE
		`, in.SessionID).Scan(&storeID, &status, &openedBy); err != nil {
			return domain.NotFound("Session de caisse introuvable.")
		}
		decision, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &storeID, access.CashClose)
		if err != nil {
			return err
		}
		if status != "open" {
			return domain.FailedPrecondition("cash_session_closed", "Cette session est déjà clôturée.")
		}
		if decision.Role == "cashier" && openedBy != p.UserID {
			return domain.PermissionDenied("cash_session", "Vous ne pouvez clôturer que votre propre session.")
		}
		rows, err := tx.Query(ctx, `
			SELECT currency, COALESCE(SUM(CASE WHEN direction = 'in' THEN amount ELSE -amount END), 0)
			FROM cash_transactions WHERE session_id = $1 GROUP BY currency
		`, in.SessionID)
		if err != nil {
			return err
		}
		defer rows.Close()
		expected := map[string]decimal.Decimal{}
		for rows.Next() {
			var currency string
			var amount pgtype.Numeric
			if err := rows.Scan(&currency, &amount); err != nil {
				return err
			}
			parsed, err := money.FromNum(amount)
			if err != nil {
				return err
			}
			expected[currency] = parsed
		}
		if err := rows.Err(); err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, count := range in.Counts {
			if !validCurrency(count.Currency) {
				return domain.Invalid("currency", "Indiquez une devise valide pour le comptage.")
			}
			exp := expected[count.Currency]
			counted := money.Round(count.Counted)
			variance := counted.Sub(exp)
			if !variance.IsZero() && count.Reason == "" {
				return domain.Invalid("variance", "Expliquez l'écart entre le montant compté et le montant attendu.")
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO cash_session_counts (
					company_id, session_id, currency, expected_amount, counted_amount, variance, variance_reason
				) VALUES ($1,$2,$3,$4,$5,$6,$7)
			`, p.CompanyID, in.SessionID, count.Currency, money.Num(exp), money.Num(counted), money.Num(variance), blankNull(count.Reason)); err != nil {
				return err
			}
			seen[count.Currency] = true
		}
		for currency := range expected {
			if !seen[currency] {
				return domain.Invalid("count", "Indiquez le montant compté pour chaque devise mouvementée.")
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE cash_sessions SET status = 'closed', closed_by = $2, closed_at = now() WHERE id = $1
		`, in.SessionID, p.UserID); err != nil {
			return err
		}
		return audit(ctx, tx, p, &storeID, "cash.close", "cash_session", in.SessionID, map[string]string{})
	})
}

func (s *Service) AdjustCash(ctx context.Context, p tenant.Principal, in CashAdjustmentInput) error {
	return s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		if in.Direction != "in" && in.Direction != "out" {
			return domain.Invalid("direction", "Le sens de la correction doit être une entrée ou une sortie.")
		}
		if !money.Round(in.Amount).IsPositive() || in.Reason == "" {
			return domain.Invalid("adjustment", "Indiquez un montant et le motif de la correction.")
		}
		var storeID uuid.UUID
		var status string
		var openedBy uuid.UUID
		if err := tx.QueryRow(ctx, `
			SELECT store_id, status, opened_by FROM cash_sessions WHERE id = $1 FOR UPDATE
		`, in.SessionID).Scan(&storeID, &status, &openedBy); err != nil {
			return domain.NotFound("Session de caisse introuvable.")
		}
		decision, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &storeID, access.CashClose)
		if err != nil {
			return err
		}
		if status != "open" {
			return domain.FailedPrecondition("cash_session", "La correction se fait sur une session ouverte. La session clôturée reste inchangée.")
		}
		if decision.Role == "cashier" && openedBy != p.UserID {
			return domain.PermissionDenied("cash_session", "Vous ne pouvez corriger que votre propre session.")
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO cash_transactions (
				id, company_id, session_id, currency, direction, kind, amount, ref_type, ref_id, created_by
			) VALUES ($1,$2,$3,$4,$5,'adjustment',$6,'cash_session',$7,$8)
		`, uuid.New(), p.CompanyID, in.SessionID, in.Currency, in.Direction, money.Num(money.Round(in.Amount)), in.ReferenceSessionID, p.UserID); err != nil {
			return err
		}
		return audit(ctx, tx, p, &storeID, "cash.adjust", "cash_transaction", in.SessionID, map[string]string{"reason": in.Reason})
	})
}

func (s *Service) CreateExpense(ctx context.Context, p tenant.Principal, in ExpenseInput) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		decision, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &in.StoreID, access.ExpensesCreate)
		if err != nil {
			return err
		}
		amount := money.Round(in.Amount)
		if in.Category == "" || !amount.IsPositive() || !validCurrency(in.Currency) {
			return domain.Invalid("expense", "Indiquez une catégorie, un montant et une devise.")
		}
		if in.PaidFromCash {
			if err := lockCashSession(ctx, tx, p, decision.Role, in.StoreID, in.CashSessionID, in.Currency); err != nil {
				return err
			}
		}
		id = uuid.New()
		var session any
		if in.PaidFromCash {
			session = *in.CashSessionID
		}
		var receipt any
		if in.MediaID != nil {
			if err := readyReceipt(ctx, tx, *in.MediaID); err != nil {
				return err
			}
			receipt = *in.MediaID
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO expenses (
				id, company_id, store_id, category, amount, currency, paid_from_cash, cash_session_id, media_id, note, created_by
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		`, id, p.CompanyID, in.StoreID, in.Category, money.Num(amount), in.Currency, in.PaidFromCash, session, receipt, blankNull(in.Note), p.UserID); err != nil {
			return err
		}
		if in.PaidFromCash {
			if _, err := tx.Exec(ctx, `
				INSERT INTO cash_transactions (
					id, company_id, session_id, currency, direction, kind, amount, ref_type, ref_id, created_by
				) VALUES ($1,$2,$3,$4,'out','expense',$5,'expense',$6,$7)
			`, uuid.New(), p.CompanyID, *in.CashSessionID, in.Currency, money.Num(amount), id, p.UserID); err != nil {
				return err
			}
		}
		return audit(ctx, tx, p, &in.StoreID, "expense.create", "expense", id, map[string]string{"category": in.Category})
	})
	return id, err
}
