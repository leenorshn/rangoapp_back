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
	"rangoapp/internal/easystock/testutil"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

func TestTransferKeepsDraftDensityThroughDispatchReceiptAndCancel(t *testing.T) {
	admin, app, _, _ := testutilPools(t)
	w := shop(t, admin)
	dest := insertStore(t, admin, w, "Annexe densité")
	ctx := context.Background()
	svc := commerce.New(app)
	oil := oilProduct(t, svc, w, "Huile transfert", "kg", "mass", "litre", "volume")
	litre := unitByName(t, oil, "litre")
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.92"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "transfert-ouverture", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: oil.ID, ProductUnitID: litre, Quantity: decimal.NewFromInt(20)}},
	}); err != nil {
		t.Fatal(err)
	}

	draft, err := svc.CreateStockTransfer(ctx, w.p, commerce.StockTransferInput{
		IdempotencyKey: "transfert-brouillon", SourceStoreID: w.store, DestinationStoreID: dest,
		Lines: []commerce.StockTransferLine{{ProductID: oil.ID, ProductUnitID: litre, Quantity: decimal.NewFromInt(10)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertTransferLine(t, admin, uuid.MustParse(draft.ID), "10", "1", "9.2", "0.92")
	if !storeQty(t, admin, w.company, w.store, oil.ID).Equal(decimal.RequireFromString("18.4")) {
		t.Fatal("draft changed stock")
	}

	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.95"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	transferID := uuid.MustParse(draft.ID)
	if _, err := svc.DispatchStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "transfert-envoi", TransferID: transferID}); err != nil {
		t.Fatal(err)
	}
	assertMovementTrace(t, admin, oil.ID, "transfer_out", "-9.2", "litre", "10", "1", "0.92", "recorded")
	if !storeQty(t, admin, w.company, w.store, oil.ID).Equal(decimal.RequireFromString("9.2")) {
		t.Fatal("dispatch quantity")
	}

	if _, err := svc.ReceiveStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "transfert-recoit", TransferID: transferID}); err != nil {
		t.Fatal(err)
	}
	assertMovementTrace(t, admin, oil.ID, "transfer_in", "9.2", "litre", "10", "1", "0.92", "recorded")
	if !storeQty(t, admin, w.company, dest, oil.ID).Equal(decimal.RequireFromString("9.2")) {
		t.Fatal("destination quantity")
	}
	if _, err := svc.CancelStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "transfert-annule-recu", TransferID: transferID}); reasonOf(err) != "transfer_received" {
		t.Fatalf("cancel received = %v", err)
	}
	if countMovements(t, admin, oil.ID) != 3 {
		t.Fatal("cancel after receipt wrote a movement")
	}
}

