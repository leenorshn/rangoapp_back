package integration

import (
	"context"
	"testing"

	"rangoapp/internal/easystock/api"
	"rangoapp/internal/easystock/catalog"
	"rangoapp/internal/easystock/commerce"
	"rangoapp/internal/easystock/identity"
	"rangoapp/internal/easystock/readmodel"
	"rangoapp/internal/easystock/testutil"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

func TestDensityLitreToKilogramAndBack(t *testing.T) {
	admin, app, identityPool, _ := testutil.Pools(t)
	w := shop(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	oil := oilProduct(t, svc, w, "Huile", "kg", "mass", "litre", "volume")
	litre := unitByName(t, oil, "litre")
	if oil.CrossDimensionEnabled || oil.DensityKgPerLiter != "" {
		t.Fatalf("density starts off: %+v", oil)
	}
	for _, unit := range oil.Units {
		if unit.Name == "kg" && unit.Dimension != "mass" {
			t.Fatalf("base dimension %s", unit.Dimension)
		}
		if unit.Name == "litre" && unit.Dimension != "volume" {
			t.Fatalf("litre dimension %s", unit.Dimension)
		}
	}
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "huile-sans-densite", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: oil.ID, ProductUnitID: litre, Quantity: decimal.NewFromInt(10)}},
	}); reasonOf(err) != "density" {
		t.Fatalf("missing density = %v", err)
	}
	if balances := balanceCount(t, admin, oil.ID); balances != 0 {
		t.Fatalf("stock rows %d", balances)
	}

	keeper := addMember(t, admin, w, "stockkeeper", w.store)
	if _, err := svc.SetProductDensity(ctx, keeper, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.92"), HasDensity: true, Enabled: true,
	}); reasonOf(err) != "catalog.manage" {
		t.Fatalf("stockkeeper = %v", err)
	}
	set, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.92"), HasDensity: true,
		Reference: "huile 20C", HasReference: true, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if set.DensityKgPerLiter != "0.92" || !set.CrossDimensionEnabled || set.DensityReference != "huile 20C" || set.DensityUpdatedBy != w.p.UserID || set.DensityUpdatedAt == nil {
		t.Fatalf("%+v", set)
	}
	opened, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "huile-10l", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: oil.ID, ProductUnitID: litre, Quantity: decimal.NewFromInt(10)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if opened.Lines[0].BaseQuantity != "9.2" {
		t.Fatalf("opened %+v", opened.Lines[0])
	}
	if got := stockQty(t, admin, w, oil.ID); !got.Equal(decimal.RequireFromString("9.2")) {
		t.Fatalf("stock %s", got)
	}
	assertMovementDensity(t, admin, oil.ID, "opening", "9.2", "0.92")

	sold, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "vente-1l", oil.ID, "litre", "1", "2"))
	if err != nil {
		t.Fatal(err)
	}
	if saleBaseQty(t, admin, sold.SaleID).String() != "0.92" {
		t.Fatalf("sale base %s", saleBaseQty(t, admin, sold.SaleID))
	}
	if got := stockQty(t, admin, w, oil.ID); !got.Equal(decimal.RequireFromString("8.28")) {
		t.Fatalf("after sale %s", got)
	}

	paint := oilProduct(t, svc, w, "Peinture", "litre", "volume", "kg", "mass")
	kg := unitByName(t, paint, "kg")
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: paint.ID, Density: decimal.RequireFromString("0.92"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	back, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "peinture-9-2", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: paint.ID, ProductUnitID: kg, Quantity: decimal.RequireFromString("9.2")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if back.Lines[0].BaseQuantity != "10" {
		t.Fatalf("kg to litre %+v", back.Lines[0])
	}
	if got := stockQty(t, admin, w, paint.ID); !got.Equal(decimal.NewFromInt(10)) {
		t.Fatalf("paint %s", got)
	}
	assertMovementDensity(t, admin, paint.ID, "opening", "10", "0.92")

	schema, err := api.Schema(api.Deps{Commerce: svc, Reads: readmodel.New(app), Identity: identity.New(identityPool)})
	if err != nil {
		t.Fatal(err)
	}
	view := api.Execute(api.WithPrincipal(ctx, w.p), schema, `mutation ($input: SetProductDensityInput!) {
		setProductDensity(input: $input) { densityKgPerLiter crossDimensionConversionEnabled units { name dimension } }
	}`, map[string]any{"input": map[string]any{
		"productId": oil.ID.String(), "densityKgPerLiter": "0.92", "enabled": true,
	}})
	if len(view.Errors) > 0 {
		t.Fatal(view.Errors)
	}
	body := view.Data.(map[string]any)["setProductDensity"].(map[string]any)
	if body["densityKgPerLiter"] != "0.92" || body["crossDimensionConversionEnabled"] != true {
		t.Fatalf("%v", body)
	}
	journal := api.Execute(api.WithPrincipal(ctx, w.p), schema, `query ($store: ID!, $product: ID!) {
		stockMovements(storeId: $store, productId: $product, type: "opening") { quantity densityKgPerLiter }
	}`, map[string]any{"store": w.store.String(), "product": oil.ID.String()})
	if len(journal.Errors) > 0 {
		t.Fatal(journal.Errors)
	}
	rows := journal.Data.(map[string]any)["stockMovements"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["densityKgPerLiter"] != "0.92" {
		t.Fatalf("journal %+v", rows)
	}
}

func TestDensityRejectsZeroNegativeAndDisabled(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := shop(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	oil := oilProduct(t, svc, w, "Huile", "kg", "mass", "litre", "volume")
	litre := unitByName(t, oil, "litre")
	zero := decimal.Zero
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{ProductID: oil.ID, Density: zero, HasDensity: true, Enabled: true}); reasonOf(err) != "density" {
		t.Fatalf("zero = %v", err)
	}
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{ProductID: oil.ID, Density: decimal.RequireFromString("-0.92"), HasDensity: true, Enabled: true}); reasonOf(err) != "density" {
		t.Fatalf("negative = %v", err)
	}
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{ProductID: oil.ID, Enabled: true}); reasonOf(err) != "density" {
		t.Fatalf("enabled without density = %v", err)
	}
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.9200001"), HasDensity: true, Enabled: true,
	}); reasonOf(err) != "density" {
		t.Fatalf("scale = %v", err)
	}
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.92"), HasDensity: true, Enabled: false,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "huile-coupee", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: oil.ID, ProductUnitID: litre, Quantity: decimal.NewFromInt(1)}},
	}); reasonOf(err) != "density" {
		t.Fatalf("disabled = %v", err)
	}
	if balances := balanceCount(t, admin, oil.ID); balances != 0 {
		t.Fatalf("stock rows %d", balances)
	}
}

