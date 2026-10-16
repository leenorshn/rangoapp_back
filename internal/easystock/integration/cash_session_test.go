package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"rangoapp/internal/easystock/api"
	"rangoapp/internal/easystock/commerce"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/readmodel"
	"rangoapp/internal/easystock/reports"
	"rangoapp/internal/easystock/tenant"
	"rangoapp/internal/easystock/testutil"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

func closeSeed(t *testing.T, svc *commerce.Service, w world) {
	t.Helper()
	if err := svc.CloseCashSession(context.Background(), w.p, commerce.CloseCashInput{SessionID: w.session}); err != nil {
		t.Fatal(err)
	}
}

func openDrawer(t *testing.T, svc *commerce.Service, w world, openings ...commerce.OpeningAmount) commerce.OpenedCashSession {
	t.Helper()
	opened, err := svc.OpenCashSession(context.Background(), w.p, commerce.OpenCashInput{
		StoreID: w.store, RegisterID: w.register, Openings: openings,
	})
	if err != nil {
		t.Fatal(err)
	}
	if opened.ID == uuid.Nil || opened.Status != "open" || opened.OpenedBy != w.user || opened.StoreID != w.store {
		t.Fatalf("%+v", opened)
	}
	return opened
}

func cashPay(w world, key, currency, qty, price, paid string, session uuid.UUID) commerce.ConfirmSaleInput {
	in := saleInput(w, key, qty, paid)
	in.Currency = currency
	in.Lines[0].Quantity = money.Must(qty)
	in.Lines[0].ExpectedUnitPrice = money.Must(price)
	in.Payments = []commerce.PaymentInput{{Method: "cash", Amount: money.Must(paid)}}
	in.CashSessionID = &session
	return in
}

func addCashier(t *testing.T, admin *pgxpool.Pool, w world) tenant.Principal {
	t.Helper()
	id := uuid.New()
	ctx := context.Background()
	if _, err := admin.Exec(ctx, `INSERT INTO users (id, display_name) VALUES ($1, 'Caissier')`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO organization_memberships (company_id, user_id, org_role, status) VALUES ($1,$2,'cashier','active')`, w.company, id); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO store_assignments (company_id, store_id, user_id) VALUES ($1,$2,$3)`, w.company, w.store, id); err != nil {
		t.Fatal(err)
	}
	return tenant.Principal{UserID: id, CompanyID: w.company, OrgRole: "cashier", RequestID: "cashier"}
}

func moneyBy(rows []readmodel.Money, currency string) string {
	for _, row := range rows {
		if row.Currency == currency {
			return row.Amount
		}
	}
	return ""
}

func requireMoney(t *testing.T, rows []readmodel.Money, currency, want string) {
	t.Helper()
	got := moneyBy(rows, currency)
	if got == "" || !decimalEqual(got, want) {
		t.Fatalf("%s = %q, want %s in %+v", currency, got, want, rows)
	}
}

func TestOpenCashSessionAcceptsFloatsAndRejectsASecondDrawer(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := seed(t, admin)
	svc := commerce.New(app)
	ctx := context.Background()
	if _, err := svc.OpenCashSession(ctx, w.p, commerce.OpenCashInput{
		StoreID: w.store, RegisterID: w.register,
		Openings: []commerce.OpeningAmount{{Currency: "USD", Amount: money.Must("10")}},
	}); reasonOf(err) != "cash_session_open" {
		t.Fatalf("second open: %v", err)
	}
	closeSeed(t, svc, w)
	if _, err := svc.OpenCashSession(ctx, w.p, commerce.OpenCashInput{
		StoreID: w.store, RegisterID: w.register,
		Openings: []commerce.OpeningAmount{{Currency: "USD", Amount: money.Must("-1")}},
	}); reasonOf(err) != "opening" {
		t.Fatalf("negative: %v", err)
	}
	opened := openDrawer(t, svc, w,
		commerce.OpeningAmount{Currency: "USD", Amount: money.Must("20")},
		commerce.OpeningAmount{Currency: "CDF", Amount: decimal.Zero},
	)
	view, err := readmodel.New(app).CashSession(ctx, w.p, opened.ID)
	if err != nil {
		t.Fatal(err)
	}
	requireMoney(t, view.Opening, "USD", "20")
	requireMoney(t, view.Opening, "CDF", "0")
	requireMoney(t, view.Balances, "USD", "20")
	requireMoney(t, view.Balances, "CDF", "0")
}