func TestTransferCancelReplaysHistoricalDensity(t *testing.T) {
	admin, app, _, _ := testutilPools(t)
	w := shop(t, admin)
	dest := insertStore(t, admin, w, "Annexe annulation")
	ctx := context.Background()
	svc := commerce.New(app)
	oil := oilProduct(t, svc, w, "Huile annulation", "kg", "mass", "litre", "volume")
	litre := unitByName(t, oil, "litre")
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.92"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "annulation-ouverture", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: oil.ID, ProductUnitID: litre, Quantity: decimal.NewFromInt(10)}},
	}); err != nil {
		t.Fatal(err)
	}
	draft, err := svc.CreateStockTransfer(ctx, w.p, commerce.StockTransferInput{
		IdempotencyKey: "annulation-brouillon", SourceStoreID: w.store, DestinationStoreID: dest,
		Lines: []commerce.StockTransferLine{{ProductID: oil.ID, ProductUnitID: litre, Quantity: decimal.NewFromInt(10)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	draftID := uuid.MustParse(draft.ID)
	if _, err := svc.CancelStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "annulation-brouillon-stop", TransferID: draftID}); err != nil {
		t.Fatal(err)
	}
	if countMovements(t, admin, oil.ID) != 1 {
		t.Fatal("draft cancel wrote a stock movement")
	}

	sent, err := svc.CreateStockTransfer(ctx, w.p, commerce.StockTransferInput{
		IdempotencyKey: "annulation-transit", SourceStoreID: w.store, DestinationStoreID: dest,
		Lines: []commerce.StockTransferLine{{ProductID: oil.ID, ProductUnitID: litre, Quantity: decimal.NewFromInt(10)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	sentID := uuid.MustParse(sent.ID)
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.95"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DispatchStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "annulation-envoi", TransferID: sentID}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CancelStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "annulation-retour", TransferID: sentID}); err != nil {
		t.Fatal(err)
	}
	assertMovementTrace(t, admin, oil.ID, "transfer_out", "-9.2", "litre", "10", "1", "0.92", "recorded")
	var back int
	if err := admin.QueryRow(ctx, `
		SELECT count(*) FROM stock_movements
		WHERE product_id = $1 AND movement_type = 'transfer_in' AND reason = 'returned_to_source'
		  AND quantity_delta = 9.2 AND density_kg_per_liter = 0.92 AND origin_factor = 1
		  AND origin_quantity = 10 AND origin_provenance = 'recorded'
	`, oil.ID).Scan(&back); err != nil || back != 1 {
		t.Fatalf("compensating movement %d err %v", back, err)
	}
	if !storeQty(t, admin, w.company, w.store, oil.ID).Equal(decimal.RequireFromString("9.2")) {
		t.Fatal("source was not restored")
	}
	if !storeMissing(t, admin, w.company, dest, oil.ID) && !storeQty(t, admin, w.company, dest, oil.ID).IsZero() {
		t.Fatal("destination received a cancelled transfer")
	}
}

func TestTransferWithoutDensityKeepsNull(t *testing.T) {
	admin, app, _, _ := testutilPools(t)
	w := shop(t, admin)
	dest := insertStore(t, admin, w, "Annexe carton")
	ctx := context.Background()
	svc := commerce.New(app)
	water := mustProduct(t, svc, w, catalog.CreateInput{
		Name: "Eau transfert", BaseUnit: "bouteille", StoreID: w.store,
		Units: []catalog.UnitInput{{Name: "carton", Factor: decimal.NewFromInt(24), HasFactor: true}},
		Prices: []catalog.PriceInput{
			{StoreID: w.store, UnitName: "bouteille", Currency: "USD", Amount: amount("1")},
			{StoreID: w.store, UnitName: "carton", Currency: "USD", Amount: amount("20")},
		},
	})
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: water.ID, Density: decimal.RequireFromString("0.92"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	carton := unitByName(t, water, "carton")
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "carton-ouverture", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: water.ID, ProductUnitID: carton, Quantity: decimal.NewFromInt(2)}},
	}); err != nil {
		t.Fatal(err)
	}
	draft, err := svc.CreateStockTransfer(ctx, w.p, commerce.StockTransferInput{
		IdempotencyKey: "carton-transfert", SourceStoreID: w.store, DestinationStoreID: dest,
		Lines: []commerce.StockTransferLine{{ProductID: water.ID, ProductUnitID: carton, Quantity: decimal.NewFromInt(1)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertTransferLine(t, admin, uuid.MustParse(draft.ID), "1", "24", "24", "")
	if _, err := svc.DispatchStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "carton-envoi", TransferID: uuid.MustParse(draft.ID)}); err != nil {
		t.Fatal(err)
	}
	assertMovementTrace(t, admin, water.ID, "transfer_out", "-24", "carton", "1", "24", "", "recorded")
}

func TestRefundCopiesSaleDensityNotTheCurrentOne(t *testing.T) {
	admin, app, identityPool, _ := testutilPools(t)
	w := shop(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	oil := oilProduct(t, svc, w, "Huile retour", "kg", "mass", "litre", "volume")
	litre := unitByName(t, oil, "litre")
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.92"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "retour-ouverture", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: oil.ID, ProductUnitID: litre, Quantity: decimal.NewFromInt(10)}},
	}); err != nil {
		t.Fatal(err)
	}
	sold, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "retour-vente-3", oil.ID, "litre", "3", "2"))
	if err != nil {
		t.Fatal(err)
	}
	assertMovementTrace(t, admin, oil.ID, "sale", "-2.76", "litre", "3", "1", "0.92", "recorded")
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.95"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	itemID := saleItemID(t, admin, sold.SaleID)
	for i, key := range []string{"retour-1", "retour-2", "retour-3"} {
		if _, err := svc.CreateRefund(ctx, w.p, refundInput(w, key, sold.SaleID, itemID, "1")); err != nil {
			t.Fatalf("refund %d %v", i, err)
		}
	}
	if _, err := svc.CreateRefund(ctx, w.p, refundInput(w, "retour-trop", sold.SaleID, itemID, "1")); reasonOf(err) != "refund_quantity" {
		t.Fatalf("excess = %v", err)
	}
	var restored string
	if err := admin.QueryRow(ctx, `
		SELECT COALESCE(SUM(quantity_delta), 0)::text FROM stock_movements
		WHERE product_id = $1 AND movement_type = 'refund'
	`, oil.ID).Scan(&restored); err != nil {
		t.Fatal(err)
	}
	if moneyMust(t, restored).String() != "2.76" {
		t.Fatalf("restored %s", restored)
	}
	var documented int
	if err := admin.QueryRow(ctx, `
		SELECT count(*) FROM stock_movements
		WHERE product_id = $1 AND movement_type = 'refund'
		  AND density_kg_per_liter = 0.92 AND origin_factor = 1 AND origin_provenance = 'recorded'
	`, oil.ID).Scan(&documented); err != nil || documented != 3 {
		t.Fatalf("documented refunds %d err %v", documented, err)
	}
	if !stockQty(t, admin, w, oil.ID).Equal(decimal.RequireFromString("9.2")) {
		t.Fatalf("stock %s", stockQty(t, admin, w, oil.ID))
	}
	replay, err := svc.CreateRefund(ctx, w.p, refundInput(w, "retour-1", sold.SaleID, itemID, "1"))
	if err != nil {
		t.Fatal(err)
	}
	if countMovements(t, admin, oil.ID) != 5 {
		t.Fatal("idempotent refund duplicated a movement")
	}
	_ = replay

	foreign := shop(t, admin)
	if _, err := svc.CreateRefund(ctx, foreign.p, refundInput(w, "retour-etranger", sold.SaleID, itemID, "1")); reasonOf(err) != "not_found" {
		t.Fatalf("cross refund = %v", err)
	}

	schema, err := api.Schema(api.Deps{Commerce: svc, Reads: readmodel.New(app), Identity: identity.New(identityPool)})
	if err != nil {
		t.Fatal(err)
	}
	journal := api.Execute(api.WithPrincipal(ctx, w.p), schema, `query ($store: ID!, $product: ID!) {
		stockMovements(storeId: $store, productId: $product, type: "sale_return") {
			quantity originQuantity originFactor densityKgPerLiter originProvenance
		}
	}`, map[string]any{"store": w.store.String(), "product": oil.ID.String()})
	if len(journal.Errors) > 0 {
		t.Fatal(journal.Errors)
	}
	rows := journal.Data.(map[string]any)["stockMovements"].([]any)
	if len(rows) != 3 || rows[0].(map[string]any)["originFactor"] != "1" || rows[0].(map[string]any)["densityKgPerLiter"] != "0.92" {
		t.Fatalf("journal %+v", rows)
	}
}

