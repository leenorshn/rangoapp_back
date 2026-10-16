package commerce

import (
	"testing"

	"rangoapp/internal/easystock/domain"

	"github.com/shopspring/decimal"
)

func TestConvertQuantitySameDimensionDoesNotRound(t *testing.T) {
	base, used, err := convertQuantity(decimal.RequireFromString("2"), decimal.RequireFromString("24"), "count", conversion{baseDimension: "count"})
	if err != nil {
		t.Fatal(err)
	}
	if !base.Equal(decimal.NewFromInt(48)) || used != nil {
		t.Fatalf("base %s density %v", base, used)
	}
}

func TestConvertQuantityOilDensity(t *testing.T) {
	density := decimal.RequireFromString("0.92")
	conv := conversion{baseDimension: "mass", enabled: true, density: density, ok: true}
	kg, used, err := convertQuantity(decimal.NewFromInt(10), decimal.NewFromInt(1), "volume", conv)
	if err != nil || !kg.Equal(decimal.RequireFromString("9.2")) || used == nil || !used.Equal(density) {
		t.Fatalf("litre to kg %s %v %v", kg, used, err)
	}
	back := conversion{baseDimension: "volume", enabled: true, density: density, ok: true}
	litres, _, err := convertQuantity(decimal.RequireFromString("9.2"), decimal.NewFromInt(1), "mass", back)
	if err != nil || !litres.Equal(decimal.NewFromInt(10)) {
		t.Fatalf("kg to litre %s %v", litres, err)
	}
}

func TestConvertQuantityRoundsOnce(t *testing.T) {
	conv := conversion{baseDimension: "mass", enabled: true, density: decimal.RequireFromString("0.92"), ok: true}
	base, _, err := convertQuantity(decimal.RequireFromString("1.23456789"), decimal.NewFromInt(1), "volume", conv)
	if err != nil {
		t.Fatal(err)
	}
	if base.String() != "1.135802" {
		t.Fatalf("rounded %s", base)
	}
}

func TestConvertQuantityRefusesMissingDensityAndIncompatibleDimension(t *testing.T) {
	_, _, err := convertQuantity(decimal.NewFromInt(1), decimal.NewFromInt(1), "volume", conversion{baseDimension: "mass"})
	if got := reason(err); got != "density" {
		t.Fatalf("density refusal %s", got)
	}
	conv := conversion{baseDimension: "count", enabled: true, density: decimal.RequireFromString("0.92"), ok: true}
	_, _, err = convertQuantity(decimal.NewFromInt(1), decimal.NewFromInt(1), "volume", conv)
	if got := reason(err); got != "dimension" {
		t.Fatalf("dimension refusal %s", got)
	}
	tiny := conversion{baseDimension: "mass", enabled: true, density: decimal.RequireFromString("0.000001"), ok: true}
	_, _, err = convertQuantity(decimal.RequireFromString("0.0000001"), decimal.NewFromInt(1), "volume", tiny)
	if got := reason(err); got != "quantity" {
		t.Fatalf("tiny quantity %s", got)
	}
}

func reason(err error) string {
	parsed, ok := domain.As(err)
	if !ok {
		return ""
	}
	return parsed.Reason
}

func TestRefundBaseKeepsRemainderOnTheLastSlice(t *testing.T) {
	soldQty := decimal.NewFromInt(3)
	soldBase := decimal.NewFromInt(1)
	first := refundBase(soldQty, soldBase, decimal.Zero, decimal.Zero, decimal.NewFromInt(1))
	second := refundBase(soldQty, soldBase, decimal.NewFromInt(1), first, decimal.NewFromInt(1))
	third := refundBase(soldQty, soldBase, decimal.NewFromInt(2), first.Add(second), decimal.NewFromInt(1))
	if first.String() != "0.333333" || second.String() != "0.333333" || third.String() != "0.333334" {
		t.Fatalf("slices %s %s %s", first, second, third)
	}
	if !first.Add(second).Add(third).Equal(soldBase) {
		t.Fatalf("sum %s", first.Add(second).Add(third))
	}
}

func TestConversionExplainedRequiresHistoricalProof(t *testing.T) {
	qty := decimal.NewFromInt(2)
	factor := decimal.NewFromInt(12)
	density := decimal.RequireFromString("0.92")
	base := decimal.RequireFromString("22.08")
	if !conversionExplained(qty, base, &factor, &density) {
		t.Fatal("litres to kilograms")
	}
	litres := decimal.NewFromInt(10)
	if !conversionExplained(decimal.RequireFromString("9.2"), litres, factorPtr(1), &density) {
		t.Fatal("kilograms to litres")
	}
	if !conversionExplained(decimal.NewFromInt(2), decimal.NewFromInt(48), factorPtr(24), nil) {
		t.Fatal("same dimension")
	}
	if conversionExplained(qty, base, &factor, nil) {
		t.Fatal("density was required")
	}
	if conversionExplained(qty, base, nil, &density) {
		t.Fatal("factor was required")
	}
	wide := decimal.RequireFromString("1.0000004")
	one := decimal.NewFromInt(1)
	if !conversionExplained(wide, wide, &one, nil) {
		t.Fatal("six decimal stock scale")
	}
}

func factorPtr(value int64) *decimal.Decimal {
	parsed := decimal.NewFromInt(value)
	return &parsed
}
