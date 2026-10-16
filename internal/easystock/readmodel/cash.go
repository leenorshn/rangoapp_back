package readmodel

import (
	"context"
	"errors"

	"rangoapp/internal/easystock/access"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// CashSession loads one drawer session. Amounts stay hidden when the caller
// neither opened it nor can read reports.
func (s *Service) CashSession(ctx context.Context, p tenant.Principal, id uuid.UUID) (CashSession, error) {
	var out CashSession
	err := s.with(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		session, err := loadCashSession(ctx, tx, p, id)
		if err != nil {
			return err
		}
		if err := attachCashFacts(ctx, tx, []*CashSession{&session}); err != nil {
			return err
		}
		out = session
		return nil
	})
	return out, err
}

// CashSessions lists drawer sessions of one store, newest first.
func (s *Service) CashSessions(ctx context.Context, p tenant.Principal, storeID uuid.UUID, status string, page Page) ([]CashSession, error) {
	if status != "" && status != "open" && status != "closed" {
		return nil, domain.Invalid("status", "Le statut de session doit être open ou closed.")
	}
	page, err := page.normalize()
	if err != nil {
		return nil, err
	}
	var out []CashSession
	err = s.with(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		decision, err := authorizeFirst(ctx, tx, p, &storeID, access.ReportsRead, access.CashOpen, access.CashClose)
		if err != nil {
			return err
		}
		if err := requireStore(ctx, tx, storeID); err != nil {
			return err
		}
		seeAll := access.RoleAllows(decision.Role, access.ReportsRead)
		paged := !page.After.IsZero()
		rows, err := tx.Query(ctx, `
			SELECT id, store_id, register_id, opened_by, closed_by, status, opened_at, closed_at, close_note
			FROM cash_sessions
			WHERE store_id = $1
			  AND ($2 = '' OR status = $2)
			  AND (NOT $3 OR (opened_at, id) < ($4, $5))
			ORDER BY opened_at DESC, id DESC
			LIMIT $6
		`, storeID, status, paged, page.After, page.AfterID, page.Limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			session, err := scanCashSession(rows.Scan)
			if err != nil {
				return err
			}
			session.AmountsOK = seeAll || session.OpenedBy == p.UserID
			if !session.AmountsOK {
				session.Note = ""
			}
			out = append(out, session)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		ptrs := make([]*CashSession, len(out))
		for i := range out {
			ptrs[i] = &out[i]
		}
		return attachCashFacts(ctx, tx, ptrs)
	})
	return out, err
}

// CashTransactions lists movements of one session in chronological order.
func (s *Service) CashTransactions(ctx context.Context, p tenant.Principal, sessionID uuid.UUID, page Page) ([]CashMovement, error) {
	page, err := page.normalize()
	if err != nil {
		return nil, err
	}
	var out []CashMovement
	err = s.with(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		session, err := loadCashSession(ctx, tx, p, sessionID)
		if err != nil {
			return err
		}
		if !session.AmountsOK {
			return domain.PermissionDenied("cash_session", "Les montants de cette session ne vous sont pas visibles.")
		}
		paged := !page.After.IsZero()
		rows, err := tx.Query(ctx, `
			SELECT id, currency, direction, kind, amount, created_by, created_at
			FROM cash_transactions
			WHERE session_id = $1
			  AND (NOT $2 OR (created_at, id) > ($3, $4))
			ORDER BY created_at, id
			LIMIT $5
		`, sessionID, paged, page.After, page.AfterID, page.Limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			movement, err := scanMovement(rows.Scan)
			if err != nil {
				return err
			}
			out = append(out, movement)
		}
		return rows.Err()
	})
	return out, err
}

func loadCashSession(ctx context.Context, tx pgx.Tx, p tenant.Principal, id uuid.UUID) (CashSession, error) {
	row := tx.QueryRow(ctx, `
		SELECT id, store_id, register_id, opened_by, closed_by, status, opened_at, closed_at, close_note
		FROM cash_sessions WHERE id = $1
	`, id)
	session, err := scanCashSession(row.Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return CashSession{}, domain.NotFound("Session de caisse introuvable.")
	}
	if err != nil {
		return CashSession{}, err
	}
	decision, err := authorizeFirst(ctx, tx, p, &session.StoreID, access.ReportsRead, access.CashOpen, access.CashClose)
	if err != nil {
		return CashSession{}, err
	}
	session.AmountsOK = access.RoleAllows(decision.Role, access.ReportsRead) || session.OpenedBy == p.UserID
	if !session.AmountsOK {
		session.Note = ""
	}
	return session, nil
}

func scanCashSession(scan func(dest ...any) error) (CashSession, error) {
	var session CashSession
	var note *string
	if err := scan(
		&session.ID, &session.StoreID, &session.RegisterID, &session.OpenedBy, &session.ClosedBy,
		&session.Status, &session.OpenedAt, &session.ClosedAt, &note,
	); err != nil {
		return CashSession{}, err
	}
	if note != nil {
		session.Note = *note
	}
	return session, nil
}

