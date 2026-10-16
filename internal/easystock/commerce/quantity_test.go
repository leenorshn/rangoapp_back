package commerce

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestSoldQuantityAllowsWholePacks(t *testing.T) {
	if err := soldQuantityAllowed(false, decimal.RequireFromString("2")); err != nil {
		t.Fatal(err)
	}
	if err := soldQuantityAllowed(false, decimal.RequireFromString("2.0")); err != nil {
		t.Fatal(err)
	}
	if err := soldQuantityAllowed(false, decimal.RequireFromString("1.5")); err == nil {
		t.Fatal("fractional package was accepted")
	}
	if err := soldQuantityAllowed(true, decimal.RequireFromString("0.5")); err != nil {
		t.Fatal(err)
	}
}