func TestRefundOfIncompleteSaleDoesNotInventDensity(t *testing.T) {
	admin, app, _, _ := testutilPools(t)
	w := shop(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	oil := oilProduct(t, svc, w, "Huile ancienne", "kg", "mass", "litre", "volume")
	litre := unitByName(t, oil, "litre")
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.92"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "ancienne-ouverture", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: oil.ID, ProductUnitID: litre, Quantity: decimal.NewFromInt(2)}},
	}); err != nil {
		t.Fatal(err)
	}
	sold, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "ancienne-vente", oil.ID, "litre", "1", "2"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
		UPDATE stock_movements
		SET origin_factor = NULL, density_kg_per_liter = NULL, origin_provenance = 'legacy_unknown'
		WHERE ref_id = $1 AND movement_type = 'sale'
	`, sold.SaleID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.95"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateRefund(ctx, w.p, refundInput(w, "ancienne-retour", sold.SaleID, saleItemID(t, admin, sold.SaleID), "1")); err != nil {
		t.Fatal(err)
	}
	assertMovementTrace(t, admin, oil.ID, "refund", "0.92", "litre", "1", "", "", "recovered")
	if !stockQty(t, admin, w, oil.ID).Equal(decimal.RequireFromString("1.84")) {
		t.Fatalf("stock %s", stockQty(t, admin, w, oil.ID))
	}
}

func TestSaleAndReceiptKeepOriginFactor(t *testing.T) {
	admin, app, _, _ := testutilPools(t)
	w := shop(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	oil := mustProduct(t, svc, w, catalog.CreateInput{
		Name: "Huile bidon", BaseUnit: "kg", BaseDimension: "mass", AllowFractional: true, StoreID: w.store,
		Units: []catalog.UnitInput{{
			Name: "bidon", Factor: decimal.NewFromInt(12), HasFactor: true, Dimension: "volume", HasDimension: true,
		}},
		Prices: []catalog.PriceInput{
			{StoreID: w.store, UnitName: "kg", Currency: "USD", Amount: amount("1")},
			{StoreID: w.store, UnitName: "bidon", Currency: "USD", Amount: amount("10")},
		},
	})
	bidon := unitByName(t, oil, "bidon")
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.92"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
		IdempotencyKey: "bidon-reception", StoreID: w.store, Currency: "USD",
		Lines: []commerce.ReceiveLine{{ProductID: oil.ID, ProductUnitID: &bidon, Quantity: decimal.NewFromInt(3), UnitCost: decimal.NewFromInt(8)}},
	}); err != nil {
		t.Fatal(err)
	}
	assertMovementTrace(t, admin, oil.ID, "receive", "33.12", "bidon", "3", "12", "0.92", "recorded")
	if _, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "bidon-vente", oil.ID, "bidon", "2", "10")); err != nil {
		t.Fatal(err)
	}
	assertMovementTrace(t, admin, oil.ID, "sale", "-22.08", "bidon", "2", "12", "0.92", "recorded")
	if !stockQty(t, admin, w, oil.ID).Equal(decimal.RequireFromString("11.04")) {
		t.Fatalf("after sale %s", stockQty(t, admin, w, oil.ID))
	}
	if _, err := svc.UpdateCatalogUnit(ctx, w.p, bidon, catalog.UnitInput{Factor: decimal.NewFromInt(15), HasFactor: true}); reasonOf(err) != "factor_locked" {
		t.Fatalf("factor change = %v", err)
	}
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.95"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	assertMovementTrace(t, admin, oil.ID, "sale", "-22.08", "bidon", "2", "12", "0.92", "recorded")
	assertMovementTrace(t, admin, oil.ID, "receive", "33.12", "bidon", "3", "12", "0.92", "recorded")

	futaille, err := svc.CreateCatalogUnit(ctx, w.p, oil.ID, catalog.UnitInput{
		Name: "futaille", Factor: decimal.NewFromInt(15), HasFactor: true, Dimension: "volume", HasDimension: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpsertCatalogPrice(ctx, w.p, catalog.PriceInput{
		StoreID: w.store, ProductID: oil.ID, UnitID: futaille.ID, Currency: "USD", Amount: amount("12"),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
		IdempotencyKey: "futaille-reception", StoreID: w.store, Currency: "USD",
		Lines: []commerce.ReceiveLine{{ProductID: oil.ID, ProductUnitID: &futaille.ID, Quantity: decimal.NewFromInt(1), UnitCost: decimal.NewFromInt(9)}},
	}); err != nil {
		t.Fatal(err)
	}
	assertMovementTrace(t, admin, oil.ID, "receive", "14.25", "futaille", "1", "15", "0.95", "recorded")
	if _, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "futaille-vente", oil.ID, "futaille", "1", "12")); err != nil {
		t.Fatal(err)
	}
	assertMovementTrace(t, admin, oil.ID, "sale", "-14.25", "futaille", "1", "15", "0.95", "recorded")
	assertMovementTrace(t, admin, oil.ID, "sale", "-22.08", "bidon", "2", "12", "0.92", "recorded")
	assertMovementTrace(t, admin, oil.ID, "receive", "33.12", "bidon", "3", "12", "0.92", "recorded")
}

func TestSaleWithoutDensityStillStoresFactor(t *testing.T) {
	admin, app, _, _ := testutilPools(t)
	w := shop(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	water := mustProduct(t, svc, w, catalog.CreateInput{
		Name: "Eau pièce", BaseUnit: "bouteille", StoreID: w.store,
		Units: []catalog.UnitInput{{Name: "carton", Factor: decimal.NewFromInt(24), HasFactor: true}},
		Prices: []catalog.PriceInput{
			{StoreID: w.store, UnitName: "bouteille", Currency: "USD", Amount: amount("1")},
			{StoreID: w.store, UnitName: "carton", Currency: "USD", Amount: amount("20")},
		},
	})
	carton := unitByName(t, water, "carton")
	if _, err := svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
		IdempotencyKey: "carton-reception", StoreID: w.store, Currency: "USD",
		Lines: []commerce.ReceiveLine{{ProductID: water.ID, ProductUnitID: &carton, Quantity: decimal.NewFromInt(1), UnitCost: decimal.NewFromInt(10)}},
	}); err != nil {
		t.Fatal(err)
	}
	assertMovementTrace(t, admin, water.ID, "receive", "24", "carton", "1", "24", "", "recorded")
	if _, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "carton-vente", water.ID, "carton", "1", "20")); err != nil {
		t.Fatal(err)
	}
	assertMovementTrace(t, admin, water.ID, "sale", "-24", "carton", "1", "24", "", "recorded")
}

func TestInventoryAdjustmentStaysRecorded(t *testing.T) {
	admin, app, _, _ := testutilPools(t)
	w := shop(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	oil := oilProduct(t, svc, w, "Huile inventaire", "kg", "mass", "litre", "volume")
	litre := unitByName(t, oil, "litre")
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.92"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "inventaire-ouverture", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: oil.ID, ProductUnitID: litre, Quantity: decimal.NewFromInt(10)}},
	}); err != nil {
		t.Fatal(err)
	}
	session, err := svc.StartStockInventory(ctx, w.p, commerce.StartInventoryInput{StoreID: w.store})
	if err != nil {
		t.Fatal(err)
	}
	counted, err := svc.RecordInventoryCount(ctx, w.p, commerce.RecordCountInput{
		InventoryID: session.ID, ProductID: oil.ID, ProductUnitID: litre, CountedQuantity: decimal.NewFromInt(8), Reason: "casse",
	})
	if err != nil || counted.CountedBaseTotal != "7.36" {
		t.Fatalf("%+v %v", counted, err)
	}
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.95"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CompleteStockInventory(ctx, w.p, commerce.FinishInventoryInput{InventoryID: session.ID}); err != nil {
		t.Fatal(err)
	}
	assertMovementTrace(t, admin, oil.ID, "inventory_adjustment", "-1.84", "kg", "-1.84", "", "", "recorded")
	if !stockQty(t, admin, w, oil.ID).Equal(decimal.RequireFromString("7.36")) {
		t.Fatalf("stock %s", stockQty(t, admin, w, oil.ID))
	}
	var legacy int
	if err := admin.QueryRow(ctx, `
		SELECT count(*) FROM stock_movements
		WHERE product_id = $1 AND origin_provenance = 'legacy_unknown' AND movement_type = 'inventory_adjustment'
	`, oil.ID).Scan(&legacy); err != nil || legacy != 0 {
		t.Fatalf("legacy adjustments %d err %v", legacy, err)
	}
}

func TestHistoricalTransferLineKeepsNullDensity(t *testing.T) {
	admin, app, _, _ := testutilPools(t)
	w := shop(t, admin)
	dest := insertStore(t, admin, w, "Annexe ancienne")
	ctx := context.Background()
	svc := commerce.New(app)
	oil := oilProduct(t, svc, w, "Huile ancienne ligne", "kg", "mass", "litre", "volume")
	litre := unitByName(t, oil, "litre")
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.92"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "ancienne-ligne-ouverture", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: oil.ID, ProductUnitID: litre, Quantity: decimal.NewFromInt(30)}},
	}); err != nil {
		t.Fatal(err)
	}
	first, err := svc.CreateStockTransfer(ctx, w.p, commerce.StockTransferInput{
		IdempotencyKey: "ancienne-ligne-a", SourceStoreID: w.store, DestinationStoreID: dest,
		Lines: []commerce.StockTransferLine{{ProductID: oil.ID, ProductUnitID: litre, Quantity: decimal.NewFromInt(10)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.CreateStockTransfer(ctx, w.p, commerce.StockTransferInput{
		IdempotencyKey: "ancienne-ligne-b", SourceStoreID: w.store, DestinationStoreID: dest,
		Lines: []commerce.StockTransferLine{{ProductID: oil.ID, ProductUnitID: litre, Quantity: decimal.NewFromInt(10)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
		UPDATE stock_transfer_lines SET density_kg_per_liter = NULL
		WHERE transfer_id IN ($1, $2)
	`, first.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.95"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	firstID := uuid.MustParse(first.ID)
	secondID := uuid.MustParse(second.ID)
	if _, err := svc.DispatchStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "ancienne-envoi-a", TransferID: firstID}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReceiveStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "ancienne-recoit-a", TransferID: firstID}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DispatchStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "ancienne-envoi-b", TransferID: secondID}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CancelStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "ancienne-annule-b", TransferID: secondID}); err != nil {
		t.Fatal(err)
	}
	var unexplained int
	if err := admin.QueryRow(ctx, `
		SELECT count(*) FROM stock_movements
		WHERE product_id = $1
		  AND movement_type IN ('transfer_out', 'transfer_in')
		  AND quantity_delta IN (-9.2, 9.2)
		  AND origin_quantity = 10 AND origin_factor = 1
		  AND density_kg_per_liter IS NULL
		  AND origin_provenance = 'recovered'
	`, oil.ID).Scan(&unexplained); err != nil || unexplained != 4 {
		t.Fatalf("historical transfer movements %d err %v", unexplained, err)
	}
	var invented int
	if err := admin.QueryRow(ctx, `
		SELECT count(*) FROM stock_movements
		WHERE product_id = $1 AND density_kg_per_liter = 0.95
	`, oil.ID).Scan(&invented); err != nil || invented != 0 {
		t.Fatalf("invented density %d err %v", invented, err)
	}
	if !storeQty(t, admin, w.company, w.store, oil.ID).Equal(decimal.RequireFromString("18.4")) {
		t.Fatalf("source %s", storeQty(t, admin, w.company, w.store, oil.ID))
	}
	if !storeQty(t, admin, w.company, dest, oil.ID).Equal(decimal.RequireFromString("9.2")) {
		t.Fatalf("destination %s", storeQty(t, admin, w.company, dest, oil.ID))
	}
}

