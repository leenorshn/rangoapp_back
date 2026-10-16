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

func TestOpenInitialStockConvertsAndReplays(t *testing.T) {
	admin, app, identityPool, _ := testutil.Pools(t)
	w := shop(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	water := mustProduct(t, svc, w, catalog.CreateInput{
		Name: "Eau minérale", BaseUnit: "bouteille", StoreID: w.store,
		Units: []catalog.UnitInput{{Name: "carton", Factor: decimal.NewFromInt(24), HasFactor: true}},
		Prices: []catalog.PriceInput{
			{StoreID: w.store, UnitName: "bouteille", Currency: "USD", Amount: amount("0.25")},
		},
	})
	sugar := mustProduct(t, svc, w, catalog.CreateInput{
		Name: "Sucre", BaseUnit: "kg", AllowFractional: true, StoreID: w.store,
		Units: []catalog.UnitInput{{Name: "sac", Factor: decimal.NewFromInt(25), HasFactor: true}},
		Prices: []catalog.PriceInput{
			{StoreID: w.store, UnitName: "kg", Currency: "USD", Amount: amount("1")},
		},
	})
	soap := mustProduct(t, svc, w, catalog.CreateInput{
		Name: "Savon", BaseUnit: "pièce", StoreID: w.store,
		Prices: []catalog.PriceInput{
			{StoreID: w.store, UnitName: "pièce", Currency: "USD", Amount: amount("1")},
		},
	})
	carton := unitByName(t, water, "carton")
	sac := unitByName(t, sugar, "sac")
	soapUnit := unitByName(t, soap, "pièce")
	waterCost := decimal.NewFromInt(4)
	sugarCost := decimal.NewFromInt(20)
	soapCost := decimal.NewFromInt(1)
	opened, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "ouverture-trio", StoreID: w.store, Note: "Stock déjà en rayon",
		Lines: []commerce.OpeningLine{
			{ProductID: water.ID, ProductUnitID: carton, Quantity: decimal.NewFromInt(10), UnitCost: &waterCost, Currency: "USD"},
			{ProductID: sugar.ID, ProductUnitID: sac, Quantity: decimal.NewFromInt(2), UnitCost: &sugarCost, Currency: "USD"},
			{ProductID: soap.ID, ProductUnitID: soapUnit, Quantity: decimal.NewFromInt(20), UnitCost: &soapCost, Currency: "USD"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(opened.Lines) != 3 || opened.Lines[0].BaseQuantity != "240" || opened.Lines[0].Factor != "24" || opened.Lines[0].UnitName != "carton" {
		t.Fatalf("water line %+v", opened.Lines)
	}
	if opened.Lines[1].BaseQuantity != "50" || opened.Lines[1].Factor != "25" {
		t.Fatalf("sugar line %+v", opened.Lines[1])
	}
	if opened.Lines[2].BaseQuantity != "20" || opened.Note != "Stock déjà en rayon" || opened.CreatedBy != w.p.UserID.String() {
		t.Fatalf("%+v", opened)
	}
	if got := stockQty(t, admin, w, water.ID); !got.Equal(decimal.NewFromInt(240)) {
		t.Fatalf("water %s", got)
	}
	if got := stockQty(t, admin, w, sugar.ID); !got.Equal(decimal.NewFromInt(50)) {
		t.Fatalf("sugar %s", got)
	}
	if got := stockQty(t, admin, w, soap.ID); !got.Equal(decimal.NewFromInt(20)) {
		t.Fatalf("soap %s", got)
	}
	var purchases, sales, cash int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM purchases WHERE company_id = $1`, w.company).Scan(&purchases); err != nil || purchases != 0 {
		t.Fatalf("purchases=%d err=%v", purchases, err)
	}
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM sales WHERE company_id = $1`, w.company).Scan(&sales); err != nil || sales != 0 {
		t.Fatalf("sales=%d err=%v", sales, err)
	}
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM cash_transactions WHERE company_id = $1`, w.company).Scan(&cash); err != nil || cash != 0 {
		t.Fatalf("cash=%d err=%v", cash, err)
	}
	var originFactor, originName, movementType string
	var originQty string
	if err := admin.QueryRow(ctx, `
		SELECT movement_type, origin_unit_name, origin_quantity::text, origin_factor::text
		FROM stock_movements
		WHERE product_id = $1 AND movement_type = 'opening'
	`, water.ID).Scan(&movementType, &originName, &originQty, &originFactor); err != nil {
		t.Fatal(err)
	}
	if originName != "carton" || originQty != "10.000000" && originQty != "10" || originFactor != "24.000000" && originFactor != "24" {
		t.Fatalf("origin %s %s %s", originName, originQty, originFactor)
	}
	again, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "ouverture-trio", StoreID: w.store, Note: "Stock déjà en rayon",
		Lines: []commerce.OpeningLine{
			{ProductID: water.ID, ProductUnitID: carton, Quantity: decimal.NewFromInt(10), UnitCost: &waterCost, Currency: "USD"},
			{ProductID: sugar.ID, ProductUnitID: sac, Quantity: decimal.NewFromInt(2), UnitCost: &sugarCost, Currency: "USD"},
			{ProductID: soap.ID, ProductUnitID: soapUnit, Quantity: decimal.NewFromInt(20), UnitCost: &soapCost, Currency: "USD"},
		},
	})
	if err != nil || again.OpeningID != opened.OpeningID {
		t.Fatalf("replay %+v %v", again, err)
	}
	if got := stockQty(t, admin, w, water.ID); !got.Equal(decimal.NewFromInt(240)) {
		t.Fatalf("replay changed water to %s", got)
	}
	changedCost := decimal.NewFromInt(9)
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "ouverture-trio", StoreID: w.store,
		Lines: []commerce.OpeningLine{
			{ProductID: water.ID, ProductUnitID: carton, Quantity: decimal.NewFromInt(1), UnitCost: &changedCost, Currency: "USD"},
		},
	}); reasonOf(err) != "idempotency_mismatch" {
		t.Fatalf("mismatch = %v", err)
	}
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "ouverture-eau-bis", StoreID: w.store,
		Lines: []commerce.OpeningLine{
			{ProductID: water.ID, ProductUnitID: carton, Quantity: decimal.NewFromInt(1), UnitCost: &waterCost, Currency: "USD"},
		},
	}); reasonOf(err) != "opening_closed" {
		t.Fatalf("second opening = %v", err)
	}
	if _, err := svc.UpdateCatalogUnit(ctx, w.p, carton, catalog.UnitInput{Name: "carton", Factor: decimal.NewFromInt(20), HasFactor: true}); reasonOf(err) != "factor_locked" {
		t.Fatalf("factor lock = %v", err)
	}
	if _, err := svc.UpdateCatalogUnit(ctx, w.p, sac, catalog.UnitInput{Name: "sac", Factor: decimal.NewFromInt(20), HasFactor: true}); reasonOf(err) != "factor_locked" {
		t.Fatalf("sac factor lock = %v", err)
	}

	schema, err := api.Schema(api.Deps{Commerce: svc, Reads: readmodel.New(app), Identity: identity.New(identityPool)})
	if err != nil {
		t.Fatal(err)
	}
	result := api.Execute(api.WithPrincipal(ctx, w.p), schema, `mutation ($input: OpenInitialStockInput!) {
		openInitialStock(input: $input) { id lines { baseQuantity factor } }
	}`, map[string]any{"input": map[string]any{
		"idempotencyKey": "ouverture-trio", "storeId": w.store.String(), "note": "Stock déjà en rayon",
		"lines": []any{
			map[string]any{"productId": water.ID.String(), "productUnitId": carton.String(), "quantity": "10", "unitCost": "4", "currency": "USD"},
			map[string]any{"productId": sugar.ID.String(), "productUnitId": sac.String(), "quantity": "2", "unitCost": "20", "currency": "USD"},
			map[string]any{"productId": soap.ID.String(), "productUnitId": soapUnit.String(), "quantity": "20", "unitCost": "1", "currency": "USD"},
		},
	}})
	if len(result.Errors) > 0 {
		t.Fatal(result.Errors)
	}
	body := result.Data.(map[string]any)["openInitialStock"].(map[string]any)
	if body["id"] != opened.OpeningID {
		t.Fatalf("graphql replay %v", body["id"])
	}
}

func TestOpenInitialStockRefusesHistory(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := shop(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	water := mustProduct(t, svc, w, catalog.CreateInput{
		Name: "Eau", BaseUnit: "bouteille", StoreID: w.store,
		Prices: []catalog.PriceInput{{StoreID: w.store, UnitName: "bouteille", Currency: "USD", Amount: amount("1")}},
	})
	bottle := unitByName(t, water, "bouteille")
	cost := decimal.NewFromInt(1)
	if _, err := svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
		IdempotencyKey: "recu-avant-ouverture", StoreID: w.store, Currency: "USD",
		Lines: []commerce.ReceiveLine{{ProductID: water.ID, ProductUnitID: &bottle, Quantity: decimal.NewFromInt(5), UnitCost: cost}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "ouverture-apres-reception", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: water.ID, ProductUnitID: bottle, Quantity: decimal.NewFromInt(5), UnitCost: &cost, Currency: "USD"}},
	}); reasonOf(err) != "opening_closed" {
		t.Fatalf("after receipt = %v", err)
	}
	if got := stockQty(t, admin, w, water.ID); !got.Equal(decimal.NewFromInt(5)) {
		t.Fatalf("receipt stock changed to %s", got)
	}

	soldOut := mustProduct(t, svc, w, catalog.CreateInput{
		Name: "Savon vendu", BaseUnit: "pièce", StoreID: w.store,
		Prices: []catalog.PriceInput{{StoreID: w.store, UnitName: "pièce", Currency: "USD", Amount: amount("1")}},
	})
	piece := unitByName(t, soldOut, "pièce")
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "ouverture-savon", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: soldOut.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(5), UnitCost: &cost, Currency: "USD"}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "vend-tout", soldOut.ID, "pièce", "5", "1")); err != nil {
		t.Fatal(err)
	}
	if got := stockQty(t, admin, w, soldOut.ID); !got.IsZero() {
		t.Fatalf("expected zero, got %s", got)
	}
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "reouverture-savon", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: soldOut.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(5), UnitCost: &cost, Currency: "USD"}},
	}); reasonOf(err) != "opening_closed" {
		t.Fatalf("zero after sale = %v", err)
	}

	imported := mustProduct(t, svc, w, catalog.CreateInput{Name: "Importé", BaseUnit: "pièce"})
	importedUnit := unitByName(t, imported, "pièce")
	if _, err := admin.Exec(ctx, `
		INSERT INTO stock_balances (company_id, store_id, product_id, quantity, cost_complete)
		VALUES ($1,$2,$3,8,false)
	`, w.company, w.store, imported.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.OpenInitialStock(ctx, w.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "ouverture-import", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: imported.ID, ProductUnitID: importedUnit, Quantity: decimal.NewFromInt(8), UnitCost: &cost, Currency: "USD"}},
	}); reasonOf(err) != "opening_closed" {
		t.Fatalf("existing balance = %v", err)
	}
	if got := stockQty(t, admin, w, imported.ID); !got.Equal(decimal.NewFromInt(8)) {
		t.Fatalf("import stock %s", got)
	}
}

func TestStockLossValueAndExistingFlows(t *testing.T) {
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
		IdempotencyKey: "ouverture-valeur", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(20), UnitCost: &cost, Currency: "USD"}},
	}); err != nil {
		t.Fatal(err)
	}
	qty, avg, currency, complete := balanceState(t, admin, w, soap.ID)
	if qty != "20" && qty != "20.000000" || !avg.Equal(decimal.NewFromInt(4)) || currency != "USD" || !complete {
		t.Fatalf("after opening qty=%s avg=%s %s complete=%v", qty, avg, currency, complete)
	}
	lost, err := svc.RecordStockLoss(ctx, w.p, commerce.StockLossInput{
		IdempotencyKey: "perte-5", StoreID: w.store, ProductID: soap.ID, ProductUnitID: piece,
		Quantity: decimal.NewFromInt(5), Reason: "damaged", Note: "Casse",
	})
	if err != nil || lost.Type != "loss" || lost.Quantity != "-5" || lost.OriginQuantity != "5" || lost.Reason != "damaged" || lost.ActorID != w.p.UserID.String() {
		t.Fatalf("%+v %v", lost, err)
	}
	if got := stockQty(t, admin, w, soap.ID); !got.Equal(decimal.NewFromInt(15)) {
		t.Fatalf("after loss %s", got)
	}
	qty, avg, currency, complete = balanceState(t, admin, w, soap.ID)
	if !avg.Equal(decimal.NewFromInt(4)) || currency != "USD" || !complete {
		t.Fatalf("loss changed cost avg=%s %s complete=%v qty=%s", avg, currency, complete, qty)
	}
	if !moneyMust(t, qty).Mul(avg).Equal(decimal.NewFromInt(60)) {
		t.Fatalf("remaining value qty=%s avg=%s", qty, avg)
	}
	var exitValue, exitCurrency string
	if err := admin.QueryRow(ctx, `
		SELECT exit_value::text, exit_currency FROM stock_movements WHERE id = $1
	`, uuid.MustParse(lost.ID)).Scan(&exitValue, &exitCurrency); err != nil {
		t.Fatal(err)
	}
	if !moneyEqual(t, exitValue, "20") || exitCurrency != "USD" {
		t.Fatalf("exit %s %s", exitValue, exitCurrency)
	}
	var cash, sales int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM cash_transactions WHERE company_id = $1`, w.company).Scan(&cash); err != nil || cash != 0 {
		t.Fatalf("cash=%d err=%v", cash, err)
	}
	replay, err := svc.RecordStockLoss(ctx, w.p, commerce.StockLossInput{
		IdempotencyKey: "perte-5", StoreID: w.store, ProductID: soap.ID, ProductUnitID: piece,
		Quantity: decimal.NewFromInt(5), Reason: "damaged", Note: "Casse",
	})
	if err != nil || replay.ID != lost.ID {
		t.Fatalf("replay %+v %v", replay, err)
	}
	if got := stockQty(t, admin, w, soap.ID); !got.Equal(decimal.NewFromInt(15)) {
		t.Fatalf("replay stock %s", got)
	}
	if _, err := svc.RecordStockLoss(ctx, w.p, commerce.StockLossInput{
		IdempotencyKey: "perte-5", StoreID: w.store, ProductID: soap.ID, ProductUnitID: piece,
		Quantity: decimal.NewFromInt(1), Reason: "stolen",
	}); reasonOf(err) != "idempotency_mismatch" {
		t.Fatalf("mismatch = %v", err)
	}
	if _, err := svc.RecordStockLoss(ctx, w.p, commerce.StockLossInput{
		IdempotencyKey: "perte-trop", StoreID: w.store, ProductID: soap.ID, ProductUnitID: piece,
		Quantity: decimal.NewFromInt(16), Reason: "missing",
	}); reasonOf(err) != "stock_conflict" {
		t.Fatalf("too much = %v", err)
	}
	if got := stockQty(t, admin, w, soap.ID); !got.Equal(decimal.NewFromInt(15)) {
		t.Fatalf("rejected loss changed stock to %s", got)
	}
	if _, err := svc.RecordStockLoss(ctx, w.p, commerce.StockLossInput{
		IdempotencyKey: "perte-sans-motif", StoreID: w.store, ProductID: soap.ID, ProductUnitID: piece,
		Quantity: decimal.NewFromInt(1),
	}); reasonOf(err) != "reason" {
		t.Fatalf("reason = %v", err)
	}

	sale, err := svc.ConfirmSale(ctx, w.p, saleOf(w, "vend-savon", soap.ID, "pièce", "1", "2"))
	if err != nil || sale.Net != "2" {
		t.Fatalf("sale %+v %v", sale, err)
	}
	var net string
	if err := admin.QueryRow(ctx, `SELECT net_amount::text FROM sales WHERE id = $1`, sale.SaleID).Scan(&net); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordStockLoss(ctx, w.p, commerce.StockLossInput{
		IdempotencyKey: "perte-apres-vente", StoreID: w.store, ProductID: soap.ID, ProductUnitID: piece,
		Quantity: decimal.NewFromInt(1), Reason: "expired",
	}); err != nil {
		t.Fatal(err)
	}
	var netAfter string
	if err := admin.QueryRow(ctx, `SELECT net_amount::text FROM sales WHERE id = $1`, sale.SaleID).Scan(&netAfter); err != nil || netAfter != net {
		t.Fatalf("sale changed %s -> %s err=%v", net, netAfter, err)
	}
	if got := stockQty(t, admin, w, soap.ID); !got.Equal(decimal.NewFromInt(13)) {
		t.Fatalf("after sale and loss %s", got)
	}
	if _, err := svc.ReceiveStock(ctx, w.p, commerce.ReceiveStockInput{
		IdempotencyKey: "reception-apres-ouverture", StoreID: w.store, Currency: "USD",
		Lines: []commerce.ReceiveLine{{ProductID: soap.ID, ProductUnitID: &piece, Quantity: decimal.NewFromInt(2), UnitCost: decimal.NewFromInt(4)}},
	}); err != nil {
		t.Fatal(err)
	}
	if got := stockQty(t, admin, w, soap.ID); !got.Equal(decimal.NewFromInt(15)) {
		t.Fatalf("after receipt %s", got)
	}
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM sales WHERE company_id = $1`, w.company).Scan(&sales); err != nil || sales != 1 {
		t.Fatalf("sales=%d err=%v", sales, err)
	}
	var purchases int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM purchases WHERE company_id = $1`, w.company).Scan(&purchases); err != nil || purchases != 1 {
		t.Fatalf("purchases=%d err=%v", purchases, err)
	}

	if _, err := admin.Exec(ctx, `UPDATE subscription_snapshots SET plan_code = 'pro' WHERE company_id = $1`, w.company); err != nil {
		t.Fatal(err)
	}
	summary, err := readmodel.New(app).StockSummary(ctx, w.p, w.store)
	if err != nil || !summary.CostVisible || len(summary.StockCostValues) != 1 || summary.StockCostValues[0].Currency != "USD" {
		t.Fatalf("%+v %v", summary, err)
	}
	if !moneyEqual(t, summary.StockCostValues[0].Amount, "60") {
		t.Fatalf("summary cost %s", summary.StockCostValues[0].Amount)
	}

	schema, err := api.Schema(api.Deps{Commerce: svc, Reads: readmodel.New(app), Identity: identity.New(identityPool)})
	if err != nil {
		t.Fatal(err)
	}
	view := api.Execute(api.WithPrincipal(ctx, w.p), schema, `mutation ($input: StockLossInput!) {
		recordStockLoss(input: $input) { id type quantity originUnit originQuantity reason }
	}`, map[string]any{"input": map[string]any{
		"idempotencyKey": "perte-graphql", "storeId": w.store.String(), "productId": soap.ID.String(),
		"productUnitId": piece.String(), "quantity": "1", "reason": "other",
	}})
	if len(view.Errors) > 0 {
		t.Fatal(view.Errors)
	}
	row := view.Data.(map[string]any)["recordStockLoss"].(map[string]any)
	if row["type"] != "loss" || row["quantity"] != "-1" || row["originQuantity"] != "1" || row["reason"] != "other" {
		t.Fatalf("%v", row)
	}
	journal, err := readmodel.New(app).StockMovements(ctx, w.p, readmodel.MovementQuery{StoreID: w.store, ProductID: soap.ID, Type: "loss"})
	if err != nil || len(journal) != 3 || journal[0].Type != "loss" {
		t.Fatalf("%+v %v", journal, err)
	}
}

