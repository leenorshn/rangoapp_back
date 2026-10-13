package commerce

import (
	"testing"

	"rangoapp/internal/easystock/money"

	"github.com/shopspring/decimal"
)

func TestApplyRateRoundsMoneyOnce(t *testing.T) {
	converted, applied := ApplyRate(money.Must("10"), money.Must("2800.5"), false)
	if converted.StringFixed(4) != "28005.0000" || applied.StringFixed(8) != "2800.50000000" {
		t.Fatalf("direct %s %s", converted, applied)
	}
	converted, applied = ApplyRate(money.Must("2800.5"), money.Must("2800.5"), true)
	if converted.StringFixed(4) != "1.0000" {
		t.Fatalf("inverse amount %s", converted)
	}
	one, _ := ApplyRate(money.Must("1"), money.Must("2800"), true)
	if one.StringFixed(4) != "0.0004" {
		t.Fatalf("small inverse %s", one)
	}
	if _, _, _, err := normalizeRate("usd", "USD", money.Must("1")); err == nil {
		t.Fatal("same pair accepted")
	}
	if _, _, _, err := normalizeRate("USD", "CDF", money.Must("1.123456789")); err == nil {
		t.Fatal("ninth decimal accepted")
	}
	if _, _, _, err := normalizeRate("USD", "CDF", decimal.Zero); err == nil {
		t.Fatal("zero rate accepted")
	}
}