func TestDensityChangeKeepsOldMovementsAndInventory(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := shop(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	oil := oilProduct(t, svc, w, "Huile", "kg", "mass", "litre", "volume")
	litre := unitByName(t, oil, "litre")
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.92"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "huile-avant", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: oil.ID, ProductUnitID: litre, Quantity: decimal.NewFromInt(10)}},
	}); err != nil {
		t.Fatal(err)
	}
	session, err := svc.StartStockInventory(ctx, w.p, commerce.StartInventoryInput{StoreID: w.store})
	if err != nil {
		t.Fatal(err)
	}
	counted, err := svc.RecordInventoryCount(ctx, w.p, commerce.RecordCountInput{
		InventoryID: session.ID, ProductID: oil.ID, ProductUnitID: litre, CountedQuantity: decimal.NewFromInt(10),
	})
	if err != nil || counted.CountedBaseTotal != "9.2" {
		t.Fatalf("%+v %v", counted, err)
	}
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.80"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	var countedBase string
	if err := admin.QueryRow(ctx, `SELECT base_quantity::text FROM inventory_count_lines WHERE inventory_id = $1`, session.ID).Scan(&countedBase); err != nil {
		t.Fatal(err)
	}
	if moneyMust(t, countedBase).String() != "9.2" {
		t.Fatalf("count rewritten %s", countedBase)
	}
	if _, err := svc.CompleteStockInventory(ctx, w.p, commerce.FinishInventoryInput{InventoryID: session.ID}); err != nil {
		t.Fatal(err)
	}
	if got := stockQty(t, admin, w, oil.ID); !got.Equal(decimal.RequireFromString("9.2")) {
		t.Fatalf("inventory rewrote stock %s", got)
	}
	assertMovementDensity(t, admin, oil.ID, "opening", "9.2", "0.92")
	if _, err := svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
		IdempotencyKey: "huile-apres", StoreID: w.store, Currency: "USD",
		Lines: []commerce.ReceiveLine{{ProductID: oil.ID, ProductUnitID: &litre, Quantity: decimal.NewFromInt(10), UnitCost: decimal.RequireFromString("0.80")}},
	}); err != nil {
		t.Fatal(err)
	}
	if got := stockQty(t, admin, w, oil.ID); !got.Equal(decimal.RequireFromString("17.2")) {
		t.Fatalf("new receipt %s", got)
	}
	assertMovementDensity(t, admin, oil.ID, "opening", "9.2", "0.92")
	assertMovementDensity(t, admin, oil.ID, "receive", "8", "0.8")
}

