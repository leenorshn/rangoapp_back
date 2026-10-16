package commerce

import (
	"context"
	"strings"

	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/money"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// conversion is the product's kg/litre setting. Same-dimension packaging ignores it.
type conversion struct {
	baseDimension string
	enabled       bool
	density       decimal.Decimal
	ok            bool
}

// convertQuantity turns a packaging quantity into the product base unit.
// Same dimension stays quantity × factor, with no extra rounding.
// Mass and volume convert only when this product has a positive density and the switch is on.
// The result is rounded once, to six decimal places, half away from zero.
// The returned density is the value used for that conversion, so a later change cannot rewrite it.
func convertQuantity(qty, factor decimal.Decimal, unitDimension string, conv conversion) (decimal.Decimal, *decimal.Decimal, error) {
	unitDimension = normalizeDimension(unitDimension)
	baseDimension := normalizeDimension(conv.baseDimension)
	if unitDimension == baseDimension {
		return qty.Mul(factor), nil, nil
	}
	if !conv.enabled || !conv.ok || !conv.density.IsPositive() {
		return decimal.Zero, nil, domain.FailedPrecondition("density", "Cette conversion kg/litre exige une densité positive activée sur l'article.")
	}
	var raw decimal.Decimal
	switch {
	case baseDimension == "mass" && unitDimension == "volume":
		raw = qty.Mul(factor).Mul(conv.density)
	case baseDimension == "volume" && unitDimension == "mass":
		raw = qty.Mul(factor).Div(conv.density)
	default:
		return decimal.Zero, nil, domain.Invalid("dimension", "Cette unité ne se convertit pas vers l'unité de base.")
	}
	base := raw.Round(6)
	if !base.IsPositive() {
		return decimal.Zero, nil, domain.Invalid("quantity", "La quantité convertie est trop petite pour être stockée.")
	}
	used := conv.density
	return base, &used, nil
}

func normalizeDimension(value string) string {
	switch strings.TrimSpace(value) {
	case "mass", "volume":
		return strings.TrimSpace(value)
	default:
		return "count"
	}
}

func loadConversion(ctx context.Context, tx pgx.Tx, productID uuid.UUID) (conversion, error) {
	var baseDimension string
	var enabled bool
	var densityText *string
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(b.dimension, 'count'),
		       p.cross_dimension_conversion_enabled,
		       p.density_kg_per_liter::text
		FROM products p
		JOIN product_units b ON b.product_id = p.id AND b.is_base
		WHERE p.id = $1
	`, productID).Scan(&baseDimension, &enabled, &densityText)
	if err != nil {
		return conversion{}, err
	}
	conv := conversion{baseDimension: normalizeDimension(baseDimension), enabled: enabled}
	if densityText != nil && strings.TrimSpace(*densityText) != "" {
		parsed, err := decimal.NewFromString(*densityText)
		if err != nil {
			return conversion{}, err
		}
		conv.density = parsed
		conv.ok = parsed.IsPositive()
	}
	return conv, nil
}

func densityArg(value *decimal.Decimal) any {
	if value == nil {
		return nil
	}
	return money.Num(*value)
}

func factorArg(value *decimal.Decimal) any {
	if value == nil || !value.IsPositive() {
		return nil
	}
	return money.Num(*value)
}

// refundBase restores a share of the sale's stored base quantity.
// The last refund of the line receives the remainder, so successive partials sum to the original base.
// Earlier refunds are rounded once, to six decimal places, half away from zero.
func refundBase(soldQty, soldBase, prevQty, prevBase, qty decimal.Decimal) decimal.Decimal {
	if prevQty.Add(qty).Equal(soldQty) {
		return soldBase.Sub(prevBase)
	}
	return soldBase.Mul(qty).Div(soldQty).Round(6)
}

// conversionExplained reports whether the stored factor and density reproduce the base quantity.
// A missing density is valid only when quantity × factor already equals the base.
// Comparison uses the six-decimal scale stored on stock rows.
// Neither value is taken from the product's current settings.
func conversionExplained(qty, base decimal.Decimal, factor, density *decimal.Decimal) bool {
	if factor == nil || !factor.IsPositive() || !qty.IsPositive() || !base.IsPositive() {
		return false
	}
	stored := base.Round(6)
	scaled := qty.Mul(*factor).Round(6)
	if density == nil {
		return scaled.Equal(stored)
	}
	if !density.IsPositive() {
		return false
	}
	return scaled.Mul(*density).Round(6).Equal(stored) || scaled.Div(*density).Round(6).Equal(stored)
}

// soldQuantityAllowed checks the quantity of the chosen packaging.
// Two cartons stay valid when the product forbids fractions of its base unit.
// Only a fractional number of packages is rejected in that case.
func soldQuantityAllowed(allowFractional bool, quantity decimal.Decimal) error {
	if allowFractional || quantity.Equal(quantity.Truncate(0)) {
		return nil
	}
	return domain.Invalid("quantity", "Cet article n'accepte pas les quantités fractionnaires.")
}