func attachCashFacts(ctx context.Context, tx pgx.Tx, sessions []*CashSession) error {
	ids := make([]uuid.UUID, 0, len(sessions))
	byID := make(map[uuid.UUID]*CashSession, len(sessions))
	for _, session := range sessions {
		if session == nil || !session.AmountsOK {
			continue
		}
		session.Opening = []Money{}
		session.TotalIn = []Money{}
		session.TotalOut = []Money{}
		session.Balances = []Money{}
		session.Counted = []Money{}
		session.Variances = []Money{}
		session.Transactions = []CashMovement{}
		ids = append(ids, session.ID)
		byID[session.ID] = session
	}
	if len(ids) == 0 {
		return nil
	}
	totals, err := tx.Query(ctx, `
		SELECT session_id, currency,
		       COALESCE(SUM(amount) FILTER (WHERE kind = 'opening'), 0),
		       COALESCE(SUM(amount) FILTER (WHERE direction = 'in' AND kind <> 'opening'), 0),
		       COALESCE(SUM(amount) FILTER (WHERE direction = 'out'), 0),
		       COALESCE(SUM(CASE WHEN direction = 'in' THEN amount ELSE -amount END), 0)
		FROM cash_transactions
		WHERE session_id = ANY($1)
		GROUP BY session_id, currency
		ORDER BY session_id, currency
	`, ids)
	if err != nil {
		return err
	}
	defer totals.Close()
	for totals.Next() {
		var sessionID uuid.UUID
		var currency string
		var opening, inbound, outbound, theoretical pgtype.Numeric
		if err := totals.Scan(&sessionID, &currency, &opening, &inbound, &outbound, &theoretical); err != nil {
			return err
		}
		session := byID[sessionID]
		if session == nil {
			continue
		}
		if err := appendMoney(&session.Opening, currency, opening); err != nil {
			return err
		}
		if err := appendMoney(&session.TotalIn, currency, inbound); err != nil {
			return err
		}
		if err := appendMoney(&session.TotalOut, currency, outbound); err != nil {
			return err
		}
		if err := appendMoney(&session.Balances, currency, theoretical); err != nil {
			return err
		}
	}
	if err := totals.Err(); err != nil {
		return err
	}

	counts, err := tx.Query(ctx, `
		SELECT session_id, currency, counted_amount, variance
		FROM cash_session_counts
		WHERE session_id = ANY($1)
		ORDER BY session_id, currency
	`, ids)
	if err != nil {
		return err
	}
	defer counts.Close()
	for counts.Next() {
		var sessionID uuid.UUID
		var currency string
		var counted, variance pgtype.Numeric
		if err := counts.Scan(&sessionID, &currency, &counted, &variance); err != nil {
			return err
		}
		session := byID[sessionID]
		if session == nil {
			continue
		}
		if err := appendMoney(&session.Counted, currency, counted); err != nil {
			return err
		}
		if err := appendMoney(&session.Variances, currency, variance); err != nil {
			return err
		}
	}
	if err := counts.Err(); err != nil {
		return err
	}

	movements, err := tx.Query(ctx, `
		SELECT id, session_id, currency, direction, kind, amount, created_by, created_at
		FROM cash_transactions
		WHERE session_id = ANY($1)
		ORDER BY session_id, created_at, id
	`, ids)
	if err != nil {
		return err
	}
	defer movements.Close()
	for movements.Next() {
		var sessionID uuid.UUID
		var movement CashMovement
		var amount pgtype.Numeric
		if err := movements.Scan(
			&movement.ID, &sessionID, &movement.Currency, &movement.Direction, &movement.Kind,
			&amount, &movement.CreatedBy, &movement.CreatedAt,
		); err != nil {
			return err
		}
		movement.Amount, err = numText(amount)
		if err != nil {
			return err
		}
		if session := byID[sessionID]; session != nil {
			session.Transactions = append(session.Transactions, movement)
		}
	}
	return movements.Err()
}

func scanMovement(scan func(dest ...any) error) (CashMovement, error) {
	var movement CashMovement
	var amount pgtype.Numeric
	if err := scan(
		&movement.ID, &movement.Currency, &movement.Direction, &movement.Kind,
		&amount, &movement.CreatedBy, &movement.CreatedAt,
	); err != nil {
		return CashMovement{}, err
	}
	text, err := numText(amount)
	if err != nil {
		return CashMovement{}, err
	}
	movement.Amount = text
	return movement, nil
}

func appendMoney(dst *[]Money, currency string, value pgtype.Numeric) error {
	text, err := numText(value)
	if err != nil {
		return err
	}
	*dst = append(*dst, Money{Currency: currency, Amount: text})
	return nil
}