func TestDensityPrecisionRoundsOnce(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := shop(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	oil := oilProduct(t, svc, w, "Huile fine", "kg", "mass", "litre", "volume")
	litre := unitByName(t, oil, "litre")
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.92"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	qty := decimal.RequireFromString("1.23456789")
	for i, key := range []string{"precision-a", "precision-b"} {
		if _, err := svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
			IdempotencyKey: key, StoreID: w.store, Currency: "USD",
			Lines: []commerce.ReceiveLine{{ProductID: oil.ID, ProductUnitID: &litre, Quantity: qty, UnitCost: decimal.NewFromInt(1)}},
		}); err != nil {
			t.Fatalf("receipt %d %v", i, err)
		}
	}
	if got := stockQty(t, admin, w, oil.ID); got.String() != "2.271604" {
		t.Fatalf("sum of one rounding %s", got)
	}
	var n int
	if err := admin.QueryRow(ctx, `
		SELECT count(*) FROM stock_movements
		WHERE product_id = $1 AND quantity_delta = 1.135802 AND density_kg_per_liter = 0.92
	`, oil.ID).Scan(&n); err != nil || n != 2 {
		t.Fatalf("snapshots %d err %v", n, err)
	}
}

func TestCartonPieceSaleAndInventoryIgnoreDensity(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := shop(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	oil := oilProduct(t, svc, w, "Huile", "kg", "mass", "litre", "volume")
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.92"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	water := mustProduct(t, svc, w, catalog.CreateInput{
		Name: "Eau", BaseUnit: "bouteille", StoreID: w.store,
		Units: []catalog.UnitInput{{Name: "carton", Factor: decimal.NewFromInt(24), HasFactor: true}},
		Prices: []catalog.PriceInput{
			{StoreID: w.store, UnitName: "bouteille", Currency: "USD", Amount: amount("0.25")},
			{StoreID: w.store, UnitName: "carton", Currency: "USD", Amount: amount("5")},
		},
	})
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: water.ID, Density: decimal.RequireFromString("0.92"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	carton := unitByName(t, water, "carton")
	opened, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "eau-cartons", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: water.ID, ProductUnitID: carton, Quantity: decimal.NewFromInt(2)}},
	})
	if err != nil || opened.Lines[0].BaseQuantity != "48" {
		t.Fatalf("%+v %v", opened, err)
	}
	if _, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "vente-carton", water.ID, "carton", "1", "5")); err != nil {
		t.Fatal(err)
	}
	if got := stockQty(t, admin, w, water.ID); !got.Equal(decimal.NewFromInt(24)) {
		t.Fatalf("after sale %s", got)
	}
	var density *string
	if err := admin.QueryRow(ctx, `
		SELECT density_kg_per_liter::text FROM stock_movements WHERE product_id = $1 AND movement_type = 'sale'
	`, water.ID).Scan(&density); err != nil || density != nil {
		t.Fatalf("sale density %v err %v", density, err)
	}
	session, err := svc.StartStockInventory(ctx, w.p, commerce.StartInventoryInput{StoreID: w.store})
	if err != nil {
		t.Fatal(err)
	}
	counted, err := svc.RecordInventoryCount(ctx, w.p, commerce.RecordCountInput{
		InventoryID: session.ID, ProductID: water.ID, ProductUnitID: carton, CountedQuantity: decimal.NewFromInt(1),
	})
	if err != nil || counted.CountedBaseTotal != "24" {
		t.Fatalf("%+v %v", counted, err)
	}
	if _, err := svc.CompleteStockInventory(ctx, w.p, commerce.FinishInventoryInput{InventoryID: session.ID}); err != nil {
		t.Fatal(err)
	}
	if got := stockQty(t, admin, w, water.ID); !got.Equal(decimal.NewFromInt(24)) {
		t.Fatalf("inventory %s", got)
	}
}

