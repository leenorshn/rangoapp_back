package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"rangoapp/internal/easystock/api"
	"rangoapp/internal/easystock/catalog"
	"rangoapp/internal/easystock/commerce"
	"rangoapp/internal/easystock/identity"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/readmodel"
	"rangoapp/internal/easystock/reports"
	"rangoapp/internal/easystock/testutil"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestMovementOriginRecovery(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := shop(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	water := mustProduct(t, svc, w, catalog.CreateInput{
		Name: "Eau", BaseUnit: "bouteille", StoreID: w.store,
		Units:  []catalog.UnitInput{{Name: "carton", Factor: decimal.NewFromInt(24), HasFactor: true}},
		Prices: []catalog.PriceInput{{StoreID: w.store, UnitName: "bouteille", Currency: "USD", Amount: amount("0.25")}},
	})
	bottle := unitByName(t, water, "bouteille")
	carton := unitByName(t, water, "carton")
	cost := decimal.NewFromInt(1)
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "ouverture-historique", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: water.ID, ProductUnitID: bottle, Quantity: decimal.NewFromInt(12), UnitCost: &cost, Currency: "USD"}},
	}); err != nil {
		t.Fatal(err)
	}
	before := stockQty(t, admin, w, water.ID)

	supplier := uuid.New()
	purchase := uuid.New()
	if _, err := admin.Exec(ctx, `INSERT INTO suppliers (id, company_id, store_id, name) VALUES ($1,$2,$3,'Ancien')`, supplier, w.company, w.store); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
		INSERT INTO purchases (id, company_id, store_id, supplier_id, currency, total_cost, created_by)
		VALUES ($1,$2,$3,$4,'USD',48,$5)
	`, purchase, w.company, w.store, supplier, w.user); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
		INSERT INTO purchase_items (id, company_id, purchase_id, product_id, product_unit_id, quantity, base_quantity, unit_cost, line_cost)
		VALUES ($1,$2,$3,$4,$5,2,48,24,48)
	`, uuid.New(), w.company, purchase, water.ID, carton); err != nil {
		t.Fatal(err)
	}
	recoveredID := uuid.New()
	if _, err := admin.Exec(ctx, `
		INSERT INTO stock_movements (id, company_id, store_id, product_id, quantity_delta, movement_type, ref_type, ref_id, actor_user_id)
		VALUES ($1,$2,$3,$4,48,'receive','purchase',$5,$6)
	`, recoveredID, w.company, w.store, water.ID, purchase, w.user); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `UPDATE product_units SET factor = 20 WHERE id = $1`, carton); err != nil {
		t.Fatal(err)
	}

	ambiguousPurchase := uuid.New()
	if _, err := admin.Exec(ctx, `
		INSERT INTO purchases (id, company_id, store_id, supplier_id, currency, total_cost, created_by)
		VALUES ($1,$2,$3,$4,'USD',20,$5)
	`, ambiguousPurchase, w.company, w.store, supplier, w.user); err != nil {
		t.Fatal(err)
	}
	for _, unit := range []uuid.UUID{bottle, carton} {
		if _, err := admin.Exec(ctx, `
			INSERT INTO purchase_items (id, company_id, purchase_id, product_id, product_unit_id, quantity, base_quantity, unit_cost, line_cost)
			VALUES ($1,$2,$3,$4,$5,1,10,1,10)
		`, uuid.New(), w.company, ambiguousPurchase, water.ID, unit); err != nil {
			t.Fatal(err)
		}
	}
	ambiguousID := uuid.New()
	if _, err := admin.Exec(ctx, `
		INSERT INTO stock_movements (id, company_id, store_id, product_id, quantity_delta, movement_type, ref_type, ref_id)
		VALUES ($1,$2,$3,$4,10,'receive','purchase',$5)
	`, ambiguousID, w.company, w.store, water.ID, ambiguousPurchase); err != nil {
		t.Fatal(err)
	}
	unknownID := uuid.New()
	if _, err := admin.Exec(ctx, `
		INSERT INTO stock_movements (id, company_id, store_id, product_id, quantity_delta, movement_type)
		VALUES ($1,$2,$3,$4,3,'adjustment')
	`, unknownID, w.company, w.store, water.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `SELECT recover_stock_movement_origins()`); err != nil {
		t.Fatal(err)
	}

	var provenance, unitName, qty, factor, delta string
	if err := admin.QueryRow(ctx, `
		SELECT origin_provenance, origin_unit_name, origin_quantity::text, origin_factor::text, quantity_delta::text
		FROM stock_movements WHERE id = $1
	`, recoveredID).Scan(&provenance, &unitName, &qty, &factor, &delta); err != nil {
		t.Fatal(err)
	}
	if provenance != "recovered" || unitName != "carton" || !money.Must(qty).Equal(decimal.NewFromInt(2)) || !money.Must(factor).Equal(decimal.NewFromInt(24)) || !money.Must(delta).Equal(decimal.NewFromInt(48)) {
		t.Fatalf("recovered %s %s %s %s %s", provenance, unitName, qty, factor, delta)
	}
	var ambiguousProvenance string
	var ambiguousUnit *string
	var ambiguousDelta string
	if err := admin.QueryRow(ctx, `
		SELECT origin_provenance, origin_unit_name, quantity_delta::text FROM stock_movements WHERE id = $1
	`, ambiguousID).Scan(&ambiguousProvenance, &ambiguousUnit, &ambiguousDelta); err != nil {
		t.Fatal(err)
	}
	if ambiguousProvenance != "legacy_unknown" || ambiguousUnit != nil || !money.Must(ambiguousDelta).Equal(decimal.NewFromInt(10)) {
		t.Fatalf("ambiguous %s %v %s", ambiguousProvenance, ambiguousUnit, ambiguousDelta)
	}
	var unknownProvenance string
	var unknownUnit *string
	var unknownDelta string
	if err := admin.QueryRow(ctx, `
		SELECT origin_provenance, origin_unit_name, quantity_delta::text FROM stock_movements WHERE id = $1
	`, unknownID).Scan(&unknownProvenance, &unknownUnit, &unknownDelta); err != nil {
		t.Fatal(err)
	}
	if unknownProvenance != "legacy_unknown" || unknownUnit != nil || !money.Must(unknownDelta).Equal(decimal.NewFromInt(3)) {
		t.Fatalf("unknown %s %v %s", unknownProvenance, unknownUnit, unknownDelta)
	}
	var recorded string
	if err := admin.QueryRow(ctx, `
		SELECT origin_provenance FROM stock_movements
		WHERE company_id = $1 AND movement_type = 'opening'
	`, w.company).Scan(&recorded); err != nil || recorded != "recorded" {
		t.Fatalf("opening provenance %s %v", recorded, err)
	}
	if !stockQty(t, admin, w, water.ID).Equal(before) {
		t.Fatalf("recovery changed stock from %s", before)
	}

	reads := readmodel.New(app)
	rows, err := reads.StockMovements(ctx, w.p, readmodel.MovementQuery{StoreID: w.store, ProductID: water.ID, Type: "adjustment"})
	if err != nil || len(rows) != 1 || rows[0].OriginProvenance != "legacy_unknown" || rows[0].OriginUnit != "" {
		t.Fatalf("journal unknown %+v %v", rows, err)
	}
}

