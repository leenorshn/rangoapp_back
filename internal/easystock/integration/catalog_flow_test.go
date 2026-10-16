package integration

import (
	"context"
	"testing"
	"time"

	"rangoapp/internal/easystock/api"
	"rangoapp/internal/easystock/catalog"
	"rangoapp/internal/easystock/commerce"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/identity"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/readmodel"
	"rangoapp/internal/easystock/tenant"
	"rangoapp/internal/easystock/testutil"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

func amount(raw string) *decimal.Decimal {
	value := money.Must(raw)
	return &value
}

func reasonOf(err error) string {
	de, ok := domain.As(err)
	if !ok {
		return ""
	}
	return de.Reason
}

func shop(t *testing.T, admin *pgxpool.Pool) world {
	t.Helper()
	w := seed(t, admin)
	ctx := context.Background()
	if _, err := admin.Exec(ctx, `DELETE FROM stock_balances WHERE company_id = $1`, w.company); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `DELETE FROM product_prices WHERE company_id = $1`, w.company); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `DELETE FROM product_units WHERE company_id = $1`, w.company); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `DELETE FROM products WHERE company_id = $1`, w.company); err != nil {
		t.Fatal(err)
	}
	return w
}

func TestCatalogCreateUpdateArchiveAndDuplicates(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := shop(t, admin)
	svc := commerce.New(app)
	ctx := context.Background()
	created, err := svc.CreateCatalogProduct(ctx, w.p, catalog.CreateInput{
		Name: "Eau minérale", BaseUnit: "bouteille", CategoryName: "Boissons",
		Description: "50 cl", StoreID: w.store,
		Prices: []catalog.PriceInput{{
			StoreID: w.store, UnitName: "bouteille", Currency: "USD", Amount: amount("500"),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.SKU == "" || created.Category != "Boissons" || len(created.Units) != 1 || !created.Units[0].IsBase {
		t.Fatalf("%+v", created)
	}
	if created.Units[0].Factor != "1" || created.Units[0].Prices[0].UnitPrice != "500" || created.Units[0].Prices[0].Currency != "USD" {
		t.Fatalf("%+v", created.Units[0])
	}
	renamed, err := svc.UpdateCatalogProduct(ctx, w.p, created.ID, catalog.UpdateInput{
		Name: strPtr("Eau plate"), AlertThreshold: amount("3"),
	})
	if err != nil || renamed.Name != "Eau plate" || renamed.AlertThreshold != "3" || renamed.BaseUnit != "bouteille" {
		t.Fatalf("%+v %v", renamed, err)
	}
	archived, err := svc.ArchiveCatalogProduct(ctx, w.p, created.ID)
	if err != nil || !archived.Archived || archived.Status != "archived" {
		t.Fatalf("%+v %v", archived, err)
	}

	if _, err := svc.CreateCatalogProduct(ctx, w.p, catalog.CreateInput{SKU: "SAME", Name: "Un", BaseUnit: "pièce"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateCatalogProduct(ctx, w.p, catalog.CreateInput{SKU: "SAME", Name: "Deux", BaseUnit: "pièce"}); reasonOf(err) != "already_exists" {
		t.Fatalf("sku = %v", err)
	}
	if _, err := svc.CreateCatalogProduct(ctx, w.p, catalog.CreateInput{Name: "Scan", BaseUnit: "pièce", Barcode: "111"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateCatalogProduct(ctx, w.p, catalog.CreateInput{Name: "Scan 2", BaseUnit: "pièce", Barcode: "111"}); reasonOf(err) != "already_exists" {
		t.Fatalf("barcode = %v", err)
	}
	if _, err := svc.CreateCatalogUnit(ctx, w.p, created.ID, catalog.UnitInput{Name: "pack", Factor: decimal.Zero, HasFactor: true}); reasonOf(err) != "product_archived" {
		t.Fatalf("archived unit = %v", err)
	}
	active, err := svc.CreateCatalogProduct(ctx, w.p, catalog.CreateInput{Name: "Riz", BaseUnit: "kg"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateCatalogUnit(ctx, w.p, active.ID, catalog.UnitInput{Name: "sachet", Factor: decimal.RequireFromString("-1"), HasFactor: true}); reasonOf(err) != "factor" {
		t.Fatalf("negative factor = %v", err)
	}
}

func TestCatalogPricesStoresAndSale(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := shop(t, admin)
	ctx := context.Background()
	other := uuid.New()
	if _, err := admin.Exec(ctx, `INSERT INTO stores (id, company_id, name) VALUES ($1,$2,'Nord')`, other, w.company); err != nil {
		t.Fatal(err)
	}
	svc := commerce.New(app)
	created, err := svc.CreateCatalogProduct(ctx, w.p, catalog.CreateInput{
		Name: "Eau", BaseUnit: "bouteille", StoreID: w.store,
		Units: []catalog.UnitInput{{Name: "carton", Factor: decimal.NewFromInt(24), HasFactor: true}},
		Prices: []catalog.PriceInput{
			{StoreID: w.store, UnitName: "bouteille", Currency: "USD", Amount: amount("500")},
			{StoreID: w.store, UnitName: "carton", Currency: "USD", Amount: amount("10000")},
			{StoreID: other, UnitName: "bouteille", Currency: "USD", Amount: amount("700")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	cdf, err := svc.UpsertCatalogPrice(ctx, w.p, catalog.PriceInput{
		StoreID: w.store, ProductID: created.ID, UnitName: "bouteille", Currency: "CDF", Amount: amount("1500"),
	})
	if err != nil || cdf.Currency != "CDF" || cdf.UnitPrice != "1500" {
		t.Fatalf("%+v %v", cdf, err)
	}
	if _, err := svc.ConfirmSale(ctx, w.p, commerce.ConfirmSaleInput{
		IdempotencyKey: "cdf-without-carton", StoreID: w.store, Currency: "CDF",
		Lines: []commerce.SaleLineInput{{
			ProductID: created.ID, UnitName: "carton", Quantity: money.Must("1"), ExpectedUnitPrice: money.Must("1"),
		}},
		Payments: []commerce.PaymentInput{{Method: "external", Amount: money.Must("1")}},
	}); reasonOf(err) != "not_found" {
		t.Fatalf("cdf carton = %v", err)
	}
	if _, err := svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
		IdempotencyKey: "recv-eau", StoreID: w.store, SupplierName: "Dépôt", Currency: "USD",
		Lines: []commerce.ReceiveLine{{ProductID: created.ID, UnitName: "bouteille", Quantity: money.Must("50"), UnitCost: money.Must("100")}},
	}); err != nil {
		t.Fatal(err)
	}
	bottle, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "sell-bottle", created.ID, "bouteille", "1", "500"))
	if err != nil || bottle.Net != "500" {
		t.Fatalf("%+v %v", bottle, err)
	}
	carton, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "sell-carton", created.ID, "carton", "2", "10000"))
	if err != nil || carton.Net != "20000" {
		t.Fatalf("%+v %v", carton, err)
	}
	if got := stockQty(t, admin, w, created.ID); !got.Equal(money.Must("1")) {
		t.Fatalf("stock = %s", got)
	}
	if got := saleBaseQty(t, admin, carton.SaleID); !got.Equal(decimal.NewFromInt(48)) {
		t.Fatalf("base = %s", got)
	}
	if _, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "fraction", created.ID, "bouteille", "1.5", "500")); reasonOf(err) != "quantity" {
		t.Fatalf("fraction = %v", err)
	}
	otherSale, err := svc.ConfirmSale(ctx, w.p, commerce.ConfirmSaleInput{
		IdempotencyKey: "other-store-price", StoreID: other, Currency: "USD",
		Lines: []commerce.SaleLineInput{{
			ProductID: created.ID, UnitName: "bouteille", Quantity: money.Must("1"), ExpectedUnitPrice: money.Must("500"),
		}},
		Payments: []commerce.PaymentInput{{Method: "external", Amount: money.Must("500")}},
	})
	if reasonOf(err) != "price_changed" {
		t.Fatalf("other store = %+v %v", otherSale, err)
	}
	if _, err := svc.UpsertCatalogPrice(ctx, w.p, catalog.PriceInput{
		StoreID: w.store, ProductID: created.ID, UnitName: "bouteille", Currency: "USD", Amount: amount("600"),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "stale-price", created.ID, "bouteille", "1", "500")); reasonOf(err) != "price_changed" {
		t.Fatalf("price change = %v", err)
	}
	if _, err := svc.UpdateCatalogProduct(ctx, w.p, created.ID, catalog.UpdateInput{AllowFractional: boolPtr(true)}); err != nil {
		t.Fatal(err)
	}
	half, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "half", created.ID, "bouteille", "0.5", "600"))
	if err != nil || half.Net != "300" {
		t.Fatalf("%+v %v", half, err)
	}
	cartonUnit := unitByName(t, created, "carton")
	if _, err := svc.UpdateCatalogUnit(ctx, w.p, cartonUnit, catalog.UnitInput{Name: "carton", Factor: decimal.NewFromInt(12), HasFactor: true}); reasonOf(err) != "factor_locked" {
		t.Fatalf("factor lock = %v", err)
	}
	if got := saleBaseQty(t, admin, carton.SaleID); !got.Equal(decimal.NewFromInt(48)) {
		t.Fatalf("history changed to %s", got)
	}
	if _, err := svc.ArchiveCatalogProduct(ctx, w.p, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "archived", created.ID, "bouteille", "0.5", "600")); reasonOf(err) != "product_archived" {
		t.Fatalf("archived = %v", err)
	}

	empty, err := svc.CreateCatalogProduct(ctx, w.p, catalog.CreateInput{
		Name: "Vide", BaseUnit: "pièce", StoreID: w.store,
		Prices: []catalog.PriceInput{{StoreID: w.store, UnitName: "pièce", Currency: "USD", Amount: amount("10")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "no-stock", empty.ID, "pièce", "1", "10")); reasonOf(err) != "stock_conflict" {
		t.Fatalf("no stock = %v", err)
	}
	rice, err := svc.CreateCatalogProduct(ctx, w.p, catalog.CreateInput{
		Name: "Riz", BaseUnit: "kg", StoreID: w.store,
		Units:  []catalog.UnitInput{{Name: "sachet", Factor: decimal.RequireFromString("1.5"), HasFactor: true}},
		Prices: []catalog.PriceInput{{StoreID: w.store, UnitName: "sachet", Currency: "USD", Amount: amount("3")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
		IdempotencyKey: "recv-rice", StoreID: w.store, SupplierName: "Dépôt", Currency: "USD",
		Lines: []commerce.ReceiveLine{{ProductID: rice.ID, UnitName: "kg", Quantity: money.Must("3"), UnitCost: money.Must("1")}},
	}); err != nil {
		t.Fatal(err)
	}
	packed, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "rice-pack", rice.ID, "sachet", "2", "3"))
	if err != nil || packed.Net != "6" {
		t.Fatalf("%+v %v", packed, err)
	}
	if got := stockQty(t, admin, w, rice.ID); !got.Equal(decimal.Zero) {
		t.Fatalf("rice stock = %s", got)
	}
	short, err := svc.CreateCatalogProduct(ctx, w.p, catalog.CreateInput{
		Name: "Court", BaseUnit: "pièce", StoreID: w.store,
		Prices: []catalog.PriceInput{{StoreID: w.store, UnitName: "pièce", Currency: "USD", Amount: amount("2")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
		INSERT INTO stock_balances (company_id, store_id, product_id, quantity, cost_complete)
		VALUES ($1,$2,$3,1,false)
	`, w.company, w.store, short.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "short", short.ID, "pièce", "2", "2")); reasonOf(err) != "stock_conflict" {
		t.Fatalf("short = %v", err)
	}
}

func TestCatalogSyncIsStoreScopedAndIsolated(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := shop(t, admin)
	ctx := context.Background()
	other := uuid.New()
	if _, err := admin.Exec(ctx, `INSERT INTO stores (id, company_id, name) VALUES ($1,$2,'Est')`, other, w.company); err != nil {
		t.Fatal(err)
	}
	svc := commerce.New(app)
	created, err := svc.CreateCatalogProduct(ctx, w.p, catalog.CreateInput{
		Name: "Savon", BaseUnit: "pièce", StoreID: w.store,
		Prices: []catalog.PriceInput{{StoreID: w.store, UnitName: "pièce", Currency: "USD", Amount: amount("2")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := svc.CatalogChanges(ctx, w.p, time.Time{}, uuid.Nil, w.store, 50)
	if err != nil {
		t.Fatal(err)
	}
	row := findProduct(t, first, created.ID)
	if row.Name != "Savon" || len(row.Units) != 1 || len(row.Units[0].Prices) != 1 {
		t.Fatalf("%+v", row)
	}
	elsewhere, err := svc.CatalogChanges(ctx, w.p, time.Time{}, uuid.Nil, other, 50)
	if err != nil {
		t.Fatal(err)
	}
	foreign := findProduct(t, elsewhere, created.ID)
	if len(foreign.Units) != 1 || len(foreign.Units[0].Prices) != 0 {
		t.Fatalf("other store prices %+v", foreign.Units)
	}
	cursorA, cursorB := row.UpdatedAt, foreign.UpdatedAt
	if _, err := svc.UpsertCatalogPrice(ctx, w.p, catalog.PriceInput{
		StoreID: w.store, ProductID: created.ID, UnitName: "pièce", Currency: "USD", Amount: amount("3"),
	}); err != nil {
		t.Fatal(err)
	}
	nextA, err := svc.CatalogChanges(ctx, w.p, cursorA, created.ID, w.store, 50)
	if err != nil {
		t.Fatal(err)
	}
	changed := findProduct(t, nextA, created.ID)
	if changed.Units[0].Prices[0].UnitPrice != "3" {
		t.Fatalf("%+v", changed.Units[0].Prices)
	}
	nextB, err := svc.CatalogChanges(ctx, w.p, cursorB, created.ID, other, 50)
	if err != nil {
		t.Fatal(err)
	}
	if findOptional(nextB, created.ID) != nil {
		t.Fatal("price edit leaked into the other store sync")
	}
	if _, err := svc.ArchiveCatalogProduct(ctx, w.p, created.ID); err != nil {
		t.Fatal(err)
	}
	archivedPage, err := svc.CatalogChanges(ctx, w.p, changed.UpdatedAt, created.ID, w.store, 50)
	if err != nil {
		t.Fatal(err)
	}
	gone := findProduct(t, archivedPage, created.ID)
	if !gone.Archived || gone.Status != "archived" {
		t.Fatalf("%+v", gone)
	}

	w2 := shop(t, admin)
	if _, err := svc.GetProduct(ctx, w2.p, created.ID, w2.store); reasonOf(err) != "not_found" {
		t.Fatalf("isolation = %v", err)
	}
	page, err := svc.CatalogChanges(ctx, w2.p, time.Time{}, uuid.Nil, w2.store, 50)
	if err != nil {
		t.Fatal(err)
	}
	if findOptional(page, created.ID) != nil {
		t.Fatal("other company saw the product")
	}
	if _, err := svc.CreateCatalogProduct(ctx, w2.p, catalog.CreateInput{SKU: created.SKU, Name: "Même SKU", BaseUnit: "pièce"}); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogPermissionsHideCost(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := seed(t, admin)
	ctx := context.Background()
	sellerID := uuid.New()
	managerID := uuid.New()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := admin.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO users (id, display_name) VALUES ($1,'Vendeur'), ($2,'Gérant')`, sellerID, managerID)
	exec(`INSERT INTO organization_memberships (company_id, user_id, org_role, status) VALUES ($1,$2,'seller','active'), ($1,$3,'manager','active')`, w.company, sellerID, managerID)
	exec(`INSERT INTO store_assignments (company_id, store_id, user_id) VALUES ($1,$2,$3), ($1,$2,$4)`, w.company, w.store, sellerID, managerID)
	svc := commerce.New(app)
	seller := tenant.Principal{UserID: sellerID, CompanyID: w.company, RequestID: "seller"}
	manager := tenant.Principal{UserID: managerID, CompanyID: w.company, RequestID: "manager"}
	if _, err := svc.CreateCatalogProduct(ctx, seller, catalog.CreateInput{Name: "Interdit", BaseUnit: "pièce"}); reasonOf(err) != "catalog.manage" {
		t.Fatalf("seller create = %v", err)
	}
	rows, err := svc.ListProducts(ctx, seller, catalog.ListFilter{StoreID: w.store})
	if err != nil || len(rows) == 0 || len(rows[0].Units) == 0 || len(rows[0].Units[0].Prices) == 0 {
		t.Fatalf("seller list %+v %v", rows, err)
	}
	cost, err := svc.ProductCost(ctx, seller, w.store, w.product)
	if err != nil || cost != nil {
		t.Fatalf("seller cost = %+v %v", cost, err)
	}
	ownerCost, err := svc.ProductCost(ctx, w.p, w.store, w.product)
	if err != nil || ownerCost != nil {
		t.Fatalf("essentiel cost = %+v %v", ownerCost, err)
	}
	exec(`UPDATE subscription_snapshots SET plan_code = 'pro' WHERE company_id = $1`, w.company)
	ownerCost, err = svc.ProductCost(ctx, w.p, w.store, w.product)
	if err != nil || ownerCost == nil || ownerCost.AvgCost != "4" || ownerCost.Currency != "USD" {
		t.Fatalf("pro cost = %+v %v", ownerCost, err)
	}
	cost, err = svc.ProductCost(ctx, seller, w.store, w.product)
	if err != nil || cost != nil {
		t.Fatalf("seller still saw cost %+v %v", cost, err)
	}
	if _, err := svc.UpdateCatalogProduct(ctx, manager, w.product, catalog.UpdateInput{Name: strPtr("Huile gérant")}); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO permission_overrides (company_id, user_id, permission, effect) VALUES ($1,$2,'catalog.manage','deny')`, w.company, managerID)
	if _, err := svc.UpdateCatalogProduct(ctx, manager, w.product, catalog.UpdateInput{Name: strPtr("Non")}); reasonOf(err) != "catalog.manage" {
		t.Fatalf("manager deny = %v", err)
	}
}

func TestGraphQLCreateProductFeedsSale(t *testing.T) {
	admin, app, identityPool, _ := testutil.Pools(t)
	w := shop(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	schema, err := api.Schema(api.Deps{
		Commerce: svc, Reads: readmodel.New(app), Identity: identity.New(identityPool),
	})
	if err != nil {
		t.Fatal(err)
	}
	member := api.WithPrincipal(ctx, w.p)
	result := api.Execute(member, schema, `mutation ($input: ProductInput!) {
		createProduct(input: $input) {
			id sku name baseUnit allowFractional
			units { id name factor isBase active prices { currency unitPrice } }
		}
	}`, map[string]any{"input": map[string]any{
		"name": "Soda", "baseUnit": "bouteille", "storeId": w.store.String(),
		"prices": []any{
			map[string]any{"storeId": w.store.String(), "unitName": "bouteille", "currency": "USD", "unitPrice": "800"},
			map[string]any{"currency": "CDF"},
		},
	}})
	if len(result.Errors) > 0 {
		t.Fatal(result.Errors)
	}
	created := result.Data.(map[string]any)["createProduct"].(map[string]any)
	if created["name"] != "Soda" || created["baseUnit"] != "bouteille" {
		t.Fatalf("%v", created)
	}
	units := created["units"].([]any)
	unit := units[0].(map[string]any)
	prices := unit["prices"].([]any)
	if len(prices) != 1 || prices[0].(map[string]any)["currency"] != "USD" {
		t.Fatalf("%v", units)
	}
	reg := api.Execute(member, schema, `{ registration { steps { code done } } }`, nil)
	if len(reg.Errors) > 0 {
		t.Fatal(reg.Errors)
	}
	steps := reg.Data.(map[string]any)["registration"].(map[string]any)["steps"].([]any)
	if steps[2].(map[string]any)["code"] != "catalog" || steps[2].(map[string]any)["done"] != true {
		t.Fatalf("%v", steps)
	}
	productID := uuid.MustParse(created["id"].(string))
	if _, err := svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
		IdempotencyKey: "recv-soda", StoreID: w.store, SupplierName: "Dépôt", Currency: "USD",
		Lines: []commerce.ReceiveLine{{ProductID: productID, UnitName: "bouteille", Quantity: money.Must("4"), UnitCost: money.Must("200")}},
	}); err != nil {
		t.Fatal(err)
	}
	sale, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "soda", productID, "bouteille", "1", "800"))
	if err != nil || sale.Net != "800" {
		t.Fatalf("%+v %v", sale, err)
	}
	changes := api.Execute(member, schema, `query ($store: ID!) {
		catalogChanges(storeId: $store) { id name sku baseUnit allowFractional alertThreshold status archived units { name factor prices { currency unitPrice } } }
	}`, map[string]any{"store": w.store.String()})
	if len(changes.Errors) > 0 {
		t.Fatal(changes.Errors)
	}
	list := changes.Data.(map[string]any)["catalogChanges"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["name"] != "Soda" {
		t.Fatalf("%v", list)
	}
	var sales int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM sales WHERE company_id = $1`, w.company).Scan(&sales); err != nil || sales != 1 {
		t.Fatalf("sales=%d err=%v", sales, err)
	}
}

func saleOf(w world, key string, product uuid.UUID, unit, qtyText, priceText string) commerce.ConfirmSaleInput {
	qty := money.Must(qtyText)
	price := money.Must(priceText)
	return commerce.ConfirmSaleInput{
		IdempotencyKey: key, StoreID: w.store, Currency: "USD",
		Lines: []commerce.SaleLineInput{{
			ProductID: product, UnitName: unit, Quantity: qty, ExpectedUnitPrice: price,
		}},
		Payments: []commerce.PaymentInput{{Method: "external", Amount: money.Round(qty.Mul(price))}},
	}
}

func saleBaseQty(t *testing.T, admin *pgxpool.Pool, saleID uuid.UUID) decimal.Decimal {
	t.Helper()
	var raw string
	if err := admin.QueryRow(context.Background(), `SELECT base_quantity::text FROM sale_items WHERE sale_id = $1`, saleID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	return money.Must(raw)
}

func strPtr(value string) *string { return &value }

func boolPtr(value bool) *bool { return &value }

func unitByName(t *testing.T, product catalog.Product, name string) uuid.UUID {
	t.Helper()
	for _, unit := range product.Units {
		if unit.Name == name {
			return unit.ID
		}
	}
	t.Fatalf("unit %s missing", name)
	return uuid.Nil
}

func stockQty(t *testing.T, admin *pgxpool.Pool, w world, product uuid.UUID) decimal.Decimal {
	t.Helper()
	var raw string
	if err := admin.QueryRow(context.Background(), `
		SELECT quantity::text FROM stock_balances WHERE company_id = $1 AND store_id = $2 AND product_id = $3
	`, w.company, w.store, product).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	return money.Must(raw)
}

func findProduct(t *testing.T, rows []catalog.Product, id uuid.UUID) catalog.Product {
	t.Helper()
	if row := findOptional(rows, id); row != nil {
		return *row
	}
	t.Fatalf("product %s missing in %d rows", id, len(rows))
	return catalog.Product{}
}

func findOptional(rows []catalog.Product, id uuid.UUID) *catalog.Product {
	for i := range rows {
		if rows[i].ID == id {
			return &rows[i]
		}
	}
	return nil
}
