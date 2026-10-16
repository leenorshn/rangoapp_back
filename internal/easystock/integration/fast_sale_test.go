package integration

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"rangoapp/internal/easystock/api"
	"rangoapp/internal/easystock/commerce"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/readmodel"
	"rangoapp/internal/easystock/testutil"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestFastSaleCashConfirmationAndPackaging(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := seed(t, admin)
	svc := commerce.New(app)
	ctx := context.Background()

	if _, err := svc.ConfirmSale(ctx, w.p, saleInput(w, "cash-one", "1", "10")); err != nil {
		t.Fatal(err)
	}
	paid := saleInput(w, "cash-one-drawer", "1", "10")
	paid.Payments = []commerce.PaymentInput{{Method: "cash", Amount: money.Must("10")}}
	paid.CashSessionID = &w.session
	paid.IdempotencyKey = "cash-one-drawer"
	confirmed, err := svc.ConfirmSale(ctx, w.p, paid)
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.Reference != confirmed.SaleID.String() || confirmed.Net != "10" || confirmed.Paid != "10" || confirmed.Due != "0" || confirmed.DebtID != nil {
		t.Fatalf("confirmation %+v", confirmed)
	}
	if confirmed.ItemCount != 1 || confirmed.CreatedAt.IsZero() || confirmed.Currency != "USD" {
		t.Fatalf("confirmation details %+v", confirmed)
	}
	if got := stockQty(t, admin, w, w.product); !got.Equal(money.Must("18")) {
		t.Fatalf("stock after two sales = %s", got)
	}
	eqNum(t, admin, `SELECT COALESCE(SUM(amount), 0)::text FROM cash_transactions WHERE ref_id = $1 AND kind = 'sale'`, confirmed.SaleID, "10")

	several := saleInput(w, "cash-several", "2", "20")
	several.Payments = []commerce.PaymentInput{{Method: "cash", Amount: money.Must("20")}}
	several.CashSessionID = &w.session
	if _, err := svc.ConfirmSale(ctx, w.p, several); err != nil {
		t.Fatal(err)
	}

	water := uuid.New()
	bottle := uuid.New()
	carton := uuid.New()
	exec(t, admin, `INSERT INTO products (id, company_id, sku, name, base_unit) VALUES ($1,$2,'EAU','Eau minérale','bouteille')`, water, w.company)
	exec(t, admin, `INSERT INTO product_units (id, company_id, product_id, name, factor, is_base) VALUES ($1,$2,$3,'bouteille',1,true)`, bottle, w.company, water)
	exec(t, admin, `INSERT INTO product_units (id, company_id, product_id, name, factor, is_base) VALUES ($1,$2,$3,'carton',24,false)`, carton, w.company, water)
	exec(t, admin, `INSERT INTO product_prices (company_id, store_id, product_id, product_unit_id, currency, unit_price) VALUES ($1,$2,$3,$4,'CDF',500), ($1,$2,$3,$5,'CDF',10000)`, w.company, w.store, water, bottle, carton)
	exec(t, admin, `INSERT INTO stock_balances (company_id, store_id, product_id, quantity) VALUES ($1,$2,$3,100)`, w.company, w.store, water)
	packed, err := svc.ConfirmSale(ctx, w.p, commerce.ConfirmSaleInput{
		IdempotencyKey: "water-cash", StoreID: w.store, Currency: "CDF", CashSessionID: &w.session,
		Lines: []commerce.SaleLineInput{
			{ProductID: water, UnitName: "carton", Quantity: money.Must("2"), ExpectedUnitPrice: money.Must("10000")},
			{ProductID: water, UnitName: "bouteille", Quantity: money.Must("3"), ExpectedUnitPrice: money.Must("500")},
		},
		Payments: []commerce.PaymentInput{{Method: "cash", Amount: money.Must("21500")}},
	})
	if err != nil || packed.Net != "21500" || packed.Paid != "21500" || packed.ItemCount != 2 || packed.Currency != "CDF" {
		t.Fatalf("%+v %v", packed, err)
	}
	if got := stockQty(t, admin, w, water); !got.Equal(money.Must("49")) {
		t.Fatalf("bottles left = %s", got)
	}
	eqNum(t, admin, `SELECT amount::text FROM cash_transactions WHERE ref_id = $1 AND kind = 'sale' AND currency = 'CDF'`, packed.SaleID, "21500")
}