func TestMovementJournalFiltersAndPages(t *testing.T) {
	admin, app, identityPool, _ := testutil.Pools(t)
	w := shop(t, admin)
	dest := insertStore(t, admin, w, "Annexe")
	ctx := context.Background()
	svc := commerce.New(app)
	soap := mustProduct(t, svc, w, catalog.CreateInput{
		Name: "Savon", BaseUnit: "pièce", StoreID: w.store,
		Prices: []catalog.PriceInput{{StoreID: w.store, UnitName: "pièce", Currency: "USD", Amount: amount("2")}},
	})
	piece := unitByName(t, soap, "pièce")
	cost := decimal.NewFromInt(1)
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "journal-ouverture", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(40), UnitCost: &cost, Currency: "USD"}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
		IdempotencyKey: "journal-reception", StoreID: w.store, Currency: "USD",
		Lines: []commerce.ReceiveLine{{ProductID: soap.ID, ProductUnitID: &piece, Quantity: decimal.NewFromInt(5), UnitCost: cost}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "journal-vente", soap.ID, "pièce", "2", "2")); err != nil {
		t.Fatal(err)
	}
	keeper := addMember(t, admin, w, "stockkeeper", w.store)
	loss, err := svc.RecordStockLoss(ctx, keeper, commerce.StockLossInput{
		IdempotencyKey: "journal-perte-1", StoreID: w.store, ProductID: soap.ID, ProductUnitID: piece,
		Quantity: decimal.NewFromInt(1), Reason: "damaged",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordStockLoss(ctx, w.p, commerce.StockLossInput{
		IdempotencyKey: "journal-perte-2", StoreID: w.store, ProductID: soap.ID, ProductUnitID: piece,
		Quantity: decimal.NewFromInt(1), Reason: "expired",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordStockLoss(ctx, w.p, commerce.StockLossInput{
		IdempotencyKey: "journal-perte-3", StoreID: w.store, ProductID: soap.ID, ProductUnitID: piece,
		Quantity: decimal.NewFromInt(1), Reason: "missing",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReturnStockToSupplier(ctx, w.p, commerce.SupplierReturnInput{
		IdempotencyKey: "journal-retour", StoreID: w.store, Reason: "invendu",
		Lines: []commerce.SupplierReturnLine{{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(1)}},
	}); err != nil {
		t.Fatal(err)
	}
	draft, err := svc.CreateStockTransfer(ctx, w.p, commerce.StockTransferInput{
		IdempotencyKey: "journal-transfert", SourceStoreID: w.store, DestinationStoreID: dest,
		Lines: []commerce.StockTransferLine{{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(2)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	transferID := uuid.MustParse(draft.ID)
	if _, err := svc.DispatchStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "journal-expedition", TransferID: transferID}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReceiveStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "journal-arrivee", TransferID: transferID}); err != nil {
		t.Fatal(err)
	}
	session, err := svc.StartStockInventory(ctx, w.p, commerce.StartInventoryInput{StoreID: w.store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordInventoryCount(ctx, w.p, commerce.RecordCountInput{
		InventoryID: session.ID, ProductID: soap.ID, ProductUnitID: piece, CountedQuantity: decimal.NewFromInt(30),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CompleteStockInventory(ctx, w.p, commerce.FinishInventoryInput{InventoryID: session.ID}); err != nil {
		t.Fatal(err)
	}

	reads := readmodel.New(app)
	for _, kind := range []string{"opening", "receipt", "sale", "loss", "purchase_return", "transfer_out", "inventory_adjustment"} {
		rows, err := reads.StockMovements(ctx, w.p, readmodel.MovementQuery{StoreID: w.store, ProductID: soap.ID, Type: kind})
		if err != nil || len(rows) == 0 || rows[0].Type != kind || rows[0].OriginProvenance == "" {
			t.Fatalf("%s %+v %v", kind, rows, err)
		}
	}
	arrived, err := reads.StockMovements(ctx, w.p, readmodel.MovementQuery{StoreID: dest, ProductID: soap.ID, Type: "transfer_in"})
	if err != nil || len(arrived) != 1 || arrived[0].OriginProvenance != "recorded" {
		t.Fatalf("transfer_in %+v %v", arrived, err)
	}
	byKeeper, err := reads.StockMovements(ctx, w.p, readmodel.MovementQuery{StoreID: w.store, ProductID: soap.ID, Type: "loss", ActorID: keeper.UserID})
	if err != nil || len(byKeeper) != 1 || byKeeper[0].ID.String() != loss.ID {
		t.Fatalf("actor %+v %v", byKeeper, err)
	}
	first, err := reads.StockMovements(ctx, w.p, readmodel.MovementQuery{StoreID: w.store, ProductID: soap.ID, Type: "loss", Page: readmodel.Page{Limit: 1}})
	if err != nil || len(first) != 1 {
		t.Fatal(err)
	}
	second, err := reads.StockMovements(ctx, w.p, readmodel.MovementQuery{
		StoreID: w.store, ProductID: soap.ID, Type: "loss",
		Page: readmodel.Page{Limit: 1, After: first[0].CreatedAt, AfterID: first[0].ID},
	})
	if err != nil || len(second) != 1 || second[0].ID == first[0].ID {
		t.Fatalf("page %+v %v", second, err)
	}
	if _, err := svc.RecordStockLoss(ctx, w.p, commerce.StockLossInput{
		IdempotencyKey: "journal-perte-nouvelle", StoreID: w.store, ProductID: soap.ID, ProductUnitID: piece,
		Quantity: decimal.NewFromInt(1), Reason: "other",
	}); err != nil {
		t.Fatal(err)
	}
	stable, err := reads.StockMovements(ctx, w.p, readmodel.MovementQuery{
		StoreID: w.store, ProductID: soap.ID, Type: "loss",
		Page: readmodel.Page{Limit: 1, After: first[0].CreatedAt, AfterID: first[0].ID},
	})
	if err != nil || len(stable) != 1 || stable[0].ID != second[0].ID {
		t.Fatalf("stable %+v want %s", stable, second[0].ID)
	}
	period, err := reads.StockMovements(ctx, w.p, readmodel.MovementQuery{
		StoreID: w.store, ProductID: soap.ID, Type: "loss", ActorID: keeper.UserID,
		From: byKeeper[0].CreatedAt, To: byKeeper[0].CreatedAt,
	})
	if err != nil || len(period) != 1 || period[0].ID != byKeeper[0].ID {
		t.Fatalf("period %+v %v", period, err)
	}
	if _, err := reads.StockMovements(ctx, w.p, readmodel.MovementQuery{
		StoreID: w.store, From: time.Now().Add(time.Hour), To: time.Now().Add(-time.Hour),
	}); reasonOf(err) != "period" {
		t.Fatalf("bad period %v", err)
	}

	schema, err := api.Schema(api.Deps{Commerce: svc, Reads: reads, Identity: identity.New(identityPool)})
	if err != nil {
		t.Fatal(err)
	}
	view := api.Execute(api.WithPrincipal(ctx, w.p), schema, `query ($store: ID!, $actor: ID!) {
		stockMovements(storeId: $store, type: "loss", actorId: $actor, limit: 10) { type originProvenance actorId }
	}`, map[string]any{"store": w.store.String(), "actor": keeper.UserID.String()})
	if len(view.Errors) > 0 {
		t.Fatal(view.Errors)
	}
	body := view.Data.(map[string]any)["stockMovements"].([]any)
	if len(body) != 1 || body[0].(map[string]any)["originProvenance"] != "recorded" {
		t.Fatalf("%+v", body)
	}
	for key := range body[0].(map[string]any) {
		if key == "exitValue" || key == "unitCost" || key == "avgCost" {
			t.Fatalf("cost field %s", key)
		}
	}
}

func TestStockValuationTransitLossAndCurrencies(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := shop(t, admin)
	dest := insertStore(t, admin, w, "Annexe")
	ctx := context.Background()
	svc := commerce.New(app)
	oil := mustProduct(t, svc, w, catalog.CreateInput{
		Name: "Huile", BaseUnit: "bidon", StoreID: w.store,
		Prices: []catalog.PriceInput{{StoreID: w.store, UnitName: "bidon", Currency: "USD", Amount: amount("5")}},
	})
	soap := mustProduct(t, svc, w, catalog.CreateInput{
		Name: "Savon CDF", BaseUnit: "pièce", StoreID: w.store,
		Prices: []catalog.PriceInput{{StoreID: w.store, UnitName: "pièce", Currency: "CDF", Amount: amount("1000")}},
	})
	oilUnit := unitByName(t, oil, "bidon")
	soapUnit := unitByName(t, soap, "pièce")
	cost := decimal.NewFromInt(4)
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "valeur-source", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: oil.ID, ProductUnitID: oilUnit, Quantity: decimal.NewFromInt(100), UnitCost: &cost, Currency: "USD"}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "valeur-dest", StoreID: dest,
		Lines: []commerce.OpeningLine{{ProductID: oil.ID, ProductUnitID: oilUnit, Quantity: decimal.NewFromInt(20), UnitCost: &cost, Currency: "USD"}},
	}); err != nil {
		t.Fatal(err)
	}
	reads := readmodel.New(app)
	hidden, err := reads.StockSummary(ctx, w.p, uuid.Nil)
	if err != nil || hidden.CostVisible || hidden.ProductsInStock < 1 {
		t.Fatalf("essentiel %+v %v", hidden, err)
	}
	if _, err := admin.Exec(ctx, `UPDATE subscription_snapshots SET plan_code = 'pro' WHERE company_id = $1`, w.company); err != nil {
		t.Fatal(err)
	}
	if !summaryAmount(t, reads, w, uuid.Nil, "USD").Equal(decimal.NewFromInt(480)) {
		t.Fatalf("opening %s", summaryAmount(t, reads, w, uuid.Nil, "USD"))
	}
	if _, err := svc.RecordStockLoss(ctx, w.p, commerce.StockLossInput{
		IdempotencyKey: "valeur-perte", StoreID: w.store, ProductID: oil.ID, ProductUnitID: oilUnit,
		Quantity: decimal.NewFromInt(10), Reason: "damaged",
	}); err != nil {
		t.Fatal(err)
	}
	if !summaryAmount(t, reads, w, uuid.Nil, "USD").Equal(decimal.NewFromInt(440)) {
		t.Fatalf("after loss %s", summaryAmount(t, reads, w, uuid.Nil, "USD"))
	}
	var exitBefore string
	if err := admin.QueryRow(ctx, `
		SELECT exit_value::text FROM stock_movements
		WHERE company_id = $1 AND movement_type = 'loss'
	`, w.company).Scan(&exitBefore); err != nil {
		t.Fatal(err)
	}
	later := decimal.NewFromInt(10)
	if _, err := svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
		IdempotencyKey: "valeur-reception", StoreID: w.store, Currency: "USD",
		Lines: []commerce.ReceiveLine{{ProductID: oil.ID, ProductUnitID: &oilUnit, Quantity: decimal.NewFromInt(10), UnitCost: later}},
	}); err != nil {
		t.Fatal(err)
	}
	var exitAfter string
	if err := admin.QueryRow(ctx, `
		SELECT exit_value::text FROM stock_movements
		WHERE company_id = $1 AND movement_type = 'loss'
	`, w.company).Scan(&exitAfter); err != nil || exitAfter != exitBefore || !money.Must(exitAfter).Equal(decimal.NewFromInt(40)) {
		t.Fatalf("exit rewritten %s -> %s", exitBefore, exitAfter)
	}
	if _, err := svc.ReturnStockToSupplier(ctx, w.p, commerce.SupplierReturnInput{
		IdempotencyKey: "valeur-retour", StoreID: w.store, Reason: "retour",
		Lines: []commerce.SupplierReturnLine{{ProductID: oil.ID, ProductUnitID: oilUnit, Quantity: decimal.NewFromInt(5)}},
	}); err != nil {
		t.Fatal(err)
	}
	beforeTransfer := summaryAmount(t, reads, w, uuid.Nil, "USD")
	draft, err := svc.CreateStockTransfer(ctx, w.p, commerce.StockTransferInput{
		IdempotencyKey: "valeur-transfert", SourceStoreID: w.store, DestinationStoreID: dest,
		Lines: []commerce.StockTransferLine{{ProductID: oil.ID, ProductUnitID: oilUnit, Quantity: decimal.NewFromInt(10)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	transferID := uuid.MustParse(draft.ID)
	if _, err := svc.DispatchStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "valeur-expedition", TransferID: transferID}); err != nil {
		t.Fatal(err)
	}
	if !summaryAmount(t, reads, w, uuid.Nil, "USD").Equal(beforeTransfer) {
		t.Fatalf("dispatch created profit %s -> %s", beforeTransfer, summaryAmount(t, reads, w, uuid.Nil, "USD"))
	}
	sourceValue := summaryAmount(t, reads, w, w.store, "USD")
	destValue := summaryAmount(t, reads, w, dest, "USD")
	if !sourceValue.Add(destValue).Equal(beforeTransfer) {
		t.Fatalf("split source %s dest %s total %s", sourceValue, destValue, beforeTransfer)
	}
	if !destValue.Equal(decimal.NewFromInt(80)) {
		t.Fatalf("destination counted transit %s", destValue)
	}
	if _, err := svc.ReceiveStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "valeur-reception-transfert", TransferID: transferID}); err != nil {
		t.Fatal(err)
	}
	if !summaryAmount(t, reads, w, uuid.Nil, "USD").Equal(beforeTransfer) {
		t.Fatalf("receive created profit %s", summaryAmount(t, reads, w, uuid.Nil, "USD"))
	}
	if _, err := svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
		IdempotencyKey: "valeur-cdf", StoreID: w.store, Currency: "CDF",
		Lines: []commerce.ReceiveLine{{ProductID: soap.ID, ProductUnitID: &soapUnit, Quantity: decimal.NewFromInt(2), UnitCost: decimal.NewFromInt(1000)}},
	}); err != nil {
		t.Fatal(err)
	}
	usd := summaryAmount(t, reads, w, uuid.Nil, "USD")
	cdf := summaryAmount(t, reads, w, uuid.Nil, "CDF")
	if !usd.Equal(beforeTransfer) || !cdf.Equal(decimal.NewFromInt(2000)) || usd.Add(cdf).Equal(usd) {
		t.Fatalf("currencies usd %s cdf %s", usd, cdf)
	}
}

func TestCostVisibilityDoesNotLeak(t *testing.T) {
	admin, app, identityPool, _ := testutil.Pools(t)
	w := shop(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	soap := mustProduct(t, svc, w, catalog.CreateInput{
		Name: "Savon", BaseUnit: "pièce", StoreID: w.store,
		Prices: []catalog.PriceInput{{StoreID: w.store, UnitName: "pièce", Currency: "USD", Amount: amount("2")}},
	})
	piece := unitByName(t, soap, "pièce")
	cost := decimal.NewFromInt(4)
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "fuite-ouverture", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(10), UnitCost: &cost, Currency: "USD"}},
	}); err != nil {
		t.Fatal(err)
	}
	seller := addMember(t, admin, w, "seller", w.store)
	keeper := addMember(t, admin, w, "stockkeeper", w.store)
	reads := readmodel.New(app)
	sellerSummary, err := reads.StockSummary(ctx, seller, w.store)
	if err != nil || sellerSummary.CostVisible || sellerSummary.StockCostValues != nil || sellerSummary.ProductsInStock != 1 {
		t.Fatalf("seller summary %+v %v", sellerSummary, err)
	}
	session, err := svc.StartStockInventory(ctx, keeper, commerce.StartInventoryInput{StoreID: w.store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordInventoryCount(ctx, keeper, commerce.RecordCountInput{
		InventoryID: session.ID, ProductID: soap.ID, ProductUnitID: piece, CountedQuantity: decimal.NewFromInt(8),
	}); err != nil {
		t.Fatal(err)
	}
	hiddenInventory, err := svc.GetStockInventory(ctx, keeper, session.ID)
	if err != nil || hiddenInventory.Lines[0].VarianceValue != "" || hiddenInventory.Lines[0].VarianceCurrency != "" {
		t.Fatalf("keeper inventory %+v %v", hiddenInventory.Lines, err)
	}
	if _, err := admin.Exec(ctx, `UPDATE subscription_snapshots SET plan_code = 'pro' WHERE company_id = $1`, w.company); err != nil {
		t.Fatal(err)
	}
	ownerInventory, err := svc.GetStockInventory(ctx, w.p, session.ID)
	if err != nil || ownerInventory.Lines[0].VarianceValue == "" || ownerInventory.Lines[0].VarianceCurrency != "USD" {
		t.Fatalf("owner inventory %+v %v", ownerInventory.Lines, err)
	}
	stillHidden, err := svc.GetStockInventory(ctx, keeper, session.ID)
	if err != nil || stillHidden.Lines[0].VarianceValue != "" {
		t.Fatalf("keeper pro %+v %v", stillHidden.Lines, err)
	}
	sellerCost, err := svc.ProductCost(ctx, seller, w.store, soap.ID)
	if err != nil || sellerCost != nil {
		t.Fatalf("seller product %+v %v", sellerCost, err)
	}
	ownerCost, err := svc.ProductCost(ctx, w.p, w.store, soap.ID)
	if err != nil || ownerCost == nil || ownerCost.AvgCost == "" {
		t.Fatalf("owner product %+v %v", ownerCost, err)
	}
	csv, err := reports.New(app).ExportMargin(ctx, seller, reports.Filter{
		StoreIDs: []uuid.UUID{w.store}, Currency: "USD", From: time.Now().Add(-time.Hour), To: time.Now().Add(time.Hour),
	})
	if err == nil || csv != "" || strings.Contains(csv, "4") {
		t.Fatalf("seller export %q %v", csv, err)
	}
	foreign := shop(t, admin)
	if _, err := reads.StockMovements(ctx, foreign.p, readmodel.MovementQuery{StoreID: w.store}); reasonOf(err) != "not_found" {
		t.Fatalf("cross journal %v", err)
	}
	if _, err := svc.GetStockInventory(ctx, foreign.p, session.ID); reasonOf(err) != "not_found" {
		t.Fatalf("cross inventory %v", err)
	}

	schema, err := api.Schema(api.Deps{Commerce: svc, Reads: reads, Identity: identity.New(identityPool)})
	if err != nil {
		t.Fatal(err)
	}
	denied := api.Execute(api.WithPrincipal(ctx, seller), schema, `query ($id: ID!, $store: ID!) {
		product(id: $id, storeId: $store) { cost { avgCost currency } }
		stockSummary(storeId: $store) { productsInStock lowStockProducts stockCostValues { amount } incompleteCostProducts }
		stockMovements(storeId: $store) { type quantity originProvenance }
	}`, map[string]any{"id": soap.ID.String(), "store": w.store.String()})
	if len(denied.Errors) > 0 {
		t.Fatal(denied.Errors)
	}
	data := denied.Data.(map[string]any)
	product := data["product"].(map[string]any)
	if product["cost"] != nil {
		t.Fatalf("nested cost %+v", product["cost"])
	}
	summary := data["stockSummary"].(map[string]any)
	if summary["stockCostValues"] != nil || summary["incompleteCostProducts"] != nil || summary["productsInStock"] != 1 {
		t.Fatalf("summary leak %+v", summary)
	}
	if _, err := svc.RecordStockLoss(ctx, seller, commerce.StockLossInput{
		IdempotencyKey: "fuite-perte", StoreID: w.store, ProductID: soap.ID, ProductUnitID: piece,
		Quantity: decimal.NewFromInt(1), Reason: "stolen",
	}); err == nil || strings.Contains(err.Error(), "4") {
		t.Fatalf("error leaked cost: %v", err)
	}
}

func summaryAmount(t *testing.T, reads *readmodel.Service, w world, store uuid.UUID, currency string) decimal.Decimal {
	t.Helper()
	summary, err := reads.StockSummary(context.Background(), w.p, store)
	if err != nil || !summary.CostVisible {
		t.Fatalf("summary %+v %v", summary, err)
	}
	for _, item := range summary.StockCostValues {
		if item.Currency == currency {
			return money.Must(item.Amount)
		}
	}
	t.Fatalf("missing %s in %+v", currency, summary.StockCostValues)
	return decimal.Zero
}