func TestConcurrentOpenKeepsOneActiveSession(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := seed(t, admin)
	svc := commerce.New(app)
	closeSeed(t, svc, w)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	ids := make([]uuid.UUID, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			opened, err := svc.OpenCashSession(context.Background(), w.p, commerce.OpenCashInput{
				StoreID: w.store, RegisterID: w.register,
				Openings: []commerce.OpeningAmount{{Currency: "USD", Amount: money.Must("5")}},
			})
			errs[i] = err
			ids[i] = opened.ID
		}(i)
	}
	close(start)
	wg.Wait()
	success, conflict := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			success++
		case reasonOf(err) == "cash_session_open":
			conflict++
		default:
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflict=%d ids=%v errs=%v", success, conflict, ids, errs)
	}
	var openCount int
	if err := admin.QueryRow(context.Background(), `SELECT count(*) FROM cash_sessions WHERE register_id = $1 AND status = 'open'`, w.register).Scan(&openCount); err != nil || openCount != 1 {
		t.Fatalf("open sessions=%d err=%v", openCount, err)
	}
}

func TestCashSaleRequiresAnOpenSessionInTheSameStore(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := seed(t, admin)
	other := seed(t, admin)
	svc := commerce.New(app)
	ctx := context.Background()
	missing := cashPay(w, "no-session", "USD", "1", "10", "10", w.session)
	missing.CashSessionID = nil
	if _, err := svc.ConfirmSale(ctx, w.p, missing); reasonOf(err) != "cash_session" {
		t.Fatalf("missing session: %v", err)
	}
	paid, err := svc.ConfirmSale(ctx, w.p, cashPay(w, "with-session", "USD", "1", "10", "10", w.session))
	if err != nil {
		t.Fatal(err)
	}
	eqNum(t, admin, `SELECT amount::text FROM cash_transactions WHERE ref_id = $1 AND kind = 'sale'`, paid.SaleID, "10")

	store2 := uuid.New()
	if _, err := admin.Exec(ctx, `INSERT INTO stores (id, company_id, name) VALUES ($1,$2,'Annexe')`, store2, w.company); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
		INSERT INTO product_prices (company_id, store_id, product_id, product_unit_id, currency, unit_price)
		VALUES ($1,$2,$3,$4,'USD',10)
	`, w.company, store2, w.product, w.unit); err != nil {
		t.Fatal(err)
	}
	otherStore := cashPay(w, "other-store", "USD", "1", "10", "10", w.session)
	otherStore.StoreID = store2
	if _, err := svc.ConfirmSale(ctx, w.p, otherStore); reasonOf(err) != "cash_session" {
		t.Fatalf("other store: %v", err)
	}
	if _, err := svc.ConfirmSale(ctx, other.p, cashPay(w, "other-company", "USD", "1", "10", "10", w.session)); reasonOf(err) != "not_found" && reasonOf(err) != "cash_session" {
		t.Fatalf("other company sale: %v", err)
	}
	if _, err := readmodel.New(app).CashSession(ctx, other.p, w.session); reasonOf(err) != "not_found" {
		t.Fatalf("other company read: %v", err)
	}
	if err := svc.CloseCashSession(ctx, other.p, commerce.CloseCashInput{SessionID: w.session}); reasonOf(err) != "not_found" {
		t.Fatalf("other company close: %v", err)
	}
}

func TestCashSaleRecordsOneMovementPerCurrencyAndReplay(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := seed(t, admin)
	svc := commerce.New(app)
	ctx := context.Background()
	if _, err := admin.Exec(ctx, `
		INSERT INTO product_prices (company_id, store_id, product_id, product_unit_id, currency, unit_price)
		VALUES ($1,$2,$3,$4,'CDF',1500)
	`, w.company, w.store, w.product, w.unit); err != nil {
		t.Fatal(err)
	}
	usd, err := svc.ConfirmSale(ctx, w.p, cashPay(w, "usd", "USD", "1", "10", "10", w.session))
	if err != nil {
		t.Fatal(err)
	}
	cdf, err := svc.ConfirmSale(ctx, w.p, cashPay(w, "cdf", "CDF", "1", "1500", "1500", w.session))
	if err != nil {
		t.Fatal(err)
	}
	again, err := svc.ConfirmSale(ctx, w.p, cashPay(w, "usd", "USD", "1", "10", "10", w.session))
	if err != nil || again.SaleID != usd.SaleID {
		t.Fatalf("replay %+v %v", again, err)
	}
	var payments, movements int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM payments WHERE sale_id = $1`, usd.SaleID).Scan(&payments); err != nil || payments != 1 {
		t.Fatalf("payments=%d err=%v", payments, err)
	}
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM cash_transactions WHERE ref_id = $1`, usd.SaleID).Scan(&movements); err != nil || movements != 1 {
		t.Fatalf("movements=%d err=%v", movements, err)
	}
	mixed := saleInput(w, "mixed", "1", "10")
	mixed.Payments = []commerce.PaymentInput{
		{Method: "cash", Amount: money.Must("4")},
		{Method: "external", Amount: money.Must("6")},
	}
	mixed.CashSessionID = &w.session
	mix, err := svc.ConfirmSale(ctx, w.p, mixed)
	if err != nil {
		t.Fatal(err)
	}
	eqNum(t, admin, `SELECT amount::text FROM cash_transactions WHERE ref_id = $1 AND kind = 'sale'`, mix.SaleID, "4")
	var external int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM payments WHERE sale_id = $1 AND method = 'external'`, mix.SaleID).Scan(&external); err != nil || external != 1 {
		t.Fatalf("external=%d err=%v", external, err)
	}
	second, err := svc.ConfirmSale(ctx, w.p, cashPay(w, "usd-2", "USD", "1", "10", "10", w.session))
	if err != nil {
		t.Fatal(err)
	}
	view, err := readmodel.New(app).CashSession(ctx, w.p, w.session)
	if err != nil {
		t.Fatal(err)
	}
	requireMoney(t, view.TotalIn, "USD", "24")
	requireMoney(t, view.TotalIn, "CDF", "1500")
	if usd.SaleID == cdf.SaleID || second.SaleID == usd.SaleID {
		t.Fatal("sales collapsed")
	}
}