func TestFastSaleDebtRules(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := seed(t, admin)
	svc := commerce.New(app)
	ctx := context.Background()
	loc, err := time.LoadLocation("Africa/Lubumbashi")
	if err != nil {
		t.Fatal(err)
	}
	today := time.Now().In(loc).Format("2006-01-02")

	zero := creditSale(w, "debt-zero", "1", nil)
	sold, err := svc.ConfirmSale(ctx, w.p, zero)
	if err != nil {
		t.Fatal(err)
	}
	if sold.Net != "10" || sold.Paid != "0" || sold.Due != "10" || sold.DebtID == nil || sold.DebtDueDate != "2099-01-01" || sold.CustomerName != "Client" {
		t.Fatalf("%+v", sold)
	}
	var dueOn, status, original string
	if err := admin.QueryRow(ctx, `SELECT due_on::text, status, original_amount::text FROM debts WHERE id = $1`, *sold.DebtID).Scan(&dueOn, &status, &original); err != nil {
		t.Fatal(err)
	}
	if dueOn != "2099-01-01" || status != "open" || !money.Must(original).Equal(money.Must("10")) {
		t.Fatalf("debt %s %s %s", dueOn, status, original)
	}
	var cashMoves int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM cash_transactions WHERE ref_id = $1 AND kind = 'sale'`, sold.SaleID).Scan(&cashMoves); err != nil || cashMoves != 0 {
		t.Fatalf("cash moves = %d err=%v", cashMoves, err)
	}
	if got := stockQty(t, admin, w, w.product); !got.Equal(money.Must("19")) {
		t.Fatalf("stock with zero advance = %s", got)
	}

	partial := creditSale(w, "debt-partial", "2", []commerce.PaymentInput{{Method: "cash", Amount: money.Must("5")}})
	partial.CashSessionID = &w.session
	part, err := svc.ConfirmSale(ctx, w.p, partial)
	if err != nil || part.Net != "20" || part.Paid != "5" || part.Due != "15" || part.DebtID == nil {
		t.Fatalf("%+v %v", part, err)
	}
	eqNum(t, admin, `SELECT amount::text FROM cash_transactions WHERE ref_id = $1 AND kind = 'sale'`, part.SaleID, "5")
	var partialStatus string
	if err := admin.QueryRow(ctx, `SELECT status FROM debts WHERE id = $1`, *part.DebtID).Scan(&partialStatus); err != nil || partialStatus != "partial" {
		t.Fatalf("status %s %v", partialStatus, err)
	}

	external := creditSale(w, "debt-external", "1", []commerce.PaymentInput{{Method: "external", Amount: money.Must("4")}})
	ext, err := svc.ConfirmSale(ctx, w.p, external)
	if err != nil || ext.Due != "6" {
		t.Fatalf("%+v %v", ext, err)
	}
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM cash_transactions WHERE ref_id = $1 AND kind = 'sale'`, ext.SaleID).Scan(&cashMoves); err != nil || cashMoves != 0 {
		t.Fatalf("external advance moved cash: %d %v", cashMoves, err)
	}

	full := creditSale(w, "debt-full", "1", []commerce.PaymentInput{{Method: "external", Amount: money.Must("10")}})
	full.DebtDueDate = "2099-01-01"
	done, err := svc.ConfirmSale(ctx, w.p, full)
	if err != nil || done.Due != "0" || done.DebtID != nil || done.DebtDueDate != "" {
		t.Fatalf("full advance %+v %v", done, err)
	}

	negative := creditSale(w, "debt-negative", "1", []commerce.PaymentInput{{Method: "cash", Amount: money.Must("-1")}})
	negative.CashSessionID = &w.session
	if _, err := svc.ConfirmSale(ctx, w.p, negative); reasonOf(err) != "payment" {
		t.Fatalf("negative = %v", err)
	}
	over := creditSale(w, "debt-over", "1", []commerce.PaymentInput{{Method: "external", Amount: money.Must("11")}})
	if _, err := svc.ConfirmSale(ctx, w.p, over); reasonOf(err) != "payment" {
		t.Fatalf("over = %v", err)
	}
	noCustomer := creditSale(w, "debt-nocustomer", "1", nil)
	noCustomer.CustomerID = nil
	if _, err := svc.ConfirmSale(ctx, w.p, noCustomer); reasonOf(err) != "customer" {
		t.Fatalf("customer = %v", err)
	}
	noDate := creditSale(w, "debt-nodate", "1", nil)
	noDate.DebtDueDate = ""
	if _, err := svc.ConfirmSale(ctx, w.p, noDate); reasonOf(err) != "debt_due_date" {
		t.Fatalf("due date = %v", err)
	}
	past := creditSale(w, "debt-past", "1", nil)
	past.DebtDueDate = "2000-01-01"
	if _, err := svc.ConfirmSale(ctx, w.p, past); reasonOf(err) != "debt_due_date" {
		t.Fatalf("past = %v", err)
	}
	sameDay := creditSale(w, "debt-today", "1", nil)
	sameDay.DebtDueDate = today
	todaySale, err := svc.ConfirmSale(ctx, w.p, sameDay)
	if err != nil || todaySale.DebtDueDate != today {
		t.Fatalf("today %+v %v", todaySale, err)
	}

	exec(t, admin, `INSERT INTO product_prices (company_id, store_id, product_id, product_unit_id, currency, unit_price) VALUES ($1,$2,$3,$4,'CDF',1500)`, w.company, w.store, w.product, w.unit)
	cdf := creditSale(w, "debt-cdf", "2", []commerce.PaymentInput{{Method: "external", Amount: money.Must("1000")}})
	cdf.Currency = "CDF"
	cdf.Lines[0].ExpectedUnitPrice = money.Must("1500")
	cdfSale, err := svc.ConfirmSale(ctx, w.p, cdf)
	if err != nil || cdfSale.Net != "3000" || cdfSale.Paid != "1000" || cdfSale.Due != "2000" || cdfSale.Currency != "CDF" {
		t.Fatalf("cdf %+v %v", cdfSale, err)
	}

	rows, err := readmodel.New(app).Debts(ctx, w.p, w.store, "", readmodel.Page{})
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, row := range rows {
		if sold.DebtID != nil && row.ID == *sold.DebtID {
			found = true
			if row.AdvanceAmount != "0" || row.DueAmount != "10" || row.DueOn != "2099-01-01" || row.Overdue {
				t.Fatalf("listed %+v", row)
			}
		}
	}
	if !found {
		t.Fatal("debt was not listed")
	}
}

