package integration

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"rangoapp/internal/easystock/commerce"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/identity"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/reports"
	"rangoapp/internal/easystock/subscription"
	"rangoapp/internal/easystock/tenant"
	"rangoapp/internal/easystock/testutil"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

type world struct {
	company, user, store, product, unit, customer, register, session uuid.UUID
	p                                                                tenant.Principal
}

func seed(t *testing.T, admin *pgxpool.Pool) world {
	t.Helper()
	ctx := context.Background()
	w := world{
		company: uuid.New(), user: uuid.New(), store: uuid.New(), product: uuid.New(),
		unit: uuid.New(), customer: uuid.New(), register: uuid.New(), session: uuid.New(),
	}
	w.p = tenant.Principal{UserID: w.user, CompanyID: w.company, OrgRole: "owner", RequestID: "test"}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := admin.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	exec(`INSERT INTO companies (id, name, primary_currency) VALUES ($1, 'Boutique', 'USD')`, w.company)
	exec(`INSERT INTO users (id, display_name) VALUES ($1, 'Owner')`, w.user)
	exec(`INSERT INTO organization_memberships (company_id, user_id, org_role, status) VALUES ($1,$2,'owner','active')`, w.company, w.user)
	exec(`INSERT INTO stores (id, company_id, name) VALUES ($1,$2,'Centre')`, w.store, w.company)
	exec(`INSERT INTO store_assignments (company_id, store_id, user_id) VALUES ($1,$2,$3)`, w.company, w.store, w.user)
	exec(`INSERT INTO subscription_snapshots (company_id, plan_code, status, source, trial_ends_at) VALUES ($1,'essentiel','trialing','trial',$2)`, w.company, time.Now().Add(14*24*time.Hour))
	exec(`INSERT INTO products (id, company_id, sku, name, base_unit, alert_threshold) VALUES ($1,$2,$3,'Huile','piece',2)`, w.product, w.company, "SKU-"+w.product.String()[:8])
	exec(`INSERT INTO product_units (id, company_id, product_id, name, factor, is_base) VALUES ($1,$2,$3,'piece',1,true)`, w.unit, w.company, w.product)
	exec(`INSERT INTO product_prices (company_id, store_id, product_id, product_unit_id, currency, unit_price) VALUES ($1,$2,$3,$4,'USD',10)`, w.company, w.store, w.product, w.unit)
	exec(`INSERT INTO stock_balances (company_id, store_id, product_id, quantity, avg_cost, cost_currency, cost_complete) VALUES ($1,$2,$3,20,4,'USD',true)`, w.company, w.store, w.product)
	exec(`INSERT INTO customers (id, company_id, store_id, name) VALUES ($1,$2,$3,'Client')`, w.customer, w.company, w.store)
	exec(`INSERT INTO cash_registers (id, company_id, store_id, name) VALUES ($1,$2,$3,'Caisse')`, w.register, w.company, w.store)
	exec(`INSERT INTO cash_sessions (id, company_id, store_id, register_id, opened_by, status) VALUES ($1,$2,$3,$4,$5,'open')`, w.session, w.company, w.store, w.register, w.user)
	return w
}

func saleInput(w world, key string, qty, paid string) commerce.ConfirmSaleInput {
	return commerce.ConfirmSaleInput{
		IdempotencyKey: key,
		StoreID:        w.store,
		CustomerID:     &w.customer,
		Currency:       "USD",
		Lines: []commerce.SaleLineInput{{
			ProductID: w.product, UnitName: "piece",
			Quantity: money.Must(qty), ExpectedUnitPrice: money.Must("10"),
		}},
		Payments: []commerce.PaymentInput{{Method: "external", Amount: money.Must(paid)}},
	}
}

func TestAppRoleCannotBypassRLS(t *testing.T) {
	_, app, _, _ := testutil.Pools(t)
	var name string
	var super, bypass bool
	if err := app.QueryRow(context.Background(), `
		SELECT rolname, rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user
	`).Scan(&name, &super, &bypass); err != nil {
		t.Fatal(err)
	}
	if name != "easystock_app" || super || bypass {
		t.Fatalf("role = %s super=%v bypass=%v", name, super, bypass)
	}
}