func TestPartialRefundRemainderKeepsHistoricalDensity(t *testing.T) {
	admin, app, _, _ := testutilPools(t)
	w := shop(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	oil := oilProduct(t, svc, w, "Huile reste", "kg", "mass", "litre", "volume")
	litre := unitByName(t, oil, "litre")
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.92"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "reste-ouverture", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: oil.ID, ProductUnitID: litre, Quantity: decimal.NewFromInt(2)}},
	}); err != nil {
		t.Fatal(err)
	}
	sold, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "reste-vente", oil.ID, "litre", "1", "2"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.95"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	itemID := saleItemID(t, admin, sold.SaleID)
	for i, qty := range []string{"0.333333", "0.333333", "0.333334"} {
		if _, err := svc.CreateRefund(ctx, w.p, refundInput(w, "reste-"+qty+"-"+string(rune('a'+i)), sold.SaleID, itemID, qty)); err != nil {
			t.Fatalf("refund %s %v", qty, err)
		}
	}
	if _, err := svc.CreateRefund(ctx, w.p, refundInput(w, "reste-trop", sold.SaleID, itemID, "0.000001")); reasonOf(err) != "refund_quantity" {
		t.Fatalf("excess = %v", err)
	}
	var restored string
	var documented, remainder int
	if err := admin.QueryRow(ctx, `
		SELECT COALESCE(SUM(quantity_delta), 0)::text,
		       count(*) FILTER (WHERE density_kg_per_liter = 0.92 AND origin_factor = 1 AND origin_provenance = 'recorded' AND quantity_delta = 0.306666),
		       count(*) FILTER (WHERE density_kg_per_liter = 0.92 AND origin_factor = 1 AND origin_provenance = 'recovered' AND quantity_delta = 0.306668)
		FROM stock_movements
		WHERE product_id = $1 AND movement_type = 'refund'
	`, oil.ID).Scan(&restored, &documented, &remainder); err != nil {
		t.Fatal(err)
	}
	if moneyMust(t, restored).String() != "0.92" || documented != 2 || remainder != 1 {
		t.Fatalf("restored %s recorded %d remainder %d", restored, documented, remainder)
	}
	if !stockQty(t, admin, w, oil.ID).Equal(decimal.RequireFromString("1.84")) {
		t.Fatalf("stock %s", stockQty(t, admin, w, oil.ID))
	}
}

