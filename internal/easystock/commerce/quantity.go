package commerce

import (
	"rangoapp/internal/easystock/domain"

	"github.com/shopspring/decimal"
)

// soldQuantityAllowed checks the quantity of the chosen packaging.
// Two cartons stay valid when the product forbids fractions of its base unit.
// Only a fractional number of packages is rejected in that case.
func soldQuantityAllowed(allowFractional bool, quantity decimal.Decimal) error {
	if allowFractional || quantity.Equal(quantity.Truncate(0)) {
		return nil
	}
	return domain.Invalid("quantity", "Cet article n'accepte pas les quantités fractionnaires.")
}
