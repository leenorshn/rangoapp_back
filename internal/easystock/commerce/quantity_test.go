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
