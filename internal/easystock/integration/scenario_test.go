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

func TestStockModuleBusinessScenario(t *testing.T) {
	admin, app, identityPool, _ := testutil.Pools(t)
	w := shop(t, admin)
	ctx := context.Background()
	dest := insertStore(t, admin, w, "Annexe")
	svc := commerce.New(app)
	reads := readmodel.New(app)

	soap := mustProduct(t, svc, w, catalog.CreateInput{
		Name: "Savon", BaseUnit: "pièce", StoreID: w.store,
		Units: []catalog.UnitInput{{Name: "carton", Factor: decimal.NewFromInt(10), HasFactor: true}},
		Prices: []catalog.PriceInput{
			{StoreID: w.store, UnitName: "pièce", Currency: "USD", Amount: amount("2")},
			{StoreID: w.store, UnitName: "carton", Currency: "USD", Amount: amount("20")},
		},
	})
	piece := unitByName(t, soap, "pièce")
	carton := unitByName(t, soap, "carton")
	if _, err := svc.UpsertCatalogPrice(ctx, w.p, catalog.PriceInput{
		StoreID: dest, ProductID: soap.ID, UnitID: piece, Currency: "USD", Amount: amount("2"),
	}); err != nil {
		t.Fatal(err)
	}

	cost := decimal.NewFromInt(10)
	opened, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "scenario-ouverture", StoreID: w.store, Note: "Déjà en rayon",
		Lines: []commerce.OpeningLine{{ProductID: soap.ID, ProductUnitID: carton, Quantity: decimal.NewFromInt(2), UnitCost: &cost, Currency: "USD"}},
	})
	if err != nil || opened.Lines[0].BaseQuantity != "20" {
		t.Fatalf("opening %+v %v", opened, err)
	}
	replay, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "scenario-ouverture", StoreID: w.store, Note: "Déjà en rayon",
		Lines: []commerce.OpeningLine{{ProductID: soap.ID, ProductUnitID: carton, Quantity: decimal.NewFromInt(2), UnitCost: &cost, Currency: "USD"}},
	})
	if err != nil || replay.OpeningID != opened.OpeningID {
		t.Fatalf("replay %+v %v", replay, err)
	}
	assertQtyValue(t, admin, w.company, w.store, soap.ID, 20, 20)

	if _, err := svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
		IdempotencyKey: "scenario-reception", StoreID: w.store, Currency: "USD", SupplierName: "Savonnerie",
		Lines: []commerce.ReceiveLine{{ProductID: soap.ID, ProductUnitID: &carton, Quantity: decimal.NewFromInt(1), UnitCost: cost}},
	}); err != nil {
		t.Fatal(err)
	}
	assertQtyValue(t, admin, w.company, w.store, soap.ID, 30, 30)

	if _, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "scenario-vente", soap.ID, "pièce", "4", "2")); err != nil {
		t.Fatal(err)
	}
	assertQtyValue(t, admin, w.company, w.store, soap.ID, 26, 26)

	if _, err := svc.RecordStockLoss(ctx, w.p, commerce.StockLossInput{
		IdempotencyKey: "scenario-perte", StoreID: w.store, ProductID: soap.ID, ProductUnitID: piece,
		Quantity: decimal.NewFromInt(1), Reason: "damaged", Note: "Un savon cassé",
	}); err != nil {
		t.Fatal(err)
	}
	assertQtyValue(t, admin, w.company, w.store, soap.ID, 25, 25)

	if _, err := svc.ReturnStockToSupplier(ctx, w.p, commerce.SupplierReturnInput{
		IdempotencyKey: "scenario-retour", StoreID: w.store, SupplierName: "Savonnerie", Reason: "Carton abîmé",
		Lines: []commerce.SupplierReturnLine{{ProductID: soap.ID, ProductUnitID: carton, Quantity: decimal.NewFromInt(1)}},
	}); err != nil {
		t.Fatal(err)
	}
	assertQtyValue(t, admin, w.company, w.store, soap.ID, 15, 15)

	created, err := svc.CreateStockTransfer(ctx, w.p, commerce.StockTransferInput{
		IdempotencyKey: "scenario-transfert", SourceStoreID: w.store, DestinationStoreID: dest,
		Lines: []commerce.StockTransferLine{{ProductID: soap.ID, ProductUnitID: carton, Quantity: decimal.NewFromInt(1)}},
	})
	if err != nil || created.Status != "draft" || created.InTransit != "0" {
		t.Fatalf("draft %+v %v", created, err)
	}
	if got := stockQty(t, admin, w, soap.ID); !got.Equal(decimal.NewFromInt(15)) {
		t.Fatalf("draft moved stock %s", got)
	}
	transferID := uuid.MustParse(created.ID)
	sent, err := svc.DispatchStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "scenario-envoi", TransferID: transferID})
	if err != nil || sent.Status != "in_transit" || sent.InTransit != "10" {
		t.Fatalf("dispatch %+v %v", sent, err)
	}
	if got := stockQty(t, admin, w, soap.ID); !got.Equal(decimal.NewFromInt(5)) {
		t.Fatalf("source after dispatch %s", got)
	}
	if !storeMissing(t, admin, w.company, dest, soap.ID) {
		t.Fatal("destination received before the receipt")
	}
	if got := heldValue(t, admin, w.company); !got.Equal(decimal.NewFromInt(15)) {
		t.Fatalf("value in transit %s", got)
	}
	received, err := svc.ReceiveStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "scenario-arrivee", TransferID: transferID})
	if err != nil || received.Status != "received" || received.InTransit != "0" {
		t.Fatalf("receive %+v %v", received, err)
	}
	if got := storeQty(t, admin, w.company, dest, soap.ID); !got.Equal(decimal.NewFromInt(10)) {
		t.Fatalf("destination %s", got)
	}
	assertQtyValue(t, admin, w.company, w.store, soap.ID, 5, 15)

	session, err := svc.StartStockInventory(ctx, w.p, commerce.StartInventoryInput{StoreID: w.store, Note: "Clôture"})
	if err != nil {
		t.Fatal(err)
	}
	counted, err := svc.RecordInventoryCount(ctx, w.p, commerce.RecordCountInput{
		InventoryID: session.ID, ProductID: soap.ID, ProductUnitID: piece, CountedQuantity: decimal.NewFromInt(4),
	})
	if err != nil || counted.TheoreticalQuantity != "5" || counted.Variance != "-1" {
		t.Fatalf("count %+v %v", counted, err)
	}
	if got := stockQty(t, admin, w, soap.ID); !got.Equal(decimal.NewFromInt(5)) {
		t.Fatalf("count changed stock %s", got)
	}
	done, err := svc.CompleteStockInventory(ctx, w.p, commerce.FinishInventoryInput{InventoryID: session.ID})
	if err != nil || done.Status != "completed" {
		t.Fatalf("validate %+v %v", done, err)
	}
	if got := stockQty(t, admin, w, soap.ID); !got.Equal(decimal.NewFromInt(4)) {
		t.Fatalf("validated source %s", got)
	}
	if got := storeQty(t, admin, w.company, dest, soap.ID); !got.Equal(decimal.NewFromInt(10)) {
		t.Fatalf("validated destination %s", got)
	}
	if got := heldValue(t, admin, w.company); !got.Equal(decimal.NewFromInt(14)) {
		t.Fatalf("final value %s", got)
	}

	sourceJournal, err := reads.StockMovements(ctx, w.p, readmodel.MovementQuery{StoreID: w.store, ProductID: soap.ID})
	if err != nil {
		t.Fatal(err)
	}
	wantSource := map[string]bool{"opening": false, "receipt": false, "sale": false, "loss": false, "purchase_return": false, "transfer_out": false, "inventory_adjustment": false}
	for _, row := range sourceJournal {
		if _, ok := wantSource[row.Type]; ok {
			wantSource[row.Type] = true
		}
		if row.OriginProvenance != "recorded" {
			t.Fatalf("provenance %s %+v", row.Type, row)
		}
	}
	for kind, seen := range wantSource {
		if !seen {
			t.Fatalf("missing %s in %+v", kind, sourceJournal)
		}
	}
	destJournal, err := reads.StockMovements(ctx, w.p, readmodel.MovementQuery{StoreID: dest, ProductID: soap.ID})
	if err != nil || len(destJournal) != 1 || destJournal[0].Type != "transfer_in" || destJournal[0].Quantity != "10" {
		t.Fatalf("destination journal %+v %v", destJournal, err)
	}
	if got := summaryAmount(t, reads, w, w.store, "USD"); !got.Equal(decimal.NewFromInt(4)) {
		t.Fatalf("source valuation %s", got)
	}
	if got := summaryAmount(t, reads, w, dest, "USD"); !got.Equal(decimal.NewFromInt(10)) {
		t.Fatalf("destination valuation %s", got)
	}
	company, err := reads.StockSummary(ctx, w.p, uuid.Nil)
	if err != nil || !company.CostVisible {
		t.Fatalf("company summary %+v %v", company, err)
	}
	var companyUSD decimal.Decimal
	for _, item := range company.StockCostValues {
		if item.Currency == "USD" {
			companyUSD = moneyMust(t, item.Amount)
		}
	}
	if !companyUSD.Equal(decimal.NewFromInt(14)) || company.ProductsInStock != 1 {
		t.Fatalf("company %+v", company)
	}

	seller := addMember(t, admin, w, "seller", w.store)
	if _, err := svc.RecordStockLoss(ctx, seller, commerce.StockLossInput{
		IdempotencyKey: "scenario-perte-vendeur", StoreID: w.store, ProductID: soap.ID, ProductUnitID: piece,
		Quantity: decimal.NewFromInt(1), Reason: "stolen",
	}); reasonOf(err) != "stock.loss.create" {
		t.Fatalf("seller loss = %v", err)
	}
	hidden, err := reads.StockSummary(ctx, seller, w.store)
	if err != nil || hidden.CostVisible || hidden.StockCostValues != nil || hidden.ProductsInStock != 1 {
		t.Fatalf("seller summary %+v %v", hidden, err)
	}
	if got := stockQty(t, admin, w, soap.ID); !got.Equal(decimal.NewFromInt(4)) {
		t.Fatalf("seller changed stock %s", got)
	}

	foreign := shop(t, admin)
	if _, err := svc.RecordStockLoss(ctx, foreign.p, commerce.StockLossInput{
		IdempotencyKey: "scenario-autre-entreprise", StoreID: w.store, ProductID: soap.ID, ProductUnitID: piece,
		Quantity: decimal.NewFromInt(1), Reason: "other",
	}); reasonOf(err) != "not_found" {
		t.Fatalf("cross company = %v", err)
	}
	if got := stockQty(t, admin, w, soap.ID); !got.Equal(decimal.NewFromInt(4)) {
		t.Fatalf("isolated stock %s", got)
	}

	var cash, payments int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM cash_transactions WHERE company_id = $1`, w.company).Scan(&cash); err != nil || cash != 0 {
		t.Fatalf("cash=%d err=%v", cash, err)
	}
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM supplier_payments WHERE company_id = $1`, w.company).Scan(&payments); err != nil || payments != 0 {
		t.Fatalf("payments=%d err=%v", payments, err)
	}

	schema, err := api.Schema(api.Deps{Commerce: svc, Reads: reads, Identity: identity.New(identityPool)})
	if err != nil {
		t.Fatal(err)
	}
	view := api.Execute(api.WithPrincipal(ctx, w.p), schema, `query ($store: ID!, $product: ID!) {
		stockSummary(storeId: $store) { productsInStock stockCostValues { currency amount } costMethod }
		stockMovements(storeId: $store, productId: $product, type: "opening") { type quantity originProvenance }
	}`, map[string]any{"store": w.store.String(), "product": soap.ID.String()})
	if len(view.Errors) > 0 {
		t.Fatal(view.Errors)
	}
	data := view.Data.(map[string]any)
	summary := data["stockSummary"].(map[string]any)
	if summary["productsInStock"] != 1 || summary["costMethod"] != "weighted_average" {
		t.Fatalf("graphql summary %+v", summary)
	}
	rows := data["stockMovements"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["quantity"] != "20" || rows[0].(map[string]any)["originProvenance"] != "recorded" {
		t.Fatalf("graphql journal %+v", rows)
	}
}

func assertQtyValue(t *testing.T, admin *pgxpool.Pool, company, store, product uuid.UUID, qty, value int64) {
	t.Helper()
	if got := storeQty(t, admin, company, store, product); !got.Equal(decimal.NewFromInt(qty)) {
		t.Fatalf("quantity %s, want %d", got, qty)
	}
	if got := heldValue(t, admin, company); !got.Equal(decimal.NewFromInt(value)) {
		t.Fatalf("value %s, want %d", got, value)
	}
}
