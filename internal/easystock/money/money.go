package money

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

const (
	MoneyScale int32 = 4
	RateScale  int32 = 8
)

// Num encodes an exact decimal for a PostgreSQL NUMERIC parameter.
func Num(d decimal.Decimal) pgtype.Numeric {
	var n pgtype.Numeric
	if err := n.Scan(d.String()); err != nil {
		panic(fmt.Sprintf("numeric encode: %v", err))
	}
	return n
}

// FromNum decodes a NUMERIC without using binary floating point.
func FromNum(n pgtype.Numeric) (decimal.Decimal, error) {
	if !n.Valid {
		return decimal.Zero, nil
	}
	value, err := n.Value()
	if err != nil {
		return decimal.Zero, err
	}
	text, ok := value.(string)
	if !ok || text == "" {
		return decimal.Zero, fmt.Errorf("numeric value %T", value)
	}
	return decimal.NewFromString(text)
}

// Round halves away via shopspring bankers? shopspring Round is half away from zero
// for positive commercial amounts, which is the rule used for stored money.
func Round(d decimal.Decimal) decimal.Decimal {
	return d.Round(MoneyScale)
}

func Must(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}