func TestConcurrentTransferDispatchPostsOneMovement(t *testing.T) {
	admin, app, _, _ := testutilPools(t)
	w := shop(t, admin)
	dest := insertStore(t, admin, w, "Annexe course")
	ctx := context.Background()
	svc := commerce.New(app)
	oil := oilProduct(t, svc, w, "Huile course", "kg", "mass", "litre", "volume")
	litre := unitByName(t, oil, "litre")
	if _, err := svc.SetProductDensity(ctx, w.p, catalog.DensityInput{
		ProductID: oil.ID, Density: decimal.RequireFromString("0.92"), HasDensity: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "course-ouverture", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: oil.ID, ProductUnitID: litre, Quantity: decimal.NewFromInt(10)}},
	}); err != nil {
		t.Fatal(err)
	}
	draft, err := svc.CreateStockTransfer(ctx, w.p, commerce.StockTransferInput{
		IdempotencyKey: "course-brouillon", SourceStoreID: w.store, DestinationStoreID: dest,
		Lines: []commerce.StockTransferLine{{ProductID: oil.ID, ProductUnitID: litre, Quantity: decimal.NewFromInt(10)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	transferID := uuid.MustParse(draft.ID)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, key := range []string{"course-a", "course-b"} {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			_, err := svc.DispatchStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: key, TransferID: transferID})
			errs <- err
		}(key)
	}
	wg.Wait()
	close(errs)
	ok, rejected := 0, 0
	for err := range errs {
		if err == nil {
			ok++
			continue
		}
		if reasonOf(err) != "transfer_dispatched" {
			t.Fatal(err)
		}
		rejected++
	}
	if ok != 1 || rejected != 1 {
		t.Fatalf("dispatch ok %d rejected %d", ok, rejected)
	}
	var outs int
	if err := admin.QueryRow(ctx, `
		SELECT count(*) FROM stock_movements
		WHERE product_id = $1 AND movement_type = 'transfer_out'
	`, oil.ID).Scan(&outs); err != nil || outs != 1 {
		t.Fatalf("transfer outs %d err %v", outs, err)
	}
	if !storeQty(t, admin, w.company, w.store, oil.ID).IsZero() {
		t.Fatalf("source %s", storeQty(t, admin, w.company, w.store, oil.ID))
	}
}