func TestSaleCreditDebtAndReplay(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := seed(t, admin)
	svc := commerce.New(app)
	ctx := context.Background()
	in := saleInput(w, "sale-credit", "10", "30")
	first, err := svc.ConfirmSale(ctx, w.p, in)
	if err != nil {
		t.Fatal(err)
	}
	if first.Net != "100" || first.Paid != "30" || first.Due != "70" || first.DebtID == nil {
		t.Fatalf("sale = %+v", first)
	}
	second, err := svc.ConfirmSale(ctx, w.p, in)
	if err != nil {
		t.Fatal(err)
	}
	if second.SaleID != first.SaleID {
		t.Fatal("replay created another sale")
	}
	var sales int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM sales WHERE company_id = $1`, w.company).Scan(&sales); err != nil || sales != 1 {
		t.Fatalf("sales=%d err=%v", sales, err)
	}
	paid, err := svc.PayDebt(ctx, w.p, commerce.PayDebtInput{
		IdempotencyKey: "debt-20", DebtID: *first.DebtID, Amount: money.Must("20"), Method: "external",
	})
	if err != nil {
		t.Fatal(err)
	}
	if paid.Due != "50" || paid.Paid != "20" {
		t.Fatalf("debt = %+v", paid)
	}
	changed := in
	changed.Lines[0].Quantity = money.Must("1")
	_, err = svc.ConfirmSale(ctx, w.p, changed)
	if de, ok := domain.As(err); !ok || de.Reason != "idempotency_mismatch" {
		t.Fatalf("mismatch = %v", err)
	}
}

func TestConcurrentLastUnitAndIdempotency(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	ctx := context.Background()
	w := seed(t, admin)
	if _, err := admin.Exec(ctx, `UPDATE stock_balances SET quantity = 1 WHERE company_id = $1`, w.company); err != nil {
		t.Fatal(err)
	}
	svc := commerce.New(app)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = svc.ConfirmSale(ctx, w.p, saleInput(w, uuid.NewString(), "1", "10"))
		}(i)
	}
	wg.Wait()
	success := 0
	for _, err := range errs {
		if err == nil {
			success++
			continue
		}
		if de, ok := domain.As(err); !ok || de.Reason != "stock_conflict" {
			t.Fatalf("concurrent error = %v", err)
		}
	}
	if success != 1 {
		t.Fatalf("successes = %d", success)
	}

	w2 := seed(t, admin)
	svc = commerce.New(app)
	in := saleInput(w2, "same-key", "1", "10")
	ids := make([]uuid.UUID, 2)
	errs = make([]error, 2)
	wg = sync.WaitGroup{}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result, err := svc.ConfirmSale(ctx, w2.p, in)
			errs[i] = err
			ids[i] = result.SaleID
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if ids[0] == uuid.Nil || ids[0] != ids[1] {
		t.Fatalf("ids = %v", ids)
	}
	var n int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM sales WHERE company_id = $1`, w2.company).Scan(&n); err != nil || n != 1 {
		t.Fatalf("idempotent sales=%d err=%v", n, err)
	}
}

