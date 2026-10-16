package integration

import (
	"context"
	"sync"
	"testing"

	"rangoapp/internal/easystock/api"
	"rangoapp/internal/easystock/catalog"
	"rangoapp/internal/easystock/commerce"
	"rangoapp/internal/easystock/identity"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/readmodel"
	"rangoapp/internal/easystock/testutil"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

func TestSupplierReturnStockValueAndReplay(t *testing.T) {
	admin, app, identityPool, _ := testutil.Pools(t)
	w := shop(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	soap := mustProduct(t, svc, w, catalog.CreateInput{
		Name: "Savon", BaseUnit: "pièce", StoreID: w.store,
		Prices: []catalog.PriceInput{{StoreID: w.store, UnitName: "pièce", Currency: "USD", Amount: amount("1")}},
	})
	water := mustProduct(t, svc, w, catalog.CreateInput{
		Name: "Eau", BaseUnit: "bouteille", StoreID: w.store,
		Units:  []catalog.UnitInput{{Name: "carton", Factor: decimal.NewFromInt(24), HasFactor: true}},
		Prices: []catalog.PriceInput{{StoreID: w.store, UnitName: "bouteille", Currency: "USD", Amount: amount("0.25")}},
	})
	piece := unitByName(t, soap, "pièce")
	carton := unitByName(t, water, "carton")
	soapCost := decimal.NewFromInt(4)
	waterCost := decimal.NewFromInt(24)
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "ouverture-retour", StoreID: w.store,
		Lines: []commerce.OpeningLine{
			{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(15), UnitCost: &soapCost, Currency: "USD"},
			{ProductID: water.ID, ProductUnitID: carton, Quantity: decimal.NewFromInt(10), UnitCost: &waterCost, Currency: "USD"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReturnStockToSupplier(ctx, w.p, commerce.SupplierReturnInput{
		IdempotencyKey: "retour-zero", StoreID: w.store, Reason: "erreur",
		Lines: []commerce.SupplierReturnLine{{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.Zero}},
	}); reasonOf(err) != "quantity" {
		t.Fatalf("zero = %v", err)
	}
	if _, err := svc.ReturnStockToSupplier(ctx, w.p, commerce.SupplierReturnInput{
		IdempotencyKey: "retour-trop", StoreID: w.store, Reason: "casse",
		Lines: []commerce.SupplierReturnLine{
			{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(2)},
			{ProductID: water.ID, ProductUnitID: carton, Quantity: decimal.NewFromInt(100)},
		},
	}); reasonOf(err) != "stock_conflict" {
		t.Fatalf("above stock = %v", err)
	}
	if stockQty(t, admin, w, soap.ID).Equal(decimal.NewFromInt(15)) == false || stockQty(t, admin, w, water.ID).Equal(decimal.NewFromInt(240)) == false {
		t.Fatal("failed return changed stock")
	}
	var returns int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM stock_supplier_returns WHERE company_id = $1`, w.company).Scan(&returns); err != nil || returns != 0 {
		t.Fatalf("partial return rows=%d err=%v", returns, err)
	}

	returned, err := svc.ReturnStockToSupplier(ctx, w.p, commerce.SupplierReturnInput{
		IdempotencyKey: "retour-ok", StoreID: w.store, SupplierName: "Brasserie", Reference: "AV-14",
		Reason: "Produits abîmés",
		Lines: []commerce.SupplierReturnLine{
			{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(2)},
			{ProductID: water.ID, ProductUnitID: carton, Quantity: decimal.NewFromInt(2)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if returned.Reason != "Produits abîmés" || returned.Reference != "AV-14" || returned.SupplierID == "" || len(returned.Lines) != 2 {
		t.Fatalf("%+v", returned)
	}
	if stockQty(t, admin, w, soap.ID).Equal(decimal.NewFromInt(13)) == false {
		t.Fatalf("soap %s", stockQty(t, admin, w, soap.ID))
	}
	if stockQty(t, admin, w, water.ID).Equal(decimal.NewFromInt(192)) == false {
		t.Fatalf("water %s", stockQty(t, admin, w, water.ID))
	}
	assertExit(t, admin, returned.ReturnID, piece, "8")
	assertExit(t, admin, returned.ReturnID, carton, "48")
	var movementType, originName, originQty, delta string
	if err := admin.QueryRow(ctx, `
		SELECT movement_type, quantity_delta::text, origin_unit_name, origin_quantity::text
		FROM stock_movements
		WHERE ref_type = 'stock_supplier_return' AND ref_id = $1 AND product_id = $2
	`, uuid.MustParse(returned.ReturnID), water.ID).Scan(&movementType, &delta, &originName, &originQty); err != nil {
		t.Fatal(err)
	}
	if movementType != "purchase_return" || originName != "carton" || !money.Must(delta).Equal(decimal.NewFromInt(-48)) || !money.Must(originQty).Equal(decimal.NewFromInt(2)) {
		t.Fatalf("movement %s %s %s %s", movementType, delta, originName, originQty)
	}
	again, err := svc.ReturnStockToSupplier(ctx, w.p, commerce.SupplierReturnInput{
		IdempotencyKey: "retour-ok", StoreID: w.store, SupplierName: "Brasserie", Reference: "AV-14",
		Reason: "Produits abîmés",
		Lines: []commerce.SupplierReturnLine{
			{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(2)},
			{ProductID: water.ID, ProductUnitID: carton, Quantity: decimal.NewFromInt(2)},
		},
	})
	if err != nil || again.ReturnID != returned.ReturnID {
		t.Fatalf("replay %+v %v", again, err)
	}
	if stockQty(t, admin, w, soap.ID).Equal(decimal.NewFromInt(13)) == false {
		t.Fatal("replay changed stock")
	}
	if _, err := svc.ReturnStockToSupplier(ctx, w.p, commerce.SupplierReturnInput{
		IdempotencyKey: "retour-ok", StoreID: w.store, Reason: "autre",
		Lines: []commerce.SupplierReturnLine{{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(1)}},
	}); reasonOf(err) != "idempotency_mismatch" {
		t.Fatalf("mismatch = %v", err)
	}
	if _, err := svc.UpdateCatalogUnit(ctx, w.p, carton, catalog.UnitInput{Name: "carton", Factor: decimal.NewFromInt(20), HasFactor: true}); reasonOf(err) != "factor_locked" {
		t.Fatalf("factor = %v", err)
	}
	assertNoMoneyFlow(t, admin, w.company)

	schema, err := api.Schema(api.Deps{Commerce: svc, Reads: readmodel.New(app), Identity: identity.New(identityPool)})
	if err != nil {
		t.Fatal(err)
	}
	result := api.Execute(api.WithPrincipal(ctx, w.p), schema, `mutation ($input: SupplierReturnInput!) {
		returnStockToSupplier(input: $input) { id reason lines { baseQuantity unitName } }
	}`, map[string]any{"input": map[string]any{
		"idempotencyKey": "retour-ok", "storeId": w.store.String(), "supplierName": "Brasserie",
		"reference": "AV-14", "reason": "Produits abîmés",
		"lines": []any{
			map[string]any{"productId": soap.ID.String(), "productUnitId": piece.String(), "quantity": "2"},
			map[string]any{"productId": water.ID.String(), "productUnitId": carton.String(), "quantity": "2"},
		},
	}})
	if len(result.Errors) > 0 {
		t.Fatal(result.Errors)
	}
	body := result.Data.(map[string]any)["returnStockToSupplier"].(map[string]any)
	if body["id"] != returned.ReturnID {
		t.Fatalf("graphql replay %v", body["id"])
	}
	journal := api.Execute(api.WithPrincipal(ctx, w.p), schema, `query ($store: ID!) {
		stockMovements(storeId: $store, type: "purchase_return") { type quantity originUnit reason }
	}`, map[string]any{"store": w.store.String()})
	if len(journal.Errors) > 0 {
		t.Fatal(journal.Errors)
	}
	rows := journal.Data.(map[string]any)["stockMovements"].([]any)
	if len(rows) != 2 {
		t.Fatalf("journal %+v", rows)
	}
}

func TestStockTransferDispatchReceiveAndValue(t *testing.T) {
	admin, app, identityPool, _ := testutil.Pools(t)
	w := shop(t, admin)
	dest := insertStore(t, admin, w, "Annexe")
	ctx := context.Background()
	svc := commerce.New(app)
	oil := mustProduct(t, svc, w, catalog.CreateInput{
		Name: "Huile", BaseUnit: "bidon", StoreID: w.store,
		Prices: []catalog.PriceInput{{StoreID: w.store, UnitName: "bidon", Currency: "USD", Amount: amount("5")}},
	})
	unit := unitByName(t, oil, "bidon")
	cost := decimal.NewFromInt(4)
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "ouverture-source", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: oil.ID, ProductUnitID: unit, Quantity: decimal.NewFromInt(100), UnitCost: &cost, Currency: "USD"}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "ouverture-dest", StoreID: dest,
		Lines: []commerce.OpeningLine{{ProductID: oil.ID, ProductUnitID: unit, Quantity: decimal.NewFromInt(20), UnitCost: &cost, Currency: "USD"}},
	}); err != nil {
		t.Fatal(err)
	}
	if !heldValue(t, admin, w.company).Equal(decimal.NewFromInt(480)) {
		t.Fatalf("opening value %s", heldValue(t, admin, w.company))
	}
	if _, err := svc.CreateStockTransfer(ctx, w.p, commerce.StockTransferInput{
		IdempotencyKey: "meme-boutique", SourceStoreID: w.store, DestinationStoreID: w.store,
		Lines: []commerce.StockTransferLine{{ProductID: oil.ID, ProductUnitID: unit, Quantity: decimal.NewFromInt(1)}},
	}); reasonOf(err) != "transfer_store" {
		t.Fatalf("same store = %v", err)
	}
	draft, err := svc.CreateStockTransfer(ctx, w.p, commerce.StockTransferInput{
		IdempotencyKey: "transfert-huile", SourceStoreID: w.store, DestinationStoreID: dest,
		Reference: "TR-1", Note: "Réassort",
		Lines: []commerce.StockTransferLine{{ProductID: oil.ID, ProductUnitID: unit, Quantity: decimal.NewFromInt(10)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if draft.Status != "draft" || draft.Version != 1 || draft.InTransit != "0" || len(draft.Events) != 1 {
		t.Fatalf("draft %+v", draft)
	}
	if storeQty(t, admin, w.company, w.store, oil.ID).Equal(decimal.NewFromInt(100)) == false || storeQty(t, admin, w.company, dest, oil.ID).Equal(decimal.NewFromInt(20)) == false {
		t.Fatal("draft moved stock")
	}
	replay, err := svc.CreateStockTransfer(ctx, w.p, commerce.StockTransferInput{
		IdempotencyKey: "transfert-huile", SourceStoreID: w.store, DestinationStoreID: dest,
		Reference: "TR-1", Note: "Réassort",
		Lines: []commerce.StockTransferLine{{ProductID: oil.ID, ProductUnitID: unit, Quantity: decimal.NewFromInt(10)}},
	})
	if err != nil || replay.ID != draft.ID {
		t.Fatalf("create replay %+v %v", replay, err)
	}
	transferID := uuid.MustParse(draft.ID)
	if _, err := svc.ReceiveStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "reception-trop-tot", TransferID: transferID}); reasonOf(err) != "transfer_status" {
		t.Fatalf("receive draft = %v", err)
	}
	sent, err := svc.DispatchStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "expedition-huile", TransferID: transferID})
	if err != nil {
		t.Fatal(err)
	}
	if sent.Status != "in_transit" || sent.InTransit != "10" || sent.Version != 2 {
		t.Fatalf("dispatch %+v", sent)
	}
	if storeQty(t, admin, w.company, w.store, oil.ID).Equal(decimal.NewFromInt(90)) == false || storeQty(t, admin, w.company, dest, oil.ID).Equal(decimal.NewFromInt(20)) == false {
		t.Fatalf("in transit source/dest")
	}
	if !heldValue(t, admin, w.company).Equal(decimal.NewFromInt(480)) {
		t.Fatalf("dispatch value %s", heldValue(t, admin, w.company))
	}
	again, err := svc.DispatchStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "expedition-huile", TransferID: transferID})
	if err != nil || again.Version != 2 {
		t.Fatalf("dispatch replay %+v %v", again, err)
	}
	if _, err := svc.DispatchStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "expedition-bis", TransferID: transferID}); reasonOf(err) != "transfer_dispatched" {
		t.Fatalf("double dispatch = %v", err)
	}
	if storeQty(t, admin, w.company, w.store, oil.ID).Equal(decimal.NewFromInt(90)) == false {
		t.Fatal("double dispatch changed stock")
	}
	got, err := svc.ReceiveStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "reception-huile", TransferID: transferID})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "received" || got.InTransit != "0" || got.Version != 3 || len(got.Events) != 3 {
		t.Fatalf("receive %+v", got)
	}
	if storeQty(t, admin, w.company, w.store, oil.ID).Equal(decimal.NewFromInt(90)) == false || storeQty(t, admin, w.company, dest, oil.ID).Equal(decimal.NewFromInt(30)) == false {
		t.Fatal("received quantities")
	}
	if !heldValue(t, admin, w.company).Equal(decimal.NewFromInt(480)) {
		t.Fatalf("receive value %s", heldValue(t, admin, w.company))
	}
	if _, err := svc.ReceiveStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "reception-bis", TransferID: transferID}); reasonOf(err) != "transfer_received" {
		t.Fatalf("double receive = %v", err)
	}
	if _, err := svc.CancelStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "annulation-recu", TransferID: transferID}); reasonOf(err) != "transfer_received" {
		t.Fatalf("cancel received = %v", err)
	}
	var outCount, inCount, sales, cash int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM stock_movements WHERE ref_id = $1 AND movement_type = 'transfer_out'`, transferID).Scan(&outCount); err != nil || outCount != 1 {
		t.Fatalf("transfer_out=%d err=%v", outCount, err)
	}
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM stock_movements WHERE ref_id = $1 AND movement_type = 'transfer_in' AND store_id = $2`, transferID, dest).Scan(&inCount); err != nil || inCount != 1 {
		t.Fatalf("transfer_in=%d err=%v", inCount, err)
	}
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM sales WHERE company_id = $1`, w.company).Scan(&sales); err != nil || sales != 0 {
		t.Fatalf("sales=%d err=%v", sales, err)
	}
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM cash_transactions WHERE company_id = $1`, w.company).Scan(&cash); err != nil || cash != 0 {
		t.Fatalf("cash=%d err=%v", cash, err)
	}

	schema, err := api.Schema(api.Deps{Commerce: svc, Reads: readmodel.New(app), Identity: identity.New(identityPool)})
	if err != nil {
		t.Fatal(err)
	}
	view := api.Execute(api.WithPrincipal(ctx, w.p), schema, `query ($id: ID!) {
		stockTransfer(id: $id) { id status inTransit version }
	}`, map[string]any{"id": draft.ID})
	if len(view.Errors) > 0 {
		t.Fatal(view.Errors)
	}
	seen := view.Data.(map[string]any)["stockTransfer"].(map[string]any)
	if seen["status"] != "received" || seen["inTransit"] != "0" {
		t.Fatalf("graphql %+v", seen)
	}
	sourceJournal := api.Execute(api.WithPrincipal(ctx, w.p), schema, `query ($store: ID!) {
		stockMovements(storeId: $store, type: "transfer_out") { type quantity }
	}`, map[string]any{"store": w.store.String()})
	if len(sourceJournal.Errors) > 0 {
		t.Fatal(sourceJournal.Errors)
	}
	outRows := sourceJournal.Data.(map[string]any)["stockMovements"].([]any)
	if len(outRows) != 1 || outRows[0].(map[string]any)["type"] != "transfer_out" {
		t.Fatalf("journal %+v", outRows)
	}
}

func TestStockTransferCancelRestoresSource(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := shop(t, admin)
	dest := insertStore(t, admin, w, "Annexe")
	ctx := context.Background()
	svc := commerce.New(app)
	oil := mustProduct(t, svc, w, catalog.CreateInput{
		Name: "Huile", BaseUnit: "bidon", StoreID: w.store,
		Prices: []catalog.PriceInput{{StoreID: w.store, UnitName: "bidon", Currency: "USD", Amount: amount("5")}},
	})
	unit := unitByName(t, oil, "bidon")
	cost := decimal.NewFromInt(4)
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "ouverture-annulation", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: oil.ID, ProductUnitID: unit, Quantity: decimal.NewFromInt(30), UnitCost: &cost, Currency: "USD"}},
	}); err != nil {
		t.Fatal(err)
	}
	draft, err := svc.CreateStockTransfer(ctx, w.p, commerce.StockTransferInput{
		IdempotencyKey: "brouillon", SourceStoreID: w.store, DestinationStoreID: dest,
		Lines: []commerce.StockTransferLine{{ProductID: oil.ID, ProductUnitID: unit, Quantity: decimal.NewFromInt(4)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := svc.CancelStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "annule-brouillon", TransferID: uuid.MustParse(draft.ID)})
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != "cancelled" || cancelled.InTransit != "0" {
		t.Fatalf("%+v", cancelled)
	}
	var movements int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM stock_movements WHERE ref_id = $1`, uuid.MustParse(draft.ID)).Scan(&movements); err != nil || movements != 0 {
		t.Fatalf("draft movements=%d err=%v", movements, err)
	}
	if _, err := svc.CancelStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "annule-encore", TransferID: uuid.MustParse(draft.ID)}); reasonOf(err) != "transfer_cancelled" {
		t.Fatalf("second cancel = %v", err)
	}

	sent, err := svc.CreateStockTransfer(ctx, w.p, commerce.StockTransferInput{
		IdempotencyKey: "en-route", SourceStoreID: w.store, DestinationStoreID: dest,
		Lines: []commerce.StockTransferLine{{ProductID: oil.ID, ProductUnitID: unit, Quantity: decimal.NewFromInt(6)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	sentID := uuid.MustParse(sent.ID)
	if _, err := svc.DispatchStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "expedie", TransferID: sentID}); err != nil {
		t.Fatal(err)
	}
	if !heldValue(t, admin, w.company).Equal(decimal.NewFromInt(120)) {
		t.Fatalf("transit value %s", heldValue(t, admin, w.company))
	}
	back, err := svc.CancelStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "retour-source", TransferID: sentID})
	if err != nil {
		t.Fatal(err)
	}
	if back.Status != "cancelled" || back.InTransit != "0" {
		t.Fatalf("%+v", back)
	}
	if storeQty(t, admin, w.company, w.store, oil.ID).Equal(decimal.NewFromInt(30)) == false || !storeMissing(t, admin, w.company, dest, oil.ID) {
		t.Fatal("cancel did not restore source only")
	}
	if !heldValue(t, admin, w.company).Equal(decimal.NewFromInt(120)) {
		t.Fatalf("restored value %s", heldValue(t, admin, w.company))
	}
	var keptOut, returnedIn int
	var reason string
	if err := admin.QueryRow(ctx, `
		SELECT count(*) FROM stock_movements WHERE ref_id = $1 AND movement_type = 'transfer_out'
	`, sentID).Scan(&keptOut); err != nil || keptOut != 1 {
		t.Fatalf("kept out=%d err=%v", keptOut, err)
	}
	if err := admin.QueryRow(ctx, `
		SELECT count(*), COALESCE(max(reason), '')
		FROM stock_movements
		WHERE ref_id = $1 AND movement_type = 'transfer_in' AND store_id = $2
	`, sentID, w.store).Scan(&returnedIn, &reason); err != nil || returnedIn != 1 || reason != "returned_to_source" {
		t.Fatalf("return in=%d reason=%s err=%v", returnedIn, reason, err)
	}
}