func TestConcurrentLossAndSale(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := shop(t, admin)
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
	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, errs[0] = svc.ConfirmSale(ctx, w.p, saleOf(w, "vente-course", soap.ID, "pièce", "7", "1"))
	}()
	go func() {
		defer wg.Done()
		_, errs[1] = svc.RecordStockLoss(ctx, w.p, commerce.StockLossInput{
			IdempotencyKey: "perte-course", StoreID: w.store, ProductID: soap.ID, ProductUnitID: piece,
			Quantity: decimal.NewFromInt(7), Reason: "stolen",
		})
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
	if got := stockQty(t, admin, w, soap.ID); !got.Equal(decimal.NewFromInt(3)) {
		t.Fatalf("concurrent stock %s", got)
	}
	var negative int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM stock_balances WHERE company_id = $1 AND quantity < 0`, w.company).Scan(&negative); err != nil || negative != 0 {
		t.Fatalf("negative=%d err=%v", negative, err)
	}
}

func TestOpeningAndLossIsolationAndPermissions(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := shop(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	soap := mustProduct(t, svc, w, catalog.CreateInput{
		Name: "Savon", BaseUnit: "pièce", StoreID: w.store,
		Prices: []catalog.PriceInput{{StoreID: w.store, UnitName: "pièce", Currency: "USD", Amount: amount("1")}},
	})
	piece := unitByName(t, soap, "pièce")
	cost := decimal.NewFromInt(1)
	seller := addMember(t, admin, w, "seller", w.store)
	if _, err := svc.OpenInitialStock(ctx, seller, commerce.OpenInitialStockInput{
		IdempotencyKey: "vendeur-ouverture", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(1), UnitCost: &cost, Currency: "USD"}},
	}); reasonOf(err) != "stock.opening.create" {
		t.Fatalf("seller opening = %v", err)
	}
	keeper := addMember(t, admin, w, "stockkeeper", w.store)
	if _, err := svc.OpenInitialStock(ctx, keeper, commerce.OpenInitialStockInput{
		IdempotencyKey: "magasinier-ouverture", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(12), UnitCost: &cost, Currency: "USD"}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordStockLoss(ctx, seller, commerce.StockLossInput{
		IdempotencyKey: "vendeur-perte", StoreID: w.store, ProductID: soap.ID, ProductUnitID: piece,
		Quantity: decimal.NewFromInt(1), Reason: "other",
	}); reasonOf(err) != "stock.loss.create" {
		t.Fatalf("seller loss = %v", err)
	}
	foreign := shop(t, admin)
	if _, err := svc.OpenInitialStock(ctx, foreign.p, commerce.OpenInitialStockInput{
		IdempotencyKey: "autre-entreprise", StoreID: w.store,
		Lines: []commerce.OpeningLine{{ProductID: soap.ID, ProductUnitID: piece, Quantity: decimal.NewFromInt(1), UnitCost: &cost, Currency: "USD"}},
	}); reasonOf(err) != "not_found" {
		t.Fatalf("cross opening = %v", err)
	}
	if _, err := svc.RecordStockLoss(ctx, foreign.p, commerce.StockLossInput{
		IdempotencyKey: "autre-perte", StoreID: w.store, ProductID: soap.ID, ProductUnitID: piece,
		Quantity: decimal.NewFromInt(1), Reason: "other",
	}); reasonOf(err) != "not_found" {
		t.Fatalf("cross loss = %v", err)
	}
	if got := stockQty(t, admin, w, soap.ID); !got.Equal(decimal.NewFromInt(12)) {
		t.Fatalf("isolated stock %s", got)
	}
	var seen int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM stock_openings WHERE company_id = $1`, foreign.company).Scan(&seen); err != nil || seen != 0 {
		t.Fatalf("foreign openings=%d err=%v", seen, err)
	}
}