func TestCashBusinessDayComputesVarianceWithoutAdjusting(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := seed(t, admin)
	svc := commerce.New(app)
	ctx := context.Background()
	closeSeed(t, svc, w)
	opened := openDrawer(t, svc, w,
		commerce.OpeningAmount{Currency: "USD", Amount: money.Must("20")},
		commerce.OpeningAmount{Currency: "CDF", Amount: money.Must("50000")},
	)
	if _, err := admin.Exec(ctx, `
		INSERT INTO product_prices (company_id, store_id, product_id, product_unit_id, currency, unit_price)
		VALUES ($1,$2,$3,$4,'CDF',150000)
	`, w.company, w.store, w.product, w.unit); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ConfirmSale(ctx, w.p, cashPay(w, "day-usd", "USD", "8", "10", "80", opened.ID)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ConfirmSale(ctx, w.p, cashPay(w, "day-cdf", "CDF", "1", "150000", "150000", opened.ID)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateExpense(ctx, w.p, commerce.ExpenseInput{
		StoreID: w.store, Category: "sortie", Amount: money.Must("10"), Currency: "USD",
		PaidFromCash: true, CashSessionID: &opened.ID, Note: "sortie de caisse",
	}); err != nil {
		t.Fatal(err)
	}
	reads := readmodel.New(app)
	before, err := reads.CashSession(ctx, w.p, opened.ID)
	if err != nil {
		t.Fatal(err)
	}
	requireMoney(t, before.Opening, "USD", "20")
	requireMoney(t, before.Opening, "CDF", "50000")
	requireMoney(t, before.TotalIn, "USD", "80")
	requireMoney(t, before.TotalIn, "CDF", "150000")
	requireMoney(t, before.TotalOut, "USD", "10")
	requireMoney(t, before.Balances, "USD", "90")
	requireMoney(t, before.Balances, "CDF", "200000")

	if err := svc.CloseCashSession(ctx, w.p, commerce.CloseCashInput{
		SessionID: opened.ID,
		Note:      "comptage du soir",
		Counts: []commerce.CloseCount{
			{Currency: "USD", Counted: money.Must("88")},
			{Currency: "CDF", Counted: money.Must("205000")},
		},
	}); err != nil {
		t.Fatal(err)
	}
	after, err := reads.CashSession(ctx, w.p, opened.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != "closed" || after.ClosedBy == nil || *after.ClosedBy != w.user || after.ClosedAt == nil {
		t.Fatalf("%+v", after)
	}
	if after.Note != "comptage du soir" {
		t.Fatalf("note %q", after.Note)
	}
	requireMoney(t, after.Counted, "USD", "88")
	requireMoney(t, after.Counted, "CDF", "205000")
	requireMoney(t, after.Variances, "USD", "-2")
	requireMoney(t, after.Variances, "CDF", "5000")
	requireMoney(t, after.Balances, "USD", "90")
	requireMoney(t, after.Balances, "CDF", "200000")
	var adjustments int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM cash_transactions WHERE session_id = $1 AND kind = 'adjustment'`, opened.ID).Scan(&adjustments); err != nil || adjustments != 0 {
		t.Fatalf("adjustments=%d err=%v", adjustments, err)
	}
	if len(after.Transactions) != 5 {
		t.Fatalf("history %d", len(after.Transactions))
	}
	page, err := reads.CashTransactions(ctx, w.p, opened.ID, readmodel.Page{Limit: 10})
	if err != nil || len(page) != 5 {
		t.Fatalf("journal %d %v", len(page), err)
	}
	if _, err := svc.ConfirmSale(ctx, w.p, cashPay(w, "after-close", "USD", "1", "10", "10", opened.ID)); reasonOf(err) != "cash_session" {
		t.Fatalf("sale after close: %v", err)
	}
	if err := svc.CloseCashSession(ctx, w.p, commerce.CloseCashInput{
		SessionID: opened.ID,
		Counts: []commerce.CloseCount{
			{Currency: "USD", Counted: money.Must("88")},
			{Currency: "CDF", Counted: money.Must("205000")},
		},
	}); reasonOf(err) != "cash_session_closed" {
		t.Fatalf("second close: %v", err)
	}
	still, err := reads.CashSession(ctx, w.p, opened.ID)
	if err != nil || len(still.Transactions) != 5 {
		t.Fatalf("history changed %d %v", len(still.Transactions), err)
	}
	from := time.Now().Add(-time.Hour)
	to := time.Now().Add(time.Hour)
	usdCounts, err := reports.New(app).CashCounts(ctx, w.p, reports.Filter{StoreIDs: []uuid.UUID{w.store}, Currency: "USD", From: from, To: to})
	if err != nil || len(usdCounts) != 1 || !decimalEqual(usdCounts[0].Variance, "-2") {
		t.Fatalf("usd report %+v %v", usdCounts, err)
	}
	cdfCounts, err := reports.New(app).CashCounts(ctx, w.p, reports.Filter{StoreIDs: []uuid.UUID{w.store}, Currency: "CDF", From: from, To: to})
	if err != nil || len(cdfCounts) != 1 || !decimalEqual(cdfCounts[0].Variance, "5000") {
		t.Fatalf("cdf report %+v %v", cdfCounts, err)
	}
}

func TestCashCloseVarianceSign(t *testing.T) {
	cases := []struct {
		name, opening, counted, variance string
	}{
		{name: "zero", opening: "10", counted: "10", variance: "0"},
		{name: "positive", opening: "10", counted: "12", variance: "2"},
		{name: "negative", opening: "10", counted: "8", variance: "-2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			admin, app, _, _ := testutil.Pools(t)
			w := seed(t, admin)
			svc := commerce.New(app)
			closeSeed(t, svc, w)
			opened := openDrawer(t, svc, w, commerce.OpeningAmount{Currency: "USD", Amount: money.Must(tc.opening)})
			if err := svc.CloseCashSession(context.Background(), w.p, commerce.CloseCashInput{
				SessionID: opened.ID,
				Counts:    []commerce.CloseCount{{Currency: "USD", Counted: money.Must(tc.counted)}},
			}); err != nil {
				t.Fatal(err)
			}
			view, err := readmodel.New(app).CashSession(context.Background(), w.p, opened.ID)
			if err != nil {
				t.Fatal(err)
			}
			requireMoney(t, view.Variances, "USD", tc.variance)
			requireMoney(t, view.Balances, "USD", tc.opening)
		})
	}
}

func TestCashierCannotCloseAnotherDrawer(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := seed(t, admin)
	svc := commerce.New(app)
	ctx := context.Background()
	cashier := addCashier(t, admin, w)
	if err := svc.CloseCashSession(ctx, cashier, commerce.CloseCashInput{
		SessionID: w.session,
		Counts:    []commerce.CloseCount{{Currency: "USD", Counted: decimal.Zero}},
	}); reasonOf(err) != "cash_session" {
		t.Fatalf("cashier close: %v", err)
	}
	hidden, err := readmodel.New(app).CashTransactions(ctx, cashier, w.session, readmodel.Page{})
	if reasonOf(err) != "cash_session" || hidden != nil {
		t.Fatalf("hidden journal %v %v", hidden, err)
	}
	closeSeed(t, svc, w)
	own, err := svc.OpenCashSession(ctx, cashier, commerce.OpenCashInput{
		StoreID: w.store, RegisterID: w.register,
		Openings: []commerce.OpeningAmount{{Currency: "USD", Amount: money.Must("3")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseCashSession(ctx, w.p, commerce.CloseCashInput{
		SessionID: own.ID,
		Counts:    []commerce.CloseCount{{Currency: "USD", Counted: money.Must("3")}},
	}); err != nil {
		t.Fatalf("owner close: %v", err)
	}
}

func TestCloseAndSaleStayConsistent(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := seed(t, admin)
	svc := commerce.New(app)
	ctx := context.Background()
	closeSeed(t, svc, w)
	opened := openDrawer(t, svc, w, commerce.OpeningAmount{Currency: "USD", Amount: money.Must("20")})
	var wg sync.WaitGroup
	var saleErr, closeErr error
	start := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, saleErr = svc.ConfirmSale(context.Background(), w.p, cashPay(w, "race-sale", "USD", "1", "10", "10", opened.ID))
	}()
	go func() {
		defer wg.Done()
		<-start
		closeErr = svc.CloseCashSession(context.Background(), w.p, commerce.CloseCashInput{
			SessionID: opened.ID,
			Counts:    []commerce.CloseCount{{Currency: "USD", Counted: money.Must("20")}},
		})
	}()
	close(start)
	wg.Wait()
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if saleErr != nil && reasonOf(saleErr) != "cash_session" {
		t.Fatal(saleErr)
	}
	var status string
	var expected, movement string
	var sales int
	if err := admin.QueryRow(ctx, `SELECT status FROM cash_sessions WHERE id = $1`, opened.ID).Scan(&status); err != nil || status != "closed" {
		t.Fatalf("status %s %v", status, err)
	}
	if err := admin.QueryRow(ctx, `SELECT expected_amount::text FROM cash_session_counts WHERE session_id = $1 AND currency = 'USD'`, opened.ID).Scan(&expected); err != nil {
		t.Fatal(err)
	}
	if err := admin.QueryRow(ctx, `
		SELECT COALESCE(SUM(CASE WHEN direction = 'in' THEN amount ELSE -amount END), 0)::text
		FROM cash_transactions WHERE session_id = $1 AND currency = 'USD'
	`, opened.ID).Scan(&movement); err != nil {
		t.Fatal(err)
	}
	if !decimalEqual(expected, movement) {
		t.Fatalf("expected %s movement %s", expected, movement)
	}
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM sales WHERE company_id = $1 AND id IN (SELECT ref_id FROM cash_transactions WHERE session_id = $2 AND kind = 'sale')`, w.company, opened.ID).Scan(&sales); err != nil {
		t.Fatal(err)
	}
	if saleErr == nil && (!decimalEqual(expected, "30") || sales != 1) {
		t.Fatalf("sale committed outside the count: expected %s sales %d", expected, sales)
	}
	if saleErr != nil && (!decimalEqual(expected, "20") || sales != 0) {
		t.Fatalf("rejected sale still moved the drawer: expected %s sales %d err %v", expected, sales, saleErr)
	}
}

func TestGraphQLCashSessionRoundTrip(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := seed(t, admin)
	svc := commerce.New(app)
	closeSeed(t, svc, w)
	schema, err := api.Schema(api.Deps{
		Commerce: svc, Reads: readmodel.New(app),
		ContextSecret: "test-secret-test-secret-test-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := api.WithPrincipal(context.Background(), w.p)
	open := api.Execute(ctx, schema, `mutation ($input: OpenCashSessionInput!) {
		openCashSession(input: $input) {
			id status storeId cashRegisterId openedBy
			openingBalances { currency amount }
			theoreticalBalances { currency amount }
		}
	}`, map[string]any{"input": map[string]any{
		"storeId": w.store.String(), "cashRegisterId": w.register.String(),
		"openingBalances": []any{
			map[string]any{"currency": "USD", "amount": "20"},
			map[string]any{"currency": "CDF", "amount": "0"},
		},
	}})
	if len(open.Errors) > 0 {
		t.Fatal(open.Errors)
	}
	body := open.Data.(map[string]any)["openCashSession"].(map[string]any)
	if body["status"] != "open" || body["openedBy"] != w.user.String() || body["storeId"] != w.store.String() {
		t.Fatalf("%v", body)
	}
	sessionID := body["id"].(string)
	listed := api.Execute(ctx, schema, `query ($store: ID!, $id: ID!) {
		cash(storeId: $store) { name openSession { id theoreticalBalances { currency amount } } }
		cashSession(id: $id) { status openingBalances { currency amount } }
		cashSessions(storeId: $store, status: "open") { id }
		cashTransactions(sessionId: $id) { kind currency amount }
	}`, map[string]any{"store": w.store.String(), "id": sessionID})
	if len(listed.Errors) > 0 {
		t.Fatal(listed.Errors)
	}
	data := listed.Data.(map[string]any)
	registers := data["cash"].([]any)
	if registers[0].(map[string]any)["openSession"].(map[string]any)["id"] != sessionID {
		t.Fatalf("%v", registers)
	}
	txs := data["cashTransactions"].([]any)
	if len(txs) != 2 {
		t.Fatalf("%v", txs)
	}
	closed := api.Execute(ctx, schema, `mutation ($input: CloseCashSessionInput!) {
		closeCashSession(input: $input) {
			status variances { currency amount } note
		}
	}`, map[string]any{"input": map[string]any{
		"cashSessionId": sessionID,
		"note":          "soir",
		"countedBalances": []any{
			map[string]any{"currency": "USD", "amount": "20"},
			map[string]any{"currency": "CDF", "amount": "0"},
		},
	}})
	if len(closed.Errors) > 0 {
		t.Fatal(closed.Errors)
	}
	closedBody := closed.Data.(map[string]any)["closeCashSession"].(map[string]any)
	if closedBody["status"] != "closed" || closedBody["note"] != "soir" {
		t.Fatalf("%v", closedBody)
	}
	again := api.Execute(ctx, schema, `mutation ($input: CloseCashSessionInput!) {
		closeCashSession(input: $input) { id }
	}`, map[string]any{"input": map[string]any{
		"cashSessionId": sessionID,
		"countedBalances": []any{
			map[string]any{"currency": "USD", "amount": "20"},
			map[string]any{"currency": "CDF", "amount": "0"},
		},
	}})
	if len(again.Errors) == 0 {
		t.Fatal("closed twice")
	}
}
