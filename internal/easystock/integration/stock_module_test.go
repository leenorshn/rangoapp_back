package integration

import (
	"context"
	"sync"
	"testing"

	"rangoapp/internal/easystock/api"
	"rangoapp/internal/easystock/catalog"
	"rangoapp/internal/easystock/commerce"
	"rangoapp/internal/easystock/identity"
	"rangoapp/internal/easystock/readmodel"
	"rangoapp/internal/easystock/tenant"
	"rangoapp/internal/easystock/testutil"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

func TestStockCartonPurchaseAndSales(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := shop(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	created, err := svc.CreateCatalogProduct(ctx, w.p, catalog.CreateInput{
		Name: "Eau minérale", BaseUnit: "bouteille", CategoryName: "Boissons", StoreID: w.store,
		AlertThreshold: decimal.NewFromInt(24),
		Units: []catalog.UnitInput{
			{Name: "demi-carton", Factor: decimal.NewFromInt(12), HasFactor: true, AllowPurchase: boolPtr(false), AllowSale: boolPtr(true)},
			{Name: "carton", Factor: decimal.NewFromInt(24), HasFactor: true, AllowPurchase: boolPtr(true), AllowSale: boolPtr(true)},
		},
		Prices: []catalog.PriceInput{
			{StoreID: w.store, UnitName: "bouteille", Currency: "USD", Amount: amount("0.25")},
			{StoreID: w.store, UnitName: "bouteille", Currency: "CDF", Amount: amount("650")},
			{StoreID: w.store, UnitName: "demi-carton", Currency: "USD", Amount: amount("2.80")},
			{StoreID: w.store, UnitName: "carton", Currency: "USD", Amount: amount("5.40")},
			{StoreID: w.store, UnitName: "carton", Currency: "CDF", Amount: amount("14000")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Units) != 3 {
		t.Fatalf("units = %d", len(created.Units))
	}
	var movements int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM stock_movements WHERE product_id = $1`, created.ID).Scan(&movements); err != nil || movements != 0 {
		t.Fatalf("creation wrote %d movements, err %v", movements, err)
	}
	carton := unitByName(t, created, "carton")
	received, err := svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
		IdempotencyKey: "eau-5-cartons", StoreID: w.store, SupplierName: "Dépôt", Currency: "USD", Reference: "BL-5",
		Lines: []commerce.ReceiveLine{{
			ProductID: created.ID, ProductUnitID: &carton, Quantity: decimal.NewFromInt(5), UnitCost: decimal.NewFromInt(4),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if received.TotalCost != "20" || len(received.Lines) != 1 || received.Lines[0].BaseQuantity != "120" {
		t.Fatalf("%+v", received)
	}
	if got := stockQty(t, admin, w, created.ID); !got.Equal(decimal.NewFromInt(120)) {
		t.Fatalf("after receipt %s", got)
	}
	again, err := svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
		IdempotencyKey: "eau-5-cartons", StoreID: w.store, SupplierName: "Dépôt", Currency: "USD", Reference: "BL-5",
		Lines: []commerce.ReceiveLine{{
			ProductID: created.ID, ProductUnitID: &carton, Quantity: decimal.NewFromInt(5), UnitCost: decimal.NewFromInt(4),
		}},
	})
	if err != nil || again.PurchaseID != received.PurchaseID {
		t.Fatalf("replay %+v %v", again, err)
	}
	if got := stockQty(t, admin, w, created.ID); !got.Equal(decimal.NewFromInt(120)) {
		t.Fatalf("replay changed stock to %s", got)
	}
	changed := commerce.ReceiveStockInput{
		IdempotencyKey: "eau-5-cartons", StoreID: w.store, SupplierName: "Dépôt", Currency: "USD",
		Lines: []commerce.ReceiveLine{{
			ProductID: created.ID, UnitName: "carton", Quantity: decimal.NewFromInt(1), UnitCost: decimal.NewFromInt(4),
		}},
	}
	if _, err := svc.ReceiveStock(ctx, w.p, changed); reasonOf(err) != "idempotency_mismatch" {
		t.Fatalf("mismatch = %v", err)
	}
	demi := unitByName(t, created, "demi-carton")
	if _, err := svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
		IdempotencyKey: "demi-refuse", StoreID: w.store, Currency: "USD",
		Lines: []commerce.ReceiveLine{{ProductID: created.ID, ProductUnitID: &demi, Quantity: decimal.NewFromInt(1), UnitCost: decimal.NewFromInt(2)}},
	}); reasonOf(err) != "unit_not_for_purchase" {
		t.Fatalf("purchase right = %v", err)
	}
	soldCartons, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "sell-2-cartons", created.ID, "carton", "2", "5.40"))
	if err != nil || soldCartons.Net != "10.8" {
		t.Fatalf("%+v %v", soldCartons, err)
	}
	if got := saleBaseQty(t, admin, soldCartons.SaleID); !got.Equal(decimal.NewFromInt(48)) {
		t.Fatalf("carton base %s", got)
	}
	soldPieces, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "sell-3-pieces", created.ID, "bouteille", "3", "0.25"))
	if err != nil || soldPieces.Net != "0.75" {
		t.Fatalf("%+v %v", soldPieces, err)
	}
	if got := stockQty(t, admin, w, created.ID); !got.Equal(decimal.NewFromInt(69)) {
		t.Fatalf("stock after sales %s", got)
	}
	if _, err := svc.UpdateCatalogUnit(ctx, w.p, carton, catalog.UnitInput{Name: "carton", Factor: decimal.NewFromInt(20), HasFactor: true}); reasonOf(err) != "factor_locked" {
		t.Fatalf("factor lock = %v", err)
	}
	reads := readmodel.New(app)
	journal, err := reads.StockMovements(ctx, w.p, readmodel.MovementQuery{StoreID: w.store, ProductID: created.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(journal) != 3 || journal[0].Type != "sale" || journal[2].Type != "receipt" {
		t.Fatalf("%+v", journal)
	}
	if journal[2].OriginUnit != "carton" || journal[2].OriginQuantity != "5" || journal[2].Quantity != "120" {
		t.Fatalf("receipt movement %+v", journal[2])
	}
}

func TestConcurrentReceiveSameProduct(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := shop(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	created, err := svc.CreateCatalogProduct(ctx, w.p, catalog.CreateInput{
		Name: "Eau", BaseUnit: "bouteille",
		Units: []catalog.UnitInput{{Name: "carton", Factor: decimal.NewFromInt(24), HasFactor: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
				IdempotencyKey: uuid.NewString(), StoreID: w.store, Currency: "USD",
				Lines: []commerce.ReceiveLine{{
					ProductID: created.ID, UnitName: "carton", Quantity: decimal.NewFromInt(5), UnitCost: decimal.NewFromInt(4),
				}},
			})
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := stockQty(t, admin, w, created.ID); !got.Equal(decimal.NewFromInt(240)) {
		t.Fatalf("concurrent stock %s", got)
	}
}

func TestStockInventoryAndConflict(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := shop(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	created, err := svc.CreateCatalogProduct(ctx, w.p, catalog.CreateInput{
		Name: "Eau", BaseUnit: "bouteille", StoreID: w.store, AlertThreshold: decimal.NewFromInt(10),
		Units:  []catalog.UnitInput{{Name: "carton", Factor: decimal.NewFromInt(24), HasFactor: true}},
		Prices: []catalog.PriceInput{{StoreID: w.store, UnitName: "bouteille", Currency: "USD", Amount: amount("1")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
		IdempotencyKey: "inv-open", StoreID: w.store, Currency: "USD",
		Lines: []commerce.ReceiveLine{{ProductID: created.ID, UnitName: "bouteille", Quantity: decimal.NewFromInt(100), UnitCost: decimal.NewFromInt(1)}},
	}); err != nil {
		t.Fatal(err)
	}
	bottle := unitByName(t, created, "bouteille")
	carton := unitByName(t, created, "carton")
	session, err := svc.StartStockInventory(ctx, w.p, commerce.StartInventoryInput{StoreID: w.store, Note: "matin"})
	if err != nil || session.Status != "counting" {
		t.Fatalf("%+v %v", session, err)
	}
	if _, err := svc.StartStockInventory(ctx, w.p, commerce.StartInventoryInput{StoreID: w.store}); reasonOf(err) != "inventory_open" {
		t.Fatalf("second start = %v", err)
	}
	cartons, err := svc.RecordInventoryCount(ctx, w.p, commerce.RecordCountInput{
		InventoryID: session.ID, ProductID: created.ID, ProductUnitID: carton, CountedQuantity: decimal.NewFromInt(3),
	})
	if err != nil {
		t.Fatal(err)
	}
	pieces, err := svc.RecordInventoryCount(ctx, w.p, commerce.RecordCountInput{
		InventoryID: session.ID, ProductID: created.ID, ProductUnitID: bottle, CountedQuantity: decimal.NewFromInt(5), Reason: "casse",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pieces.CountedBaseTotal != "77" || pieces.TheoreticalQuantity != "100" || pieces.Variance != "-23" {
		t.Fatalf("rollup %+v / %+v", cartons, pieces)
	}
	if got := stockQty(t, admin, w, created.ID); !got.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("count changed stock to %s", got)
	}
	done, err := svc.CompleteStockInventory(ctx, w.p, commerce.FinishInventoryInput{InventoryID: session.ID})
	if err != nil || done.Status != "completed" || done.Lines[0].Variance != "-23" {
		t.Fatalf("%+v %v", done, err)
	}
	if got := stockQty(t, admin, w, created.ID); !got.Equal(decimal.NewFromInt(77)) {
		t.Fatalf("validated stock %s", got)
	}
	if _, err := svc.CompleteStockInventory(ctx, w.p, commerce.FinishInventoryInput{InventoryID: session.ID}); reasonOf(err) != "inventory_completed" {
		t.Fatalf("second validate = %v", err)
	}
	var adjustments int
	if err := admin.QueryRow(ctx, `
		SELECT count(*) FROM stock_movements
		WHERE product_id = $1 AND movement_type = 'inventory_adjustment' AND quantity_delta = -23
	`, created.ID).Scan(&adjustments); err != nil || adjustments != 1 {
		t.Fatalf("adjustments=%d err=%v", adjustments, err)
	}

	if _, err := svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
		IdempotencyKey: "inv-zero", StoreID: w.store, Currency: "USD",
		Lines: []commerce.ReceiveLine{{ProductID: created.ID, UnitName: "bouteille", Quantity: decimal.NewFromInt(23), UnitCost: decimal.NewFromInt(1)}},
	}); err != nil {
		t.Fatal(err)
	}
	zero, err := svc.StartStockInventory(ctx, w.p, commerce.StartInventoryInput{StoreID: w.store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordInventoryCount(ctx, w.p, commerce.RecordCountInput{
		InventoryID: zero.ID, ProductID: created.ID, ProductUnitID: bottle, CountedQuantity: decimal.NewFromInt(100),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "during-count", created.ID, "bouteille", "1", "1")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CompleteStockInventory(ctx, w.p, commerce.FinishInventoryInput{InventoryID: zero.ID}); reasonOf(err) != "inventory_stale" {
		t.Fatalf("stale = %v", err)
	}
	if got := stockQty(t, admin, w, created.ID); !got.Equal(decimal.NewFromInt(99)) {
		t.Fatalf("sale stock %s", got)
	}
	if _, err := svc.RecordInventoryCount(ctx, w.p, commerce.RecordCountInput{
		InventoryID: zero.ID, ProductID: created.ID, ProductUnitID: bottle, CountedQuantity: decimal.NewFromInt(99),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CompleteStockInventory(ctx, w.p, commerce.FinishInventoryInput{InventoryID: zero.ID}); err != nil {
		t.Fatal(err)
	}
	if got := stockQty(t, admin, w, created.ID); !got.Equal(decimal.NewFromInt(99)) {
		t.Fatalf("recount stock %s", got)
	}

	positive, err := svc.StartStockInventory(ctx, w.p, commerce.StartInventoryInput{StoreID: w.store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordInventoryCount(ctx, w.p, commerce.RecordCountInput{
		InventoryID: positive.ID, ProductID: created.ID, ProductUnitID: bottle, CountedQuantity: decimal.NewFromInt(102),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CompleteStockInventory(ctx, w.p, commerce.FinishInventoryInput{InventoryID: positive.ID}); err != nil {
		t.Fatal(err)
	}
	if got := stockQty(t, admin, w, created.ID); !got.Equal(decimal.NewFromInt(102)) {
		t.Fatalf("positive stock %s", got)
	}
}

func TestStockFractionalAndSummaryPermissions(t *testing.T) {
	admin, app, identityPool, _ := testutil.Pools(t)
	w := shop(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	rice, err := svc.CreateCatalogProduct(ctx, w.p, catalog.CreateInput{
		Name: "Riz", BaseUnit: "kg", AllowFractional: true, StoreID: w.store, AlertThreshold: decimal.RequireFromString("5"),
		Units: []catalog.UnitInput{{
			Name: "sac", Factor: decimal.NewFromInt(25), HasFactor: true, AllowFractional: boolPtr(false),
		}},
		Prices: []catalog.PriceInput{
			{StoreID: w.store, UnitName: "kg", Currency: "USD", Amount: amount("1.5")},
			{StoreID: w.store, UnitName: "kg", Currency: "CDF", Amount: amount("4000")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
		IdempotencyKey: "rice-sacs", StoreID: w.store, Currency: "USD",
		Lines: []commerce.ReceiveLine{{ProductID: rice.ID, UnitName: "sac", Quantity: decimal.NewFromInt(2), UnitCost: decimal.NewFromInt(20)}},
	}); err != nil {
		t.Fatal(err)
	}
	if got := stockQty(t, admin, w, rice.ID); !got.Equal(decimal.NewFromInt(50)) {
		t.Fatalf("rice %s", got)
	}
	sold, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "rice-kg", rice.ID, "kg", "1.5", "1.5"))
	if err != nil || sold.Net != "2.25" {
		t.Fatalf("%+v %v", sold, err)
	}
	if got := stockQty(t, admin, w, rice.ID); !got.Equal(decimal.RequireFromString("48.5")) {
		t.Fatalf("rice left %s", got)
	}
	if _, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "rice-half-sac", rice.ID, "sac", "0.5", "30")); reasonOf(err) != "quantity" {
		t.Fatalf("fractional sac = %v", err)
	}
	closed, err := svc.CreateCatalogProduct(ctx, w.p, catalog.CreateInput{
		Name: "Fermé", BaseUnit: "pièce", StoreID: w.store,
		Units:  []catalog.UnitInput{{Name: "carton", Factor: decimal.NewFromInt(6), HasFactor: true, AllowSale: boolPtr(false)}},
		Prices: []catalog.PriceInput{{StoreID: w.store, UnitName: "carton", Currency: "USD", Amount: amount("3")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
		IdempotencyKey: "closed-buy", StoreID: w.store, Currency: "CDF",
		Lines: []commerce.ReceiveLine{{ProductID: closed.ID, UnitName: "carton", Quantity: decimal.NewFromInt(1), UnitCost: decimal.NewFromInt(1000)}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "closed-sell", closed.ID, "carton", "1", "3")); reasonOf(err) != "unit_not_for_sale" {
		t.Fatalf("sale right = %v", err)
	}

	low, err := svc.CreateCatalogProduct(ctx, w.p, catalog.CreateInput{
		Name: "Sucre", BaseUnit: "kg", StoreID: w.store, AlertThreshold: decimal.NewFromInt(10),
		Prices: []catalog.PriceInput{{StoreID: w.store, UnitName: "kg", Currency: "USD", Amount: amount("2")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
		IdempotencyKey: "low-sugar", StoreID: w.store, Currency: "USD",
		Lines: []commerce.ReceiveLine{{ProductID: low.ID, UnitName: "kg", Quantity: decimal.NewFromInt(4), UnitCost: decimal.NewFromInt(1)}},
	}); err != nil {
		t.Fatal(err)
	}
	seller := addMember(t, admin, w, "seller", w.store)
	reads := readmodel.New(app)
	hidden, err := reads.StockSummary(ctx, seller, w.store)
	if err != nil {
		t.Fatal(err)
	}
	if hidden.CostVisible || hidden.ProductsInStock != 3 || hidden.LowStockProducts != 1 || hidden.OutOfStockProducts != 0 {
		t.Fatalf("seller summary %+v", hidden)
	}
	if _, err := admin.Exec(ctx, `UPDATE subscription_snapshots SET plan_code = 'pro' WHERE company_id = $1`, w.company); err != nil {
		t.Fatal(err)
	}
	stillHidden, err := reads.StockSummary(ctx, seller, w.store)
	if err != nil || stillHidden.CostVisible {
		t.Fatalf("seller pro %+v %v", stillHidden, err)
	}
	visible, err := reads.StockSummary(ctx, w.p, w.store)
	if err != nil || !visible.CostVisible {
		t.Fatalf("owner summary %+v %v", visible, err)
	}
	if len(visible.StockCostValues) != 2 || visible.IncompleteCostProducts != 0 {
		t.Fatalf("costs %+v incomplete %d", visible.StockCostValues, visible.IncompleteCostProducts)
	}
	if visible.StockCostValues[0].Currency != "USD" || visible.StockCostValues[1].Currency != "CDF" {
		t.Fatalf("cost order %+v", visible.StockCostValues)
	}
	currencies := map[string]string{}
	for _, item := range visible.StockRetailValues {
		currencies[item.Currency] = item.Amount
	}
	if currencies["USD"] == "" || currencies["CDF"] == "" || currencies["USD"] == currencies["CDF"] {
		t.Fatalf("retail %+v", visible.StockRetailValues)
	}
	global, err := reads.StockSummary(ctx, w.p, uuid.Nil)
	if err != nil || global.ProductsInStock != visible.ProductsInStock {
		t.Fatalf("global %+v %v", global, err)
	}

	otherStore := uuid.New()
	if _, err := admin.Exec(ctx, `INSERT INTO stores (id, company_id, name) VALUES ($1,$2,'Annexe')`, otherStore, w.company); err != nil {
		t.Fatal(err)
	}
	if _, err := reads.StockSummary(ctx, seller, otherStore); reasonOf(err) != "store" {
		t.Fatalf("other store = %v", err)
	}
	foreign := shop(t, admin)
	foreignSummary, err := reads.StockSummary(ctx, foreign.p, uuid.Nil)
	if err != nil || foreignSummary.TotalProducts != 0 {
		t.Fatalf("foreign %+v %v", foreignSummary, err)
	}
	if _, err := svc.ReceiveStock(ctx, foreign.p, commerce.ReceiveStockInput{
		IdempotencyKey: "foreign", StoreID: w.store, Currency: "USD",
		Lines: []commerce.ReceiveLine{{ProductID: rice.ID, UnitName: "kg", Quantity: decimal.NewFromInt(1), UnitCost: decimal.NewFromInt(1)}},
	}); reasonOf(err) != "not_found" && reasonOf(err) != "store" {
		t.Fatalf("cross company = %v", err)
	}

	schema, err := api.Schema(api.Deps{Commerce: svc, Reads: reads, Identity: identity.New(identityPool)})
	if err != nil {
		t.Fatal(err)
	}
	member := api.WithPrincipal(ctx, w.p)
	denied := api.Execute(member, schema, `mutation ($input: ReceiveStockInput!) {
		receiveStock(input: $input) { id }
	}`, map[string]any{"input": map[string]any{
		"idempotencyKey": "graphql-demi", "storeId": w.store.String(), "currency": "USD",
		"lines": []any{map[string]any{
			"productId": rice.ID.String(), "unitName": "sac", "quantity": "0.5", "unitCost": "10",
		}},
	}})
	if len(denied.Errors) == 0 || denied.Errors[0].Extensions["code"] != "quantity" {
		t.Fatalf("%v", denied.Errors)
	}
	ok := api.Execute(member, schema, `query ($store: ID!) {
		stockSummary(storeId: $store) { totalProducts productsInStock costMethod retailMethod stockCostValues { currency amount } stockRetailValues { currency amount } }
		stockMovements(storeId: $store, type: "receipt") { type quantity originUnit }
	}`, map[string]any{"store": w.store.String()})
	if len(ok.Errors) > 0 {
		t.Fatal(ok.Errors)
	}
}

func addMember(t *testing.T, admin *pgxpool.Pool, w world, role string, store uuid.UUID) tenant.Principal {
	t.Helper()
	id := uuid.New()
	ctx := context.Background()
	if _, err := admin.Exec(ctx, `INSERT INTO users (id, display_name) VALUES ($1, $2)`, id, role); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
		INSERT INTO organization_memberships (company_id, user_id, org_role, status) VALUES ($1,$2,$3,'active')
	`, w.company, id, role); err != nil {
		t.Fatal(err)
	}
	if store != uuid.Nil {
		if _, err := admin.Exec(ctx, `
			INSERT INTO store_assignments (company_id, store_id, user_id) VALUES ($1,$2,$3)
		`, w.company, store, id); err != nil {
			t.Fatal(err)
		}
	}
	return tenant.Principal{UserID: id, CompanyID: w.company, OrgRole: role, RequestID: "test"}
}