func mustProduct(t *testing.T, svc *commerce.Service, w world, in catalog.CreateInput) catalog.Product {
	t.Helper()
	created, err := svc.CreateCatalogProduct(context.Background(), w.p, in)
	if err != nil {
		t.Fatal(err)
	}
	return created
}

func balanceState(t *testing.T, admin *pgxpool.Pool, w world, product uuid.UUID) (string, decimal.Decimal, string, bool) {
	t.Helper()
	var qty string
	var avg *string
	var currency *string
	var complete bool
	if err := admin.QueryRow(context.Background(), `
		SELECT quantity::text, avg_cost::text, cost_currency, cost_complete
		FROM stock_balances WHERE company_id = $1 AND store_id = $2 AND product_id = $3
	`, w.company, w.store, product).Scan(&qty, &avg, &currency, &complete); err != nil {
		t.Fatal(err)
	}
	parsed := decimal.Zero
	if avg != nil {
		parsed = moneyMust(t, *avg)
	}
	code := ""
	if currency != nil {
		code = *currency
	}
	return qty, parsed, code, complete
}

func moneyEqual(t *testing.T, raw, want string) bool {
	t.Helper()
	return moneyMust(t, raw).Equal(moneyMust(t, want))
}

func moneyMust(t *testing.T, raw string) decimal.Decimal {
	t.Helper()
	parsed, err := decimal.NewFromString(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