func TestSaleRollbackLeavesNoTrace(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := seed(t, admin)
	svc := commerce.New(app)
	svc.SetAfterWrite(func() error { return errors.New("boom") })
	if _, err := svc.ConfirmSale(context.Background(), w.p, saleInput(w, "rollback", "1", "10")); err == nil {
		t.Fatal("expected rollback")
	}
	var sales int
	ctx := context.Background()
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM sales WHERE company_id = $1`, w.company).Scan(&sales); err != nil || sales != 0 {
		t.Fatalf("sales=%d err=%v", sales, err)
	}
	eqNum(t, admin, `SELECT quantity::text FROM stock_balances WHERE company_id = $1`, w.company, "20")
}

func TestRefundRestoresResellableStock(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := seed(t, admin)
	svc := commerce.New(app)
	ctx := context.Background()
	in := saleInput(w, "refund-sale", "2", "20")
	in.Payments[0].Method = "cash"
	in.CashSessionID = &w.session
	sale, err := svc.ConfirmSale(ctx, w.p, in)
	if err != nil {
		t.Fatal(err)
	}
	var itemID uuid.UUID
	if err := admin.QueryRow(ctx, `SELECT id FROM sale_items WHERE sale_id = $1`, sale.SaleID).Scan(&itemID); err != nil {
		t.Fatal(err)
	}
	result, err := svc.CreateRefund(ctx, w.p, commerce.CreateRefundInput{
		IdempotencyKey: "refund-1",
		SaleID:         sale.SaleID,
		Reason:         "client a changé d'avis",
		CashSessionID:  &w.session,
		Lines:          []commerce.RefundLineInput{{SaleItemID: itemID, Quantity: money.Must("2"), Restock: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Amount != "20" || result.CashAmount != "20" {
		t.Fatalf("refund = %+v", result)
	}
	eqNum(t, admin, `SELECT quantity::text FROM stock_balances WHERE company_id = $1`, w.company, "20")
}

func TestCashCloseIsImmutable(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := seed(t, admin)
	svc := commerce.New(app)
	ctx := context.Background()
	in := saleInput(w, "cash-sale", "1", "10")
	in.Payments[0].Method = "cash"
	in.CashSessionID = &w.session
	if _, err := svc.ConfirmSale(ctx, w.p, in); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseCashSession(ctx, w.p, commerce.CloseCashInput{
		SessionID: w.session,
		Counts:    []commerce.CloseCount{{Currency: "USD", Counted: money.Must("10")}},
	}); err != nil {
		t.Fatal(err)
	}
	_, err := admin.Exec(ctx, `UPDATE cash_sessions SET closed_by = opened_by WHERE id = $1`, w.session)
	if err == nil {
		t.Fatal("closed session accepted an update")
	}
	next, err := svc.OpenCashSession(ctx, w.p, commerce.OpenCashInput{RegisterID: w.register})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AdjustCash(ctx, w.p, commerce.CashAdjustmentInput{
		SessionID: next, Currency: "USD", Direction: "out", Amount: money.Must("1"),
		Reason: "correction", ReferenceSessionID: w.session,
	}); err != nil {
		t.Fatal(err)
	}
	eqNum(t, admin, `SELECT counted_amount::text FROM cash_session_counts WHERE session_id = $1`, w.session, "10")
}

func TestTenantIsolationAndContextLeak(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	a := seed(t, admin)
	b := seed(t, admin)
	svc := commerce.New(app)
	ctx := context.Background()
	sale, err := svc.ConfirmSale(ctx, a.p, saleInput(a, "iso", "1", "10"))
	if err != nil {
		t.Fatal(err)
	}
	conn, err := app.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tenant.Set(ctx, tx, b.p); err != nil {
		t.Fatal(err)
	}
	var visible int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM sales WHERE id = $1`, sale.SaleID).Scan(&visible); err != nil {
		t.Fatal(err)
	}
	if visible != 0 {
		t.Fatal("company B saw company A sale")
	}
	if _, err := tx.Exec(ctx, `SET row_security = off`); err != nil {
		t.Fatal(err)
	}
	err = tx.QueryRow(ctx, `SELECT count(*) FROM sales WHERE id = $1`, sale.SaleID).Scan(&visible)
	if err == nil && visible != 0 {
		t.Fatal("row_security off exposed another company")
	}
	_ = tx.Rollback(ctx)
	var leaked string
	if err := conn.QueryRow(ctx, `SELECT current_setting('app.company_id', true)`).Scan(&leaked); err != nil {
		t.Fatal(err)
	}
	if leaked != "" {
		t.Fatalf("tenant leaked on connection: %q", leaked)
	}
}

func TestMarginExportAndExpiredSubscription(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := seed(t, admin)
	ctx := context.Background()
	cashier := uuid.New()
	if _, err := admin.Exec(ctx, `INSERT INTO users (id, display_name) VALUES ($1,'Cashier')`, cashier); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO organization_memberships (company_id, user_id, org_role, status) VALUES ($1,$2,'cashier','active')`, w.company, cashier); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO store_assignments (company_id, store_id, user_id) VALUES ($1,$2,$3)`, w.company, w.store, cashier); err != nil {
		t.Fatal(err)
	}
	reportsSvc := reports.New(app)
	filter := reports.Filter{StoreIDs: []uuid.UUID{w.store}, Currency: "USD", From: time.Now().Add(-time.Hour), To: time.Now().Add(time.Hour)}
	cashierP := w.p
	cashierP.UserID = cashier
	cashierP.OrgRole = "cashier"
	if _, err := reportsSvc.ExportMargin(ctx, cashierP, filter); err == nil {
		t.Fatal("cashier read a margin export")
	}
	if _, err := reportsSvc.ExportMargin(ctx, w.p, filter); err == nil {
		t.Fatal("essentiel plan exported a margin")
	}
	if _, err := admin.Exec(ctx, `UPDATE subscription_snapshots SET plan_code = 'pro' WHERE company_id = $1`, w.company); err != nil {
		t.Fatal(err)
	}
	csv, err := reportsSvc.ExportMargin(ctx, w.p, filter)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(csv, "gross_margin") || strings.Contains(csv, "net_profit") {
		t.Fatalf("csv = %s", csv)
	}
	if _, err := reportsSvc.ExportMargin(ctx, cashierP, filter); err == nil {
		t.Fatal("cashier margin export succeeded on pro")
	}
	if _, err := admin.Exec(ctx, `UPDATE subscription_snapshots SET status = 'expired' WHERE company_id = $1`, w.company); err != nil {
		t.Fatal(err)
	}
	if _, err := commerce.New(app).ConfirmSale(ctx, w.p, saleInput(w, "expired", "1", "10")); err == nil {
		t.Fatal("expired subscription accepted a sale")
	}
	if _, err := reportsSvc.NetSales(ctx, w.p, filter); err != nil {
		t.Fatalf("read after expiry: %v", err)
	}
}