func testutilPools(t *testing.T) (*pgxpool.Pool, *pgxpool.Pool, *pgxpool.Pool, *pgxpool.Pool) {
	t.Helper()
	return testutil.Pools(t)
}

func refundInput(w world, key string, saleID, itemID uuid.UUID, qty string) commerce.CreateRefundInput {
	return commerce.CreateRefundInput{
		IdempotencyKey: key,
		SaleID:         saleID,
		Reason:         "client a changé d'avis",
		CashSessionID:  &w.session,
		Lines:          []commerce.RefundLineInput{{SaleItemID: itemID, Quantity: decimal.RequireFromString(qty), Restock: true}},
	}
}

func saleItemID(t *testing.T, admin *pgxpool.Pool, saleID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := admin.QueryRow(context.Background(), `SELECT id FROM sale_items WHERE sale_id = $1`, saleID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func countMovements(t *testing.T, admin *pgxpool.Pool, product uuid.UUID) int {
	t.Helper()
	var n int
	if err := admin.QueryRow(context.Background(), `SELECT count(*) FROM stock_movements WHERE product_id = $1`, product).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func assertTransferLine(t *testing.T, admin *pgxpool.Pool, transferID uuid.UUID, qty, factor, base, density string) {
	t.Helper()
	var gotQty, gotFactor, gotBase string
	var gotDensity *string
	err := admin.QueryRow(context.Background(), `
		SELECT quantity::text, factor::text, base_quantity::text, density_kg_per_liter::text
		FROM stock_transfer_lines WHERE transfer_id = $1
	`, transferID).Scan(&gotQty, &gotFactor, &gotBase, &gotDensity)
	if err != nil {
		t.Fatal(err)
	}
	if moneyMust(t, gotQty).String() != qty || moneyMust(t, gotFactor).String() != factor || moneyMust(t, gotBase).String() != base {
		t.Fatalf("line %s %s %s", gotQty, gotFactor, gotBase)
	}
	if density == "" {
		if gotDensity != nil {
			t.Fatalf("density %s", *gotDensity)
		}
		return
	}
	if gotDensity == nil || moneyMust(t, *gotDensity).String() != density {
		t.Fatalf("density %v", gotDensity)
	}
}

func assertMovementTrace(t *testing.T, admin *pgxpool.Pool, product uuid.UUID, movementType, delta, unit, qty, factor, density, provenance string) {
	t.Helper()
	var gotDelta, gotUnit, gotQty, gotProvenance string
	var gotFactor, gotDensity *string
	err := admin.QueryRow(context.Background(), `
		SELECT quantity_delta::text, origin_unit_name, origin_quantity::text, origin_factor::text,
		       density_kg_per_liter::text, origin_provenance
		FROM stock_movements
		WHERE product_id = $1 AND movement_type = $2
		  AND quantity_delta = $3
		ORDER BY created_at DESC
		LIMIT 1
	`, product, movementType, delta).Scan(&gotDelta, &gotUnit, &gotQty, &gotFactor, &gotDensity, &gotProvenance)
	if err != nil {
		t.Fatalf("%s %s: %v", movementType, delta, err)
	}
	if gotUnit != unit || moneyMust(t, gotQty).String() != qty || gotProvenance != provenance {
		t.Fatalf("%s unit %s qty %s provenance %s", movementType, gotUnit, gotQty, gotProvenance)
	}
	if factor == "" {
		if gotFactor != nil {
			t.Fatalf("%s factor %s", movementType, *gotFactor)
		}
	} else if gotFactor == nil || moneyMust(t, *gotFactor).String() != factor {
		t.Fatalf("%s factor %v", movementType, gotFactor)
	}
	if density == "" {
		if gotDensity != nil {
			t.Fatalf("%s density %s", movementType, *gotDensity)
		}
	} else if gotDensity == nil || moneyMust(t, *gotDensity).String() != density {
		t.Fatalf("%s density %v", movementType, gotDensity)
	}
}
