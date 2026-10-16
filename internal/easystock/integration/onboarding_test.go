package integration

import (
	"context"
	"testing"
	"time"

	"rangoapp/internal/easystock/api"
	"rangoapp/internal/easystock/commerce"
	"rangoapp/internal/easystock/identity"
	"rangoapp/internal/easystock/readmodel"
	"rangoapp/internal/easystock/reports"
	"rangoapp/internal/easystock/tenant"
	"rangoapp/internal/easystock/testutil"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestCreateCompanyInvitationAndRegistration(t *testing.T) {
	admin, app, identityPool, _ := testutil.Pools(t)
	ctx := context.Background()
	idSvc := identity.New(identityPool)
	commerceSvc := commerce.New(app)
	reads := readmodel.New(app)
	schema, err := api.Schema(api.Deps{Identity: idSvc, Commerce: commerceSvc, Reads: reads, ContextSecret: "test-secret-test-secret-test-secret"})
	if err != nil {
		t.Fatal(err)
	}
	userID, err := idSvc.EnsureUser(ctx, "user_owner", "Ada")
	if err != nil {
		t.Fatal(err)
	}
	session := api.WithPrincipal(ctx, tenant.Principal{UserID: userID, RequestID: "signup"})
	query := `mutation ($input: CreateCompanyInput!) {
		createCompany(input: $input) { companyId storeId role primaryCurrency }
	}`
	vars := map[string]any{"input": map[string]any{"name": "Beni Market", "primaryCurrency": "CDF", "storeName": "Centre"}}
	first := api.Execute(session, schema, query, vars)
	if len(first.Errors) > 0 {
		t.Fatal(first.Errors)
	}
	created := first.Data.(map[string]any)["createCompany"].(map[string]any)
	if created["role"] != "owner" || created["primaryCurrency"] != "CDF" || created["storeId"] == nil {
		t.Fatalf("%v", created)
	}
	companyID := uuid.MustParse(created["companyId"].(string))
	storeID := uuid.MustParse(created["storeId"].(string))
	second := api.Execute(session, schema, query, vars)
	if len(second.Errors) > 0 {
		t.Fatal(second.Errors)
	}
	again := second.Data.(map[string]any)["createCompany"].(map[string]any)
	if again["companyId"] != companyID.String() {
		t.Fatal("retry created another company")
	}
	var companies int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM companies WHERE created_by = $1`, userID).Scan(&companies); err != nil || companies != 1 {
		t.Fatalf("companies=%d err=%v", companies, err)
	}
	member := api.WithPrincipal(ctx, tenant.Principal{UserID: userID, CompanyID: companyID, OrgRole: "owner"})
	reg := api.Execute(member, schema, `{ registration { clerkLinked steps { code done } } }`, nil)
	if len(reg.Errors) > 0 {
		t.Fatal(reg.Errors)
	}
	view := reg.Data.(map[string]any)["registration"].(map[string]any)
	if view["clerkLinked"] != false {
		t.Fatal(view)
	}
	steps := view["steps"].([]any)
	if steps[0].(map[string]any)["done"] != true || steps[1].(map[string]any)["done"] != true || steps[2].(map[string]any)["done"] != false {
		t.Fatalf("%v", steps)
	}
	header, err := idSvc.ResolveSession(ctx, "user_owner", "", companyID.String())
	if err != nil || header.CompanyID != companyID {
		t.Fatalf("header session %v %v", header, err)
	}
	if _, err := idSvc.ResolveSession(ctx, "user_owner", "", uuid.NewString()); err == nil {
		t.Fatal("foreign company header was accepted")
	}
	now := time.Now().UTC()
	if err := idSvc.Apply(ctx, identity.Event{ID: "evt_link", Type: "organization.created", OccurredAt: now, Data: []byte(`{"id":"org_beni","name":"Beni Market","created_by":"user_owner"}`)}); err != nil {
		t.Fatal(err)
	}
	var linked int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM companies WHERE clerk_org_id = 'org_beni'`).Scan(&linked); err != nil || linked != 1 {
		t.Fatalf("linked=%d err=%v", linked, err)
	}
	if _, err := idSvc.ResolveSession(ctx, "user_owner", "", companyID.String()); err == nil {
		t.Fatal("header still selected a linked company")
	}
	orgSession, err := idSvc.ResolveSession(ctx, "user_owner", "org_beni", "")
	if err != nil || orgSession.CompanyID != companyID {
		t.Fatalf("org session %v %v", orgSession, err)
	}
	if _, err := admin.Exec(ctx, `UPDATE subscription_snapshots SET plan_code = 'pro' WHERE company_id = $1`, companyID); err != nil {
		t.Fatal(err)
	}
	inviteQuery := `mutation ($input: InviteMemberInput!) { inviteMember(input: $input) { id email role token } }`
	invited := api.Execute(member, schema, inviteQuery, map[string]any{"input": map[string]any{
		"email": "cashier@shop.test", "role": "cashier", "storeId": storeID.String(),
	}})
	if len(invited.Errors) > 0 {
		t.Fatal(invited.Errors)
	}
	invite := invited.Data.(map[string]any)["inviteMember"].(map[string]any)
	token, _ := invite["token"].(string)
	if token == "" {
		t.Fatal("missing invitation token")
	}
	guestID, err := idSvc.EnsureUser(ctx, "user_cashier", "Cashier")
	if err != nil {
		t.Fatal(err)
	}
	accepted := api.Execute(api.WithPrincipal(ctx, tenant.Principal{UserID: guestID}), schema, `mutation ($token: String!) {
		acceptInvitation(token: $token) { companyId role storeId }
	}`, map[string]any{"token": token})
	if len(accepted.Errors) > 0 {
		t.Fatal(accepted.Errors)
	}
	body := accepted.Data.(map[string]any)["acceptInvitation"].(map[string]any)
	if body["role"] != "cashier" || body["storeId"] != storeID.String() {
		t.Fatalf("%v", body)
	}
	var assigned bool
	if err := admin.QueryRow(ctx, `SELECT true FROM store_assignments WHERE user_id = $1 AND store_id = $2`, guestID, storeID).Scan(&assigned); err != nil || !assigned {
		t.Fatal(err)
	}
	replay := api.Execute(api.WithPrincipal(ctx, tenant.Principal{UserID: guestID}), schema, `mutation ($token: String!) {
		acceptInvitation(token: $token) { role }
	}`, map[string]any{"token": token})
	if len(replay.Errors) > 0 {
		t.Fatal(replay.Errors)
	}
	if _, err := admin.Exec(ctx, `UPDATE subscription_snapshots SET plan_code = 'essentiel' WHERE company_id = $1`, companyID); err != nil {
		t.Fatal(err)
	}
	blocked := api.Execute(member, schema, inviteQuery, map[string]any{"input": map[string]any{
		"email": "stock@shop.test", "role": "stockkeeper", "storeId": storeID.String(),
	}})
	if len(blocked.Errors) == 0 {
		t.Fatal("essentiel accepted an operational invitation")
	}
	tx, err := app.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := tenant.Set(ctx, tx, tenant.Principal{CompanyID: uuid.New(), UserID: userID}); err != nil {
		t.Fatal(err)
	}
	var visible int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM invitations`).Scan(&visible); err != nil {
		t.Fatal(err)
	}
	if visible != 0 {
		t.Fatalf("other company sees %d invitations", visible)
	}
}

func TestAmbiguousOrganizationsAreNotMerged(t *testing.T) {
	admin, _, identityPool, _ := testutil.Pools(t)
	ctx := context.Background()
	idSvc := identity.New(identityPool)
	userID, err := idSvc.EnsureUser(ctx, "user_a", "A")
	if err != nil {
		t.Fatal(err)
	}
	first, err := idSvc.CreateCompany(ctx, userID, "", identity.CompanyInput{Name: "Une", PrimaryCurrency: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	extra := uuid.New()
	if _, err := admin.Exec(ctx, `INSERT INTO companies (id, name, primary_currency) VALUES ($1, 'Autre', 'USD')`, extra); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO organization_memberships (company_id, user_id, org_role, status) VALUES ($1,$2,'owner','active')`, extra, userID); err != nil {
		t.Fatal(err)
	}
	if err := idSvc.Apply(ctx, identity.Event{ID: "evt_multi", Type: "organization.created", OccurredAt: time.Now().UTC(), Data: []byte(`{"id":"org_new","name":"Nouvelle","created_by":"user_a"}`)}); err != nil {
		t.Fatal(err)
	}
	var stillUnlinked bool
	if err := admin.QueryRow(ctx, `SELECT clerk_org_id IS NULL FROM companies WHERE id = $1`, first.CompanyID).Scan(&stillUnlinked); err != nil || !stillUnlinked {
		t.Fatalf("linked=%v err=%v", !stillUnlinked, err)
	}
	var named string
	if err := admin.QueryRow(ctx, `SELECT name FROM companies WHERE clerk_org_id = 'org_new'`).Scan(&named); err != nil || named != "Nouvelle" {
		t.Fatalf("name=%s err=%v", named, err)
	}
}