func TestQuotasRolesAndLocalPayment(t *testing.T) {
	admin, app, identityPool, job := testutil.Pools(t)
	w := seed(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	if _, err := svc.CreateStore(ctx, w.p, "Deuxième", "Africa/Lubumbashi"); err == nil {
		t.Fatal("essentiel accepted a second store")
	}
	other := uuid.New()
	if _, err := admin.Exec(ctx, `INSERT INTO users (id, display_name) VALUES ($1,'Admin')`, other); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO organization_memberships (company_id, user_id, org_role, status) VALUES ($1,$2,'organization_admin','active')`, w.company, other); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetMemberRole(ctx, w.p, other, "cashier"); err == nil {
		t.Fatal("essentiel accepted an operational role")
	}
	if err := svc.SetMemberRole(ctx, w.p, w.user, "cashier"); err == nil {
		t.Fatal("owner changed own role")
	}
	subs := subscription.New(app, job)
	paymentID, err := subs.SubmitLocalPayment(ctx, w.p, subscription.LocalPaymentInput{
		PlanCode: "pro", Amount: decimal.RequireFromString("10"), Currency: "USD",
		ExternalReference: "AIRTEL-1", PeriodStart: time.Now(), PeriodEnd: time.Now().AddDate(0, 1, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := subs.ConfirmLocalPayment(ctx, w.p, paymentID); err != nil {
		t.Fatal(err)
	}
	var source, status string
	if err := admin.QueryRow(ctx, `SELECT source, status FROM subscription_snapshots WHERE company_id = $1`, w.company).Scan(&source, &status); err != nil {
		t.Fatal(err)
	}
	if source != "local" || status != "active" {
		t.Fatalf("subscription %s %s", source, status)
	}
	var grants int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM entitlement_grants WHERE company_id = $1 AND source = 'local'`, w.company).Scan(&grants); err != nil || grants != 1 {
		t.Fatalf("grants=%d err=%v", grants, err)
	}
	if _, err := admin.Exec(ctx, `
		UPDATE subscription_snapshots
		SET source = 'clerk', status = 'active', current_period_end = now() + interval '10 days'
		WHERE company_id = $1
	`, w.company); err != nil {
		t.Fatal(err)
	}
	second, err := subs.SubmitLocalPayment(ctx, w.p, subscription.LocalPaymentInput{
		PlanCode: "pro", Amount: decimal.RequireFromString("10"), Currency: "USD",
		ExternalReference: "AIRTEL-2", PeriodStart: time.Now(), PeriodEnd: time.Now().AddDate(0, 1, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := subs.ConfirmLocalPayment(ctx, w.p, second); err == nil {
		t.Fatal("overlapping clerk period was activated locally")
	}
	idSvc := identity.New(identityPool)
	now := time.Now().UTC()
	org := []byte(`{"id":"org_clerk","name":"Nouveau"}`)
	if err := idSvc.Apply(ctx, identity.Event{ID: "evt_org", Type: "organization.created", OccurredAt: now, Data: org}); err != nil {
		t.Fatal(err)
	}
	if err := idSvc.Apply(ctx, identity.Event{ID: "evt_org", Type: "organization.created", OccurredAt: now, Data: org}); err != nil {
		t.Fatal(err)
	}
	var events int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM webhook_events WHERE event_id = 'evt_org'`).Scan(&events); err != nil || events != 1 {
		t.Fatalf("events=%d err=%v", events, err)
	}
	later := now.Add(time.Minute)
	if err := idSvc.Apply(ctx, identity.Event{ID: "evt_org_2", Type: "organization.updated", OccurredAt: later, Data: []byte(`{"id":"org_clerk","name":"Récent"}`)}); err != nil {
		t.Fatal(err)
	}
	if err := idSvc.Apply(ctx, identity.Event{ID: "evt_org_old", Type: "organization.updated", OccurredAt: now.Add(-time.Minute), Data: []byte(`{"id":"org_clerk","name":"Ancien"}`)}); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := admin.QueryRow(ctx, `SELECT name FROM companies WHERE clerk_org_id = 'org_clerk'`).Scan(&name); err != nil || name != "Récent" {
		t.Fatalf("name=%s err=%v", name, err)
	}
}

func eqNum(t *testing.T, pool *pgxpool.Pool, query string, arg any, want string) {
	t.Helper()
	var got string
	if err := pool.QueryRow(context.Background(), query, arg).Scan(&got); err != nil {
		t.Fatal(err)
	}
	g, err := decimal.NewFromString(got)
	if err != nil {
		t.Fatal(err)
	}
	if !g.Equal(decimal.RequireFromString(want)) {
		t.Fatalf("%s != %s", got, want)
	}
}
