package integration

import (
	"context"
	"testing"

	"rangoapp/internal/easystock/legacyimport"
	"rangoapp/internal/easystock/testutil"

	"github.com/shopspring/decimal"
)

func TestLegacyImportDryRunAndReplay(t *testing.T) {
	admin, _, _, _ := testutil.Pools(t)
	ctx := context.Background()
	bundle := legacyimport.Bundle{
		Companies: []legacyimport.Company{{LegacyID: "legacy-co", Name: "Import SA", Currency: "USD"}},
		Stores:    []legacyimport.Store{{LegacyID: "legacy-store", CompanyLegacyID: "legacy-co", Name: "Centre"}},
		Products:  []legacyimport.Product{{LegacyID: "legacy-prod", CompanyLegacyID: "legacy-co", SKU: "IMP-1", Name: "Savon", BaseUnit: "piece"}},
		Stocks:    []legacyimport.Stock{{CompanyLegacyID: "legacy-co", StoreLegacyID: "legacy-store", ProductLegacyID: "legacy-prod", Quantity: "4"}},
		Sales:     []legacyimport.Sale{{LegacyID: "legacy-sale", CompanyLegacyID: "legacy-co", StoreLegacyID: "legacy-store", Currency: "USD", Net: "15.5", Paid: "10"}},
	}
	rejected := bundle
	rejected.Stocks = append(rejected.Stocks, legacyimport.Stock{
		CompanyLegacyID: "legacy-co", StoreLegacyID: "legacy-store", ProductLegacyID: "legacy-prod", Quantity: "-1",
	})
	rejected.Sales = append(rejected.Sales, legacyimport.Sale{
		LegacyID: "legacy-overpay", CompanyLegacyID: "legacy-co", StoreLegacyID: "legacy-store", Currency: "USD", Net: "1", Paid: "5",
	})

	dry, err := legacyimport.Apply(ctx, admin, rejected, true)
	if err != nil {
		t.Fatal(err)
	}
	if !dry.DryRun || dry.Sales != 1 || len(dry.Anomalies) != 2 {
		t.Fatalf("dry-run report = %+v", dry)
	}
	if len(dry.Totals) != 1 || !dry.Totals[0].Match || dry.Totals[0].Source != "15.5" {
		t.Fatalf("totals = %+v", dry.Totals)
	}
	var companies int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM companies WHERE name = 'Import SA'`).Scan(&companies); err != nil {
		t.Fatal(err)
	}
	if companies != 0 {
		t.Fatalf("dry-run left %d companies", companies)
	}

	loaded, err := legacyimport.Apply(ctx, admin, bundle, false)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Sales != 1 || len(loaded.Totals) != 1 || !loaded.Totals[0].Match {
		t.Fatalf("load report = %+v", loaded)
	}
	again, err := legacyimport.Apply(ctx, admin, bundle, false)
	if err != nil {
		t.Fatal(err)
	}
	if again.Sales != 0 {
		t.Fatalf("replay inserted %d sales", again.Sales)
	}
	companyID := legacyimport.ID("companies", "legacy-co")
	var sales int
	var net string
	if err := admin.QueryRow(ctx, `SELECT count(*), COALESCE(SUM(net_amount),0)::text FROM sales WHERE company_id = $1`, companyID).Scan(&sales, &net); err != nil {
		t.Fatal(err)
	}
	got, err := decimal.NewFromString(net)
	if err != nil {
		t.Fatal(err)
	}
	if sales != 1 || !got.Equal(decimal.RequireFromString("15.5")) {
		t.Fatalf("sales=%d net=%s", sales, net)
	}
	if legacyimport.ID("companies", "legacy-co") != companyID {
		t.Fatal("legacy mapping is not reproducible")
	}
}