func TestFastSaleCustomersSecurityAndSearch(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := seed(t, admin)
	other := seed(t, admin)
	svc := commerce.New(app)
	ctx := context.Background()
	seller := addMember(t, admin, w, "seller", w.store)
	keeper := addMember(t, admin, w, "stockkeeper", w.store)

	created, err := svc.CreateCustomer(ctx, seller, commerce.CustomerInput{
		StoreID: w.store, Name: "Amina Kabila", Address: "Avenue de la Paix 12",
		Phone: "0970000001", WhatsappPhone: "0970000001",
	})
	if err != nil || created.Phone != "+243970000001" || created.Address != "Avenue de la Paix 12" {
		t.Fatalf("%+v %v", created, err)
	}
	if _, err := svc.CreateCustomer(ctx, seller, commerce.CustomerInput{
		StoreID: w.store, Name: "Doublon", Address: "Même rue", Phone: "+243970000001",
	}); reasonOf(err) != "already_exists" {
		t.Fatalf("duplicate = %v", err)
	}
	if _, err := svc.CreateCustomer(ctx, other.p, commerce.CustomerInput{
		StoreID: other.store, Name: "Autre société", Address: "Autre ville", Phone: "0970000001",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateCustomer(ctx, seller, commerce.CustomerInput{
		StoreID: w.store, Name: "Sans contact", Address: "Rue",
	}); reasonOf(err) != "phone" {
		t.Fatalf("phone = %v", err)
	}

	usingNew := creditSale(w, "debt-new-customer", "1", nil)
	usingNew.CustomerID = &created.ID
	if _, err := svc.ConfirmSale(ctx, seller, usingNew); err != nil {
		t.Fatal(err)
	}
	usingExisting := creditSale(w, "debt-existing", "1", nil)
	if _, err := svc.ConfirmSale(ctx, seller, usingExisting); err != nil {
		t.Fatal(err)
	}
	foreign := creditSale(w, "debt-foreign", "1", nil)
	foreign.CustomerID = &other.customer
	if _, err := svc.ConfirmSale(ctx, w.p, foreign); reasonOf(err) != "not_found" {
		t.Fatalf("foreign customer = %v", err)
	}

	register := uuid.New()
	session := uuid.New()
	store2 := uuid.New()
	exec(t, admin, `INSERT INTO stores (id, company_id, name) VALUES ($1,$2,'Annexe')`, store2, w.company)
	exec(t, admin, `INSERT INTO cash_registers (id, company_id, store_id, name) VALUES ($1,$2,$3,'Autre')`, register, w.company, store2)
	exec(t, admin, `INSERT INTO cash_sessions (id, company_id, store_id, register_id, opened_by, status) VALUES ($1,$2,$3,$4,$5,'open')`, session, w.company, store2, register, w.user)
	otherDrawer := saleInput(w, "other-drawer", "1", "10")
	otherDrawer.Payments = []commerce.PaymentInput{{Method: "cash", Amount: money.Must("10")}}
	otherDrawer.CashSessionID = &session
	if _, err := svc.ConfirmSale(ctx, w.p, otherDrawer); reasonOf(err) != "cash_session" {
		t.Fatalf("other drawer = %v", err)
	}

	if _, err := svc.ConfirmSale(ctx, keeper, saleInput(w, "keeper", "1", "10")); reasonOf(err) != "sales.create" {
		t.Fatalf("stockkeeper = %v", err)
	}
	exec(t, admin, `INSERT INTO permission_overrides (company_id, user_id, permission, effect) VALUES ($1,$2,'credit.authorize','deny')`, w.company, seller.UserID)
	if _, err := svc.ConfirmSale(ctx, seller, creditSale(w, "seller-denied", "1", nil)); reasonOf(err) != "credit.authorize" {
		t.Fatalf("deny = %v", err)
	}
	if _, err := svc.ConfirmSale(ctx, seller, saleInput(w, "seller-cash", "1", "10")); err != nil {
		t.Fatalf("seller cash = %v", err)
	}
	exec(t, admin, `DELETE FROM permission_overrides WHERE user_id = $1`, seller.UserID)
	exec(t, admin, `UPDATE companies SET default_max_credit = 0 WHERE id = $1`, w.company)
	if _, err := svc.ConfirmSale(ctx, seller, creditSale(w, "seller-ceiling", "1", nil)); reasonOf(err) != "credit_ceiling" {
		t.Fatalf("ceiling = %v", err)
	}
	if _, err := svc.ConfirmSale(ctx, w.p, creditSale(w, "owner-still", "1", nil)); err != nil {
		t.Fatalf("owner ceiling = %v", err)
	}

	exec(t, admin, `UPDATE subscription_snapshots SET status = 'expired', trial_ends_at = now() - interval '1 day' WHERE company_id = $1`, other.company)
	if _, err := svc.ConfirmSale(ctx, other.p, saleInput(other, "blocked", "1", "10")); reasonOf(err) != "SUBSCRIPTION_EXPIRED" {
		t.Fatalf("subscription = %v", err)
	}

	schema, err := api.Schema(api.Deps{Commerce: svc, Reads: readmodel.New(app)})
	if err != nil {
		t.Fatal(err)
	}
	member := api.WithPrincipal(ctx, seller)
	createdGQL := api.Execute(member, schema, `mutation ($input: CreateCustomerInput!) {
		createCustomer(input: $input) { id name phone address }
	}`, map[string]any{"input": map[string]any{
		"storeId": w.store.String(), "name": "Jean Mutombo", "address": "Quartier 5", "phone": "0970000002",
	}})
	if len(createdGQL.Errors) > 0 {
		t.Fatal(createdGQL.Errors)
	}
	found := api.Execute(member, schema, `query ($store: ID!, $search: String!) {
		customers(storeId: $store, search: $search) { name phone openDebts { currency amount } }
	}`, map[string]any{"store": w.store.String(), "search": "Mutombo"})
	if len(found.Errors) > 0 {
		t.Fatal(found.Errors)
	}
	customers := found.Data.(map[string]any)["customers"].([]any)
	if len(customers) != 1 || customers[0].(map[string]any)["phone"] != "+243970000002" {
		t.Fatalf("%v", customers)
	}
	if customers[0].(map[string]any)["openDebts"] != nil {
		t.Fatal("seller saw debt balances")
	}
	ownerView := api.Execute(api.WithPrincipal(ctx, w.p), schema, `query ($store: ID!, $search: String!) {
		customers(storeId: $store, search: $search) { name openDebts { currency amount } }
	}`, map[string]any{"store": w.store.String(), "search": "+243970000001"})
	if len(ownerView.Errors) > 0 {
		t.Fatal(ownerView.Errors)
	}
	ownerRows := ownerView.Data.(map[string]any)["customers"].([]any)
	if len(ownerRows) != 1 {
		t.Fatalf("%v", ownerRows)
	}
	debts, _ := ownerRows[0].(map[string]any)["openDebts"].([]any)
	if len(debts) != 1 || debts[0].(map[string]any)["currency"] != "USD" {
		t.Fatalf("%v", ownerRows[0])
	}

	exec(t, admin, `UPDATE product_units SET allow_sale = false WHERE id = $1`, w.unit)
	hidden := api.Execute(member, schema, `query ($store: ID!) {
		products(storeId: $store, forSale: true) { id name availableQuantity availability }
	}`, map[string]any{"store": w.store.String()})
	if len(hidden.Errors) > 0 {
		t.Fatal(hidden.Errors)
	}
	for _, row := range hidden.Data.(map[string]any)["products"].([]any) {
		if row.(map[string]any)["id"] == w.product.String() {
			t.Fatal("non-sellable product was offered")
		}
	}
}

func TestFastSaleIdempotencyConcurrencyAndRollback(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := seed(t, admin)
	svc := commerce.New(app)
	ctx := context.Background()
	in := creditSale(w, "replay-debt", "1", nil)
	first, err := svc.ConfirmSale(ctx, w.p, in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.ConfirmSale(ctx, w.p, in)
	if err != nil || second.SaleID != first.SaleID || second.Due != first.Due {
		t.Fatalf("replay %+v %v", second, err)
	}
	var sales int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM sales WHERE company_id = $1`, w.company).Scan(&sales); err != nil || sales != 1 {
		t.Fatalf("sales=%d err=%v", sales, err)
	}
	changed := in
	changed.DebtDueDate = "2099-02-01"
	if _, err := svc.ConfirmSale(ctx, w.p, changed); reasonOf(err) != "idempotency_mismatch" {
		t.Fatalf("mismatch = %v", err)
	}

	race := seed(t, admin)
	if _, err := admin.Exec(ctx, `UPDATE stock_balances SET quantity = 1 WHERE company_id = $1 AND product_id = $2`, race.company, race.product); err != nil {
		t.Fatal(err)
	}
	raceSvc := commerce.New(app)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = raceSvc.ConfirmSale(ctx, race.p, creditSale(race, uuid.NewString(), "1", nil))
		}(i)
	}
	wg.Wait()
	success := 0
	for _, err := range errs {
		if err == nil {
			success++
			continue
		}
		if reasonOf(err) != "stock_conflict" {
			t.Fatalf("race = %v", err)
		}
	}
	if success != 1 {
		t.Fatalf("successes = %d", success)
	}
	var debts int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM debts WHERE company_id = $1`, race.company).Scan(&debts); err != nil || debts != 1 {
		t.Fatalf("debts=%d err=%v", debts, err)
	}

	rolled := seed(t, admin)
	rollbackSvc := commerce.New(app)
	rollbackSvc.SetAfterWrite(func() error { return errors.New("boom") })
	before := stockQty(t, admin, rolled, rolled.product)
	if _, err := rollbackSvc.ConfirmSale(ctx, rolled.p, creditSale(rolled, "rollback-debt", "1", []commerce.PaymentInput{{Method: "cash", Amount: money.Must("3")}})); err == nil {
		t.Fatal("expected rollback")
	}
	var left int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM sales WHERE company_id = $1`, rolled.company).Scan(&left); err != nil || left != 0 {
		t.Fatalf("sales left = %d %v", left, err)
	}
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM debts WHERE company_id = $1`, rolled.company).Scan(&left); err != nil || left != 0 {
		t.Fatalf("debts left = %d %v", left, err)
	}
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM cash_transactions WHERE company_id = $1 AND kind = 'sale'`, rolled.company).Scan(&left); err != nil || left != 0 {
		t.Fatalf("cash left = %d %v", left, err)
	}
	if got := stockQty(t, admin, rolled, rolled.product); !got.Equal(before) {
		t.Fatalf("stock %s want %s", got, before)
	}
}

func exec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("sql: %v", err)
	}
}

func creditSale(w world, key, qty string, payments []commerce.PaymentInput) commerce.ConfirmSaleInput {
	return commerce.ConfirmSaleInput{
		IdempotencyKey: key,
		StoreID:        w.store,
		CustomerID:     &w.customer,
		Currency:       "USD",
		DebtDueDate:    "2099-01-01",
		Lines: []commerce.SaleLineInput{{
			ProductID: w.product, UnitName: "piece",
			Quantity: money.Must(qty), ExpectedUnitPrice: money.Must("10"),
		}},
		Payments: payments,
	}
}