func TestDensityRejectsIncompatibleDimension(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := shop(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	soap := mustProduct(t, svc, w, catalog.CreateInput{
		Name: "Savon", BaseUnit: "pièce", StoreID: w.store,
		Units: []catalog.UnitInput{{Name: "litre", Factor: decimal.NewFromInt(1), HasFactor: true, Dimension: "volume", HasDimension: true}},
		Prices: []catalog.PriceInput{
			{StoreID: w.store, UnitName: "pièce", Currency: "USD", Amount: amount("1")},
		},
	})
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: soap.ID, Density: decimal.RequireFromString("0.92"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	litre := unitByName(t, soap, "litre")
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "savon-litre", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: soap.ID, ProductUnitID: litre, Quantity: decimal.NewFromInt(1)}},
	}); reasonOf(err) != "dimension" {
		t.Fatalf("incompatible = %v", err)
	}
	if balances := balanceCount(t, admin, soap.ID); balances != 0 {
		t.Fatalf("stock rows %d", balances)
	}
	piece := unitByName(t, soap, "pièce")
	if _, err := svc.UpdateCatalogUnit(ctx, w.p, piece, catalog.UnitInput{Dimension: "mass", HasDimension: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "savon-piece", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(1)}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateCatalogUnit(ctx, w.p, piece, catalog.UnitInput{Dimension: "count", HasDimension: true}); reasonOf(err) != "dimension_locked" {
		t.Fatalf("locked = %v", err)
	}
}

func oilProduct(t *testing.T, svc *commerce.Service, w world, name, base, baseDimension, other, otherDimension string) catalog.Product {
	t.Helper()
	return mustProduct(t, svc, w, catalog.CreateInput{
		Name: name, BaseUnit: base, BaseDimension: baseDimension, AllowFractional: true, StoreID: w.store,
		Units: []catalog.UnitInput{{
			Name: other, Factor: decimal.NewFromInt(1), HasFactor: true, Dimension: otherDimension, HasDimension: true,
		}},
		Prices: []catalog.PriceInput{
			{StoreID: w.store, UnitName: base, Currency: "USD", Amount: amount("1")},
			{StoreID: w.store, UnitName: other, Currency: "USD", Amount: amount("2")},
		},
	})
}

func balanceCount(t *testing.T, admin *pgxpool.Pool, product uuid.UUID) int {
	t.Helper()
	var n int
	if err := admin.QueryRow(context.Background(), `SELECT count(*) FROM stock_balances WHERE product_id = $1`, product).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func assertMovementDensity(t *testing.T, admin *pgxpool.Pool, product uuid.UUID, movementType, delta, density string) {
	t.Helper()
	var gotDelta, gotDensity string
	err := admin.QueryRow(context.Background(), `
		SELECT quantity_delta::text, density_kg_per_liter::text
		FROM stock_movements
		WHERE product_id = $1 AND movement_type = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, product, movementType).Scan(&gotDelta, &gotDensity)
	if err != nil {
		t.Fatal(err)
	}
	if moneyMust(t, gotDelta).String() != delta || moneyMust(t, gotDensity).String() != density {
		t.Fatalf("%s delta %s density %s", movementType, gotDelta, gotDensity)
	}
}