func TestStockTransferConcurrentSale(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := shop(t, admin)
	dest := insertStore(t, admin, w, "Annexe")
	ctx := context.Background()
	svc := commerce.New(app)
	soap := mustProduct(t, svc, w, catalog.CreateInput{
		Name: "Savon", BaseUnit: "pièce", StoreID: w.store,
		Prices: []catalog.PriceInput{{StoreID: w.store, UnitName: "pièce", Currency: "USD", Amount: amount("1")}},
	})
	piece := unitByName(t, soap, "pièce")
	cost := decimal.NewFromInt(1)
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "ouverture-course", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(10), UnitCost: &cost, Currency: "USD"}},
	}); err != nil {
		t.Fatal(err)
	}
	draft, err := svc.CreateStockTransfer(ctx, w.p, commerce.StockTransferInput{
		IdempotencyKey: "course", SourceStoreID: w.store, DestinationStoreID: dest,
		Lines: []commerce.StockTransferLine{{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(7)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, errs[0] = svc.ConfirmSale(ctx, w.p, saleOf(w, "vente-course", soap.ID, "pièce", "7", "1"))
	}()
	go func() {
		defer wg.Done()
		_, errs[1] = svc.DispatchStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "expedition-course", TransferID: uuid.MustParse(draft.ID)})
	}()
	wg.Wait()
	success := 0
	for _, err := range errs {
		if err == nil {
			success++
			continue
		}
		if reasonOf(err) != "stock_conflict" {
			t.Fatalf("concurrent = %v", err)
		}
	}
	if success != 1 {
		t.Fatalf("successes = %d errs=%v", success, errs)
	}
	if stockQty(t, admin, w, soap.ID).Equal(decimal.NewFromInt(3)) == false {
		t.Fatalf("remaining %s", stockQty(t, admin, w, soap.ID))
	}
	var negative int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM stock_balances WHERE company_id = $1 AND quantity < 0`, w.company).Scan(&negative); err != nil || negative != 0 {
		t.Fatalf("negative=%d err=%v", negative, err)
	}
}

func TestReturnTransferIsolationAndPermissions(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := shop(t, admin)
	dest := insertStore(t, admin, w, "Annexe")
	ctx := context.Background()
	svc := commerce.New(app)
	soap := mustProduct(t, svc, w, catalog.CreateInput{
		Name: "Savon", BaseUnit: "pièce", StoreID: w.store,
		Prices: []catalog.PriceInput{{StoreID: w.store, UnitName: "pièce", Currency: "USD", Amount: amount("1")}},
	})
	piece := unitByName(t, soap, "pièce")
	cost := decimal.NewFromInt(1)
	seller := addMember(t, admin, w, "seller", w.store)
	if _, err := svc.ReturnStockToSupplier(ctx, seller, commerce.SupplierReturnInput{
		IdempotencyKey: "vendeur-retour", StoreID: w.store, Reason: "refus",
		Lines: []commerce.SupplierReturnLine{{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(1)}},
	}); reasonOf(err) != "stock.return.create" {
		t.Fatalf("seller return = %v", err)
	}
	if _, err := svc.CreateStockTransfer(ctx, seller, commerce.StockTransferInput{
		IdempotencyKey: "vendeur-transfert", SourceStoreID: w.store, DestinationStoreID: dest,
		Lines: []commerce.StockTransferLine{{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(1)}},
	}); reasonOf(err) != "stock.transfer.create" {
		t.Fatalf("seller transfer = %v", err)
	}
	keeper := addMember(t, admin, w, "stockkeeper", w.store)
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "ouverture-droits", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(8), UnitCost: &cost, Currency: "USD"}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReturnStockToSupplier(ctx, keeper, commerce.SupplierReturnInput{
		IdempotencyKey: "magasinier-retour", StoreID: w.store, Reason: "casse",
		Lines: []commerce.SupplierReturnLine{{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(1)}},
	}); err != nil {
		t.Fatal(err)
	}
	draft, err := svc.CreateStockTransfer(ctx, keeper, commerce.StockTransferInput{
		IdempotencyKey: "magasinier-transfert", SourceStoreID: w.store, DestinationStoreID: dest,
		Lines: []commerce.StockTransferLine{{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(2)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	transferID := uuid.MustParse(draft.ID)
	if _, err := svc.DispatchStockTransfer(ctx, keeper, commerce.StockTransferAction{IdempotencyKey: "magasinier-expedie", TransferID: transferID}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReceiveStockTransfer(ctx, keeper, commerce.StockTransferAction{IdempotencyKey: "magasinier-recoit", TransferID: transferID}); reasonOf(err) != "store" {
		t.Fatalf("source keeper receive = %v", err)
	}
	if !storeMissing(t, admin, w.company, dest, soap.ID) {
		t.Fatal("unauthorized receive changed destination")
	}
	foreign := shop(t, admin)
	if _, err := svc.ReturnStockToSupplier(ctx, foreign.p, commerce.SupplierReturnInput{
		IdempotencyKey: "autre-retour", StoreID: w.store, Reason: "vol",
		Lines: []commerce.SupplierReturnLine{{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(1)}},
	}); reasonOf(err) != "not_found" {
		t.Fatalf("cross return = %v", err)
	}
	if _, err := svc.CreateStockTransfer(ctx, foreign.p, commerce.StockTransferInput{
		IdempotencyKey: "autre-transfert", SourceStoreID: w.store, DestinationStoreID: dest,
		Lines: []commerce.StockTransferLine{{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(1)}},
	}); reasonOf(err) != "not_found" {
		t.Fatalf("cross transfer = %v", err)
	}
	if _, err := svc.GetStockTransfer(ctx, foreign.p, transferID); reasonOf(err) != "not_found" {
		t.Fatalf("cross read = %v", err)
	}
	if stockQty(t, admin, w, soap.ID).Equal(decimal.NewFromInt(5)) == false {
		t.Fatalf("isolated stock %s", stockQty(t, admin, w, soap.ID))
	}
}

func TestReturnAndTransferLeaveSaleAndReceiptWorking(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
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
		IdempotencyKey: "ouverture-suite", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(20), UnitCost: &cost, Currency: "USD"}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReturnStockToSupplier(ctx, w.p, commerce.SupplierReturnInput{
		IdempotencyKey: "retour-suite", StoreID: w.store, Reason: "invendu",
		Lines: []commerce.SupplierReturnLine{{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(2)}},
	}); err != nil {
		t.Fatal(err)
	}
	draft, err := svc.CreateStockTransfer(ctx, w.p, commerce.StockTransferInput{
		IdempotencyKey: "transfert-suite", SourceStoreID: w.store, DestinationStoreID: dest,
		Lines: []commerce.StockTransferLine{{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(3)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	transferID := uuid.MustParse(draft.ID)
	if _, err := svc.DispatchStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "expedition-suite", TransferID: transferID}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReceiveStockTransfer(ctx, w.p, commerce.StockTransferAction{IdempotencyKey: "reception-suite", TransferID: transferID}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
		IdempotencyKey: "reception-apres", StoreID: w.store, Currency: "USD",
		Lines: []commerce.ReceiveLine{{ProductID: soap.ID, ProductUnitID: &piece, Quantity: decimal.NewFromInt(4), UnitCost: cost}},
	}); err != nil {
		t.Fatal(err)
	}
	sale, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "vente-apres", soap.ID, "pièce", "5", "2"))
	if err != nil {
		t.Fatal(err)
	}
	if sale.SaleID == uuid.Nil {
		t.Fatal("missing sale")
	}
	if stockQty(t, admin, w, soap.ID).Equal(decimal.NewFromInt(14)) == false {
		t.Fatalf("after sale %s", stockQty(t, admin, w, soap.ID))
	}
	if storeQty(t, admin, w.company, dest, soap.ID).Equal(decimal.NewFromInt(3)) == false {
		t.Fatal("destination changed by the sale")
	}
}

func insertStore(t *testing.T, admin *pgxpool.Pool, w world, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := admin.Exec(context.Background(), `INSERT INTO stores (id, company_id, name) VALUES ($1,$2,$3)`, id, w.company, name); err != nil {
		t.Fatal(err)
	}
	return id
}

func storeQty(t *testing.T, admin *pgxpool.Pool, company, store, product uuid.UUID) decimal.Decimal {
	t.Helper()
	var raw string
	if err := admin.QueryRow(context.Background(), `
		SELECT quantity::text FROM stock_balances WHERE company_id = $1 AND store_id = $2 AND product_id = $3
	`, company, store, product).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	return money.Must(raw)
}

func storeMissing(t *testing.T, admin *pgxpool.Pool, company, store, product uuid.UUID) bool {
	t.Helper()
	var n int
	if err := admin.QueryRow(context.Background(), `
		SELECT count(*) FROM stock_balances WHERE company_id = $1 AND store_id = $2 AND product_id = $3
	`, company, store, product).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 0
}

func heldValue(t *testing.T, admin *pgxpool.Pool, company uuid.UUID) decimal.Decimal {
	t.Helper()
	var balances, transit string
	if err := admin.QueryRow(context.Background(), `
		SELECT COALESCE(SUM(quantity * avg_cost), 0)::text
		FROM stock_balances
		WHERE company_id = $1 AND cost_complete AND avg_cost IS NOT NULL
	`, company).Scan(&balances); err != nil {
		t.Fatal(err)
	}
	if err := admin.QueryRow(context.Background(), `
		SELECT COALESCE(SUM(l.exit_value), 0)::text
		FROM stock_transfer_lines l
		JOIN stock_transfers t ON t.id = l.transfer_id
		WHERE t.company_id = $1 AND t.status = 'in_transit'
	`, company).Scan(&transit); err != nil {
		t.Fatal(err)
	}
	return money.Must(balances).Add(money.Must(transit))
}

func assertExit(t *testing.T, admin *pgxpool.Pool, returnID string, unit uuid.UUID, want string) {
	t.Helper()
	var raw string
	if err := admin.QueryRow(context.Background(), `
		SELECT exit_value::text FROM stock_supplier_return_lines
		WHERE return_id = $1 AND product_unit_id = $2
	`, uuid.MustParse(returnID), unit).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if !money.Must(raw).Equal(money.Must(want)) {
		t.Fatalf("exit %s want %s", raw, want)
	}
}

func assertNoMoneyFlow(t *testing.T, admin *pgxpool.Pool, company uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	for _, table := range []string{"purchases", "sales", "cash_transactions", "supplier_payments"} {
		var n int
		if err := admin.QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE company_id = $1`, company).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s=%d err=%v", table, n, err)
		}
	}
}