func TestGraphQLReadsSalesCustomersDebtsStockCashReports(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	ctx := context.Background()
	w := seed(t, admin)
	other := seed(t, admin)
	if _, err := admin.Exec(ctx, `
		INSERT INTO cash_transactions (company_id, session_id, currency, direction, kind, amount, created_by)
		VALUES ($1,$2,'USD','in','opening',50,$3)
	`, w.company, w.session, w.user); err != nil {
		t.Fatal(err)
	}
	svc := commerce.New(app)
	sale, err := svc.ConfirmSale(ctx, w.p, saleInput(w, "read-sale", "10", "30"))
	if err != nil {
		t.Fatal(err)
	}
	cashier := uuid.New()
	if _, err := admin.Exec(ctx, `INSERT INTO users (id, display_name) VALUES ($1, 'Vendeur')`, cashier); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO organization_memberships (company_id, user_id, org_role, status) VALUES ($1,$2,'cashier','active')`, w.company, cashier); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO store_assignments (company_id, store_id, user_id) VALUES ($1,$2,$3)`, w.company, w.store, cashier); err != nil {
		t.Fatal(err)
	}
	cashierPrincipal := tenant.Principal{UserID: cashier, CompanyID: w.company, OrgRole: "cashier"}
	if _, err := svc.ConfirmSale(ctx, cashierPrincipal, saleInput(w, "cashier-sale", "1", "10")); err != nil {
		t.Fatal(err)
	}
	schema, err := api.Schema(api.Deps{
		Reads: readmodel.New(app), Reports: reports.New(app), ContextSecret: "test-secret-test-secret-test-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	from := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	to := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	query := `query ($store: ID!, $from: DateTime!, $to: DateTime!) {
		sales(storeId: $store) { id netAmount lines { quantity lineNet } }
		customers(storeId: $store) { name }
		debts(storeId: $store) { dueAmount status customerName }
		stock(storeId: $store) { name quantity belowAlert }
		cash(storeId: $store) { name openSession { balances { currency amount } } }
		reports(storeId: $store, currency: "USD", from: $from, to: $to) {
			netSales { amount }
			paymentsByMethod { method amount }
			openDebts { openAmount }
			expenses { amount }
			lowStock { sku }
			topProducts { name netAmount }
			grossMargin { grossMargin }
			cashCounts { currency }
		}
	}`
	vars := map[string]any{"store": w.store.String(), "from": from, "to": to}
	result := api.Execute(api.WithPrincipal(ctx, w.p), schema, query, vars)
	if result.Data == nil {
		t.Fatal(result.Errors)
	}
	data := result.Data.(map[string]any)
	sales := data["sales"].([]any)
	if len(sales) != 2 {
		t.Fatalf("sales %v", sales)
	}
	found := false
	for _, item := range sales {
		if item.(map[string]any)["id"] == sale.SaleID.String() {
			found = true
		}
	}
	if !found {
		t.Fatal("owner sale missing")
	}
	customers := data["customers"].([]any)
	if len(customers) != 1 || customers[0].(map[string]any)["name"] != "Client" {
		t.Fatalf("%v", customers)
	}
	debts := data["debts"].([]any)
	if len(debts) != 1 || !decimalEqual(debts[0].(map[string]any)["dueAmount"].(string), "70") {
		t.Fatalf("%v", debts)
	}
	stock := data["stock"].([]any)
	if len(stock) != 1 || stock[0].(map[string]any)["belowAlert"] != false {
		t.Fatalf("%v", stock)
	}
	if !decimalEqual(stock[0].(map[string]any)["quantity"].(string), "9") {
		t.Fatalf("stock %v", stock[0])
	}
	registers := data["cash"].([]any)
	session := registers[0].(map[string]any)["openSession"].(map[string]any)
	balances := session["balances"].([]any)
	if len(balances) != 1 || !decimalEqual(balances[0].(map[string]any)["amount"].(string), "50") {
		t.Fatalf("%v", balances)
	}
	report := data["reports"].(map[string]any)
	if !decimalEqual(report["netSales"].(map[string]any)["amount"].(string), "110") {
		t.Fatalf("net %v", report["netSales"])
	}
	if !decimalEqual(report["openDebts"].(map[string]any)["openAmount"].(string), "70") {
		t.Fatalf("debts %v", report["openDebts"])
	}
	if report["grossMargin"] != nil || len(result.Errors) == 0 {
		t.Fatalf("margin should be hidden on essentiel, data=%v errors=%v", report["grossMargin"], result.Errors)
	}
	cashierResult := api.Execute(api.WithPrincipal(ctx, cashierPrincipal), schema, `query ($store: ID!) { sales(storeId: $store) { id } }`, map[string]any{"store": w.store.String()})
	if len(cashierResult.Errors) > 0 {
		t.Fatal(cashierResult.Errors)
	}
	own := cashierResult.Data.(map[string]any)["sales"].([]any)
	if len(own) != 1 {
		t.Fatalf("cashier sees %d sales", len(own))
	}
	foreign := api.Execute(api.WithPrincipal(ctx, other.p), schema, `query ($store: ID!) { customers(storeId: $store) { name } }`, map[string]any{"store": w.store.String()})
	if len(foreign.Errors) == 0 {
		t.Fatal("other company read the store")
	}
}

func decimalEqual(got, want string) bool {
	left, err := decimal.NewFromString(got)
	if err != nil {
		return false
	}
	right, err := decimal.NewFromString(want)
	if err != nil {
		return false
	}
	return left.Equal(right)
}

func TestStoresListAndCreateStoreDoNotOpenACompany(t *testing.T) {
	admin, app, identityPool, _ := testutil.Pools(t)
	ctx := context.Background()
	idSvc := identity.New(identityPool)
	schema, err := api.Schema(api.Deps{
		Identity: idSvc, Commerce: commerce.New(app), Reads: readmodel.New(app),
		ContextSecret: "test-secret-test-secret-test-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	userID, err := idSvc.EnsureUser(ctx, "user_stores", "Awa")
	if err != nil {
		t.Fatal(err)
	}
	session := api.WithPrincipal(ctx, tenant.Principal{UserID: userID, RequestID: "stores"})
	created := api.Execute(session, schema, `mutation ($input: CreateCompanyInput!) {
		createCompany(input: $input) { companyId storeId }
	}`, map[string]any{"input": map[string]any{
		"name": "Kivu", "primaryCurrency": "USD", "storeName": "Centre", "timezone": "Africa/Lubumbashi",
	}})
	if len(created.Errors) > 0 {
		t.Fatal(created.Errors)
	}
	body := created.Data.(map[string]any)["createCompany"].(map[string]any)
	companyID := uuid.MustParse(body["companyId"].(string))
	storeID := uuid.MustParse(body["storeId"].(string))
	owner := api.WithPrincipal(ctx, tenant.Principal{UserID: userID, CompanyID: companyID, OrgRole: "owner"})
	listed := api.Execute(owner, schema, `{ stores { id name timezone cashRegister { id name } } }`, nil)
	if len(listed.Errors) > 0 {
		t.Fatal(listed.Errors)
	}
	stores := listed.Data.(map[string]any)["stores"].([]any)
	if len(stores) != 1 {
		t.Fatalf("%v", stores)
	}
	first := stores[0].(map[string]any)
	if first["id"] != storeID.String() || first["name"] != "Centre" || first["timezone"] != "Africa/Lubumbashi" {
		t.Fatalf("%v", first)
	}
	register, _ := first["cashRegister"].(map[string]any)
	if register == nil || register["name"] != "Caisse principale" || register["id"] == "" {
		t.Fatalf("%v", first)
	}
	sellerID, err := idSvc.EnsureUser(ctx, "user_stores_seller", "Vendeur")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
		INSERT INTO organization_memberships (company_id, user_id, org_role, status)
		VALUES ($1,$2,'seller','active')
	`, companyID, sellerID); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
		INSERT INTO store_assignments (company_id, store_id, user_id) VALUES ($1,$2,$3)
	`, companyID, storeID, sellerID); err != nil {
		t.Fatal(err)
	}
	seller := api.WithPrincipal(ctx, tenant.Principal{UserID: sellerID, CompanyID: companyID, OrgRole: "seller"})
	sellerStores := api.Execute(seller, schema, `{ stores { id } }`, nil)
	if len(sellerStores.Errors) > 0 {
		t.Fatal(sellerStores.Errors)
	}
	seen := sellerStores.Data.(map[string]any)["stores"].([]any)
	if len(seen) != 1 || seen[0].(map[string]any)["id"] != storeID.String() {
		t.Fatalf("%v", seen)
	}
	denied := api.Execute(seller, schema, `mutation ($input: CreateStoreInput!) {
		createStore(input: $input) { id }
	}`, map[string]any{"input": map[string]any{"name": "Annexe"}})
	if len(denied.Errors) == 0 {
		t.Fatal("seller opened a store")
	}
	if _, err := admin.Exec(ctx, `UPDATE subscription_snapshots SET plan_code = 'business' WHERE company_id = $1`, companyID); err != nil {
		t.Fatal(err)
	}
	var companiesBefore int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM companies WHERE created_by = $1`, userID).Scan(&companiesBefore); err != nil {
		t.Fatal(err)
	}
	second := api.Execute(owner, schema, `mutation ($input: CreateStoreInput!) {
		createStore(input: $input) { id name timezone cashRegister { name } }
	}`, map[string]any{"input": map[string]any{"name": "Annexe", "timezone": "Africa/Kinshasa"}})
	if len(second.Errors) > 0 {
		t.Fatal(second.Errors)
	}
	createdStore := second.Data.(map[string]any)["createStore"].(map[string]any)
	if createdStore["id"] == storeID.String() || createdStore["name"] != "Annexe" || createdStore["timezone"] != "Africa/Kinshasa" {
		t.Fatalf("%v", createdStore)
	}
	if createdStore["cashRegister"].(map[string]any)["name"] != "Caisse principale" {
		t.Fatalf("%v", createdStore)
	}
	var companiesAfter int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM companies WHERE created_by = $1`, userID).Scan(&companiesAfter); err != nil || companiesAfter != companiesBefore {
		t.Fatalf("companies %d -> %d err=%v", companiesBefore, companiesAfter, err)
	}
	again := api.Execute(owner, schema, `{ stores { id } }`, nil)
	if len(again.Errors) > 0 {
		t.Fatal(again.Errors)
	}
	if len(again.Data.(map[string]any)["stores"].([]any)) != 2 {
		t.Fatalf("%v", again.Data)
	}
	sellerAgain := api.Execute(seller, schema, `{ stores { id } }`, nil)
	if len(sellerAgain.Errors) > 0 {
		t.Fatal(sellerAgain.Errors)
	}
	if len(sellerAgain.Data.(map[string]any)["stores"].([]any)) != 1 {
		t.Fatal("seller sees a boutique they are not assigned to")
	}
}

func TestRegistrationWithoutUser(t *testing.T) {
	schema, err := api.Schema(api.Deps{})
	if err != nil {
		t.Fatal(err)
	}
	result := api.Execute(context.Background(), schema, `{ registration { clerkLinked } }`, nil)
	if len(result.Errors) == 0 {
		t.Fatal("expected authentication error")
	}
}
