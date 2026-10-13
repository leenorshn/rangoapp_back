package commerce

import (
	"context"
	"errors"
	"strings"
	"time"

	"rangoapp/internal/easystock/access"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/tenant"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

// ExchangeRate is one company pair. The inverse is not stored.
type ExchangeRate struct {
	From      string
	To        string
	Rate      string
	UpdatedAt time.Time
}

// Conversion is an explicit amount in another currency. Currencies are never added together.
type Conversion struct {
	Amount       string
	Currency     string
	FromCurrency string
	Rate         string
	Basis        string
}

// ApplyRate multiplies or divides, then rounds the amount to the money scale.
// The inverse rate shown to the client is rounded to the rate scale afterwards,
// so the amount is not rounded twice.
func ApplyRate(amount, rate decimal.Decimal, inverse bool) (converted, applied decimal.Decimal) {
	if inverse {
		converted = money.Round(amount.Div(rate))
		return converted, decimal.NewFromInt(1).Div(rate).Round(money.RateScale)
	}
	return money.Round(amount.Mul(rate)), rate
}

func (s *Service) SetExchangeRate(ctx context.Context, p tenant.Principal, from, to string, rate decimal.Decimal) (ExchangeRate, error) {
	var out ExchangeRate
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, nil, access.RatesManage); err != nil {
			return err
		}
		from, to, rate, err := normalizeRate(from, to, rate)
		if err != nil {
			return err
		}
		var updated time.Time
		if err := tx.QueryRow(ctx, `
			INSERT INTO company_exchange_rates (company_id, from_currency, to_currency, rate)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (company_id, from_currency, to_currency)
			DO UPDATE SET rate = EXCLUDED.rate, updated_at = now()
			RETURNING updated_at
		`, p.CompanyID, from, to, money.Num(rate)).Scan(&updated); err != nil {
			return err
		}
		out = ExchangeRate{From: from, To: to, Rate: rate.StringFixed(money.RateScale), UpdatedAt: updated}
		return audit(ctx, tx, p, nil, "exchange_rate.set", "exchange_rate", p.CompanyID, map[string]string{
			"from": from, "to": to, "rate": out.Rate,
		})
	})
	return out, err
}

func (s *Service) ListExchangeRates(ctx context.Context, p tenant.Principal) ([]ExchangeRate, error) {
	var rowsOut []ExchangeRate
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := access.ActiveMember(ctx, tx, p.UserID); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			SELECT from_currency, to_currency, rate, updated_at
			FROM company_exchange_rates
			ORDER BY from_currency, to_currency
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var from, to string
			var rate pgtype.Numeric
			var updated time.Time
			if err := rows.Scan(&from, &to, &rate, &updated); err != nil {
				return err
			}
			parsed, err := money.FromNum(rate)
			if err != nil {
				return err
			}
			rowsOut = append(rowsOut, ExchangeRate{
				From: from, To: to, Rate: parsed.StringFixed(money.RateScale), UpdatedAt: updated,
			})
		}
		return rows.Err()
	})
	if rowsOut == nil {
		rowsOut = []ExchangeRate{}
	}
	return rowsOut, err
}

func (s *Service) Convert(ctx context.Context, p tenant.Principal, amount decimal.Decimal, from, to string) (Conversion, error) {
	var out Conversion
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := access.ActiveMember(ctx, tx, p.UserID); err != nil {
			return err
		}
		from = strings.ToUpper(strings.TrimSpace(from))
		to = strings.ToUpper(strings.TrimSpace(to))
		if !validCurrency(from) || !validCurrency(to) {
			return domain.Invalid("currency", "Indiquez deux codes devise sur trois lettres.")
		}
		amount = money.Round(amount)
		if amount.IsNegative() {
			return domain.Invalid("amount", "Le montant à convertir ne peut pas être négatif.")
		}
		if from == to {
			out = Conversion{
				Amount: amount.StringFixed(money.MoneyScale), Currency: to, FromCurrency: from,
				Rate: "1.00000000", Basis: "same",
			}
			return nil
		}
		direct, hasDirect, err := lookupRate(ctx, tx, from, to)
		if err != nil {
			return err
		}
		inverse, hasInverse, err := lookupRate(ctx, tx, to, from)
		if err != nil {
			return err
		}
		var converted, applied decimal.Decimal
		basis := "direct"
		switch {
		case hasDirect:
			converted, applied = ApplyRate(amount, direct, false)
		case hasInverse:
			converted, applied = ApplyRate(amount, inverse, true)
			basis = "inverse"
		default:
			return domain.FailedPrecondition("exchange_rate", "Aucun taux n'est défini entre ces devises. Enregistrez le taux avant de convertir.")
		}
		if !fitsMoney(converted) {
			return domain.Invalid("amount", "Le montant converti dépasse la précision enregistrable.")
		}
		out = Conversion{
			Amount: converted.StringFixed(money.MoneyScale), Currency: to, FromCurrency: from,
			Rate: applied.StringFixed(money.RateScale), Basis: basis,
		}
		return nil
	})
	return out, err
}

func normalizeRate(from, to string, rate decimal.Decimal) (string, string, decimal.Decimal, error) {
	from = strings.ToUpper(strings.TrimSpace(from))
	to = strings.ToUpper(strings.TrimSpace(to))
	if !validCurrency(from) || !validCurrency(to) {
		return "", "", decimal.Zero, domain.Invalid("currency", "Indiquez deux codes devise sur trois lettres.")
	}
	if from == to {
		return "", "", decimal.Zero, domain.Invalid("exchange_rate", "Le taux relie deux devises différentes.")
	}
	if !rate.IsPositive() {
		return "", "", decimal.Zero, domain.Invalid("exchange_rate", "Le taux doit être supérieur à zéro.")
	}
	if rate.Exponent() < -money.RateScale {
		return "", "", decimal.Zero, domain.Invalid("exchange_rate", "Le taux accepte au plus huit décimales.")
	}
	limit := decimal.NewFromInt(1_000_000_000_000)
	if !rate.LessThan(limit) {
		return "", "", decimal.Zero, domain.Invalid("exchange_rate", "Le taux dépasse la précision enregistrable.")
	}
	return from, to, rate, nil
}

func lookupRate(ctx context.Context, tx pgx.Tx, from, to string) (decimal.Decimal, bool, error) {
	var raw pgtype.Numeric
	err := tx.QueryRow(ctx, `
		SELECT rate FROM company_exchange_rates
		WHERE from_currency = $1 AND to_currency = $2
	`, from, to).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return decimal.Zero, false, nil
		}
		return decimal.Zero, false, err
	}
	parsed, err := money.FromNum(raw)
	if err != nil {
		return decimal.Zero, false, err
	}
	return parsed, true, nil
}

func fitsMoney(value decimal.Decimal) bool {
	limit := decimal.RequireFromString("10000000000000000")
	return value.Abs().LessThan(limit)
}
