package integration

import (
	"context"
	"strings"
	"testing"

	"rangoapp/internal/easystock/api"
	"rangoapp/internal/easystock/commerce"
	"rangoapp/internal/easystock/media"
	"rangoapp/internal/easystock/tenant"
	"rangoapp/internal/easystock/testutil"

	"github.com/google/uuid"
)

func TestExchangeRatesConvertWithoutMixingCompanies(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := seed(t, admin)
	ctx := context.Background()
	svc := commerce.New(app)
	schema, err := api.Schema(api.Deps{Commerce: svc, ContextSecret: "test-secret-test-secret-test-secret"})
	if err != nil {
		t.Fatal(err)
	}
	owner := api.WithPrincipal(ctx, w.p)
	set := `mutation ($input: SetExchangeRateInput!) {
		setExchangeRate(input: $input) { fromCurrency toCurrency rate }
	}`
	first := api.Execute(owner, schema, set, map[string]any{"input": map[string]any{
		"fromCurrency": "usd", "toCurrency": "CDF", "rate": "2800.5",
	}})
	if len(first.Errors) > 0 {
		t.Fatal(first.Errors)
	}
	direct := api.Execute(owner, schema, `query {
		convertAmount(amount: "10", fromCurrency: "USD", toCurrency: "CDF") { amount currency rate basis }
	}`, nil)
	if len(direct.Errors) > 0 {
		t.Fatal(direct.Errors)
	}
	got := direct.Data.(map[string]any)["convertAmount"].(map[string]any)
	if got["amount"] != "28005.0000" || got["basis"] != "direct" || got["rate"] != "2800.50000000" {
		t.Fatalf("%v", got)
	}
	back := api.Execute(owner, schema, `query {
		convertAmount(amount: "2800.5", fromCurrency: "CDF", toCurrency: "USD") { amount basis }
	}`, nil)
	if len(back.Errors) > 0 {
		t.Fatal(back.Errors)
	}
	inverse := back.Data.(map[string]any)["convertAmount"].(map[string]any)
	if inverse["amount"] != "1.0000" || inverse["basis"] != "inverse" {
		t.Fatalf("%v", inverse)
	}
	same := api.Execute(owner, schema, `query {
		convertAmount(amount: "10", fromCurrency: "USD", toCurrency: "USD") { amount basis }
	}`, nil)
	if len(same.Errors) > 0 {
		t.Fatal(same.Errors)
	}
	if same.Data.(map[string]any)["convertAmount"].(map[string]any)["basis"] != "same" {
		t.Fatal(same.Data)
	}
	again := api.Execute(owner, schema, set, map[string]any{"input": map[string]any{
		"fromCurrency": "USD", "toCurrency": "CDF", "rate": "2900",
	}})
	if len(again.Errors) > 0 {
		t.Fatal(again.Errors)
	}
	var rates int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM company_exchange_rates WHERE company_id = $1`, w.company).Scan(&rates); err != nil || rates != 1 {
		t.Fatalf("rates=%d err=%v", rates, err)
	}

	cashierID := uuid.New()
	if _, err := admin.Exec(ctx, `INSERT INTO users (id, display_name) VALUES ($1, 'Caisse')`, cashierID); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
		INSERT INTO organization_memberships (company_id, user_id, org_role, status) VALUES ($1,$2,'cashier','active')
	`, w.company, cashierID); err != nil {
		t.Fatal(err)
	}
	cashier := api.WithPrincipal(ctx, tenant.Principal{UserID: cashierID, CompanyID: w.company})
	listed := api.Execute(cashier, schema, `{ exchangeRates { fromCurrency toCurrency } }`, nil)
	if len(listed.Errors) > 0 {
		t.Fatal(listed.Errors)
	}
	if len(listed.Data.(map[string]any)["exchangeRates"].([]any)) != 1 {
		t.Fatal(listed.Data)
	}
	denied := api.Execute(cashier, schema, set, map[string]any{"input": map[string]any{
		"fromCurrency": "EUR", "toCurrency": "USD", "rate": "1.1",
	}})
	if len(denied.Errors) == 0 {
		t.Fatal("cashier set a company rate")
	}

	otherCompany, otherUser := uuid.New(), uuid.New()
	if _, err := admin.Exec(ctx, `INSERT INTO companies (id, name) VALUES ($1, 'Autre')`, otherCompany); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO users (id, display_name) VALUES ($1, 'Autre')`, otherUser); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
		INSERT INTO organization_memberships (company_id, user_id, org_role, status) VALUES ($1,$2,'owner','active')
	`, otherCompany, otherUser); err != nil {
		t.Fatal(err)
	}
	other := api.WithPrincipal(ctx, tenant.Principal{UserID: otherUser, CompanyID: otherCompany})
	foreign := api.Execute(other, schema, `{ exchangeRates { rate } }`, nil)
	if len(foreign.Errors) > 0 {
		t.Fatal(foreign.Errors)
	}
	if len(foreign.Data.(map[string]any)["exchangeRates"].([]any)) != 0 {
		t.Fatal("other company saw the rate")
	}
	missing := api.Execute(other, schema, `query {
		convertAmount(amount: "1", fromCurrency: "USD", toCurrency: "CDF") { amount }
	}`, nil)
	if len(missing.Errors) == 0 {
		t.Fatal("conversion used another company's rate")
	}
}

func TestMediaUploadReceiptAndProductImage(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := seed(t, admin)
	ctx := context.Background()
	store := media.NewMemory()
	svc := commerce.New(app)
	svc.SetObjects(store)
	schema, err := api.Schema(api.Deps{Commerce: svc, ContextSecret: "test-secret-test-secret-test-secret"})
	if err != nil {
		t.Fatal(err)
	}
	owner := api.WithPrincipal(ctx, w.p)
	closed := commerce.New(app)
	closedSchema, err := api.Schema(api.Deps{Commerce: closed, ContextSecret: "test-secret-test-secret-test-secret"})
	if err != nil {
		t.Fatal(err)
	}
	refused := api.Execute(owner, closedSchema, `mutation ($input: RequestMediaUploadInput!) {
		requestMediaUpload(input: $input) { mediaId }
	}`, map[string]any{"input": map[string]any{"kind": "product_image", "contentType": "image/jpeg", "byteSize": "4"}})
	if len(refused.Errors) == 0 {
		t.Fatal("upload without storage succeeded")
	}

	request := `mutation ($input: RequestMediaUploadInput!) {
		requestMediaUpload(input: $input) { mediaId uploadUrl contentType }
	}`
	bad := api.Execute(owner, schema, request, map[string]any{"input": map[string]any{
		"kind": "product_image", "contentType": "application/pdf", "byteSize": "4", "fileName": "../note.pdf",
	}})
	if len(bad.Errors) == 0 {
		t.Fatal("pdf accepted as a product image")
	}
	image := api.Execute(owner, schema, request, map[string]any{"input": map[string]any{
		"kind": "product_image", "contentType": "image/jpeg", "byteSize": "4", "fileName": "huile.jpg",
	}})
	if len(image.Errors) > 0 {
		t.Fatal(image.Errors)
	}
	upload := image.Data.(map[string]any)["requestMediaUpload"].(map[string]any)
	mediaID := upload["mediaId"].(string)
	early := api.Execute(owner, schema, `mutation ($id: ID!) { confirmMediaUpload(id: $id) { status } }`, map[string]any{"id": mediaID})
	if len(early.Errors) == 0 {
		t.Fatal("confirm succeeded before the bytes arrived")
	}
	key := strings.TrimPrefix(upload["uploadUrl"].(string), "memory://put/")
	store.Put(key, "image/jpeg", []byte("jpeg"))
	confirmed := api.Execute(owner, schema, `mutation ($id: ID!) {
		confirmMediaUpload(id: $id) { status byteSize downloadUrl fileName }
	}`, map[string]any{"id": mediaID})
	if len(confirmed.Errors) > 0 {
		t.Fatal(confirmed.Errors)
	}
	view := confirmed.Data.(map[string]any)["confirmMediaUpload"].(map[string]any)
	if view["status"] != "ready" || view["byteSize"] != "4" || view["fileName"] != "huile.jpg" || !strings.HasPrefix(view["downloadUrl"].(string), "memory://get/") {
		t.Fatalf("%v", view)
	}
	linked := api.Execute(owner, schema, `mutation ($productId: ID!, $mediaId: ID!) {
		setProductImage(productId: $productId, mediaId: $mediaId)
	}`, map[string]any{"productId": w.product.String(), "mediaId": mediaID})
	if len(linked.Errors) > 0 {
		t.Fatal(linked.Errors)
	}
	shown := api.Execute(owner, schema, `query ($productId: ID!) { productMedia(productId: $productId) { id status } }`, map[string]any{"productId": w.product.String()})
	if len(shown.Errors) > 0 {
		t.Fatal(shown.Errors)
	}
	if shown.Data.(map[string]any)["productMedia"].(map[string]any)["id"] != mediaID {
		t.Fatal(shown.Data)
	}

	receipt := api.Execute(owner, schema, request, map[string]any{"input": map[string]any{
		"kind": "expense_receipt", "contentType": "application/pdf", "byteSize": "3",
	}})
	if len(receipt.Errors) > 0 {
		t.Fatal(receipt.Errors)
	}
	receiptUpload := receipt.Data.(map[string]any)["requestMediaUpload"].(map[string]any)
	receiptID := receiptUpload["mediaId"].(string)
	pendingExpense := api.Execute(owner, schema, `mutation ($input: CreateExpenseInput!) { createExpense(input: $input) { id } }`, map[string]any{"input": map[string]any{
		"storeId": w.store.String(), "category": "transport", "amount": "5.00", "currency": "USD",
		"paidFromCash": false, "mediaId": receiptID,
	}})
	if len(pendingExpense.Errors) == 0 {
		t.Fatal("expense accepted a receipt that was not uploaded")
	}
	store.Put(strings.TrimPrefix(receiptUpload["uploadUrl"].(string), "memory://put/"), "application/pdf", []byte("pdf"))
	if len(api.Execute(owner, schema, `mutation ($id: ID!) { confirmMediaUpload(id: $id) { status } }`, map[string]any{"id": receiptID}).Errors) > 0 {
		t.Fatal("receipt confirm failed")
	}
	created := api.Execute(owner, schema, `mutation ($input: CreateExpenseInput!) { createExpense(input: $input) { id } }`, map[string]any{"input": map[string]any{
		"storeId": w.store.String(), "category": "transport", "amount": "5.00", "currency": "USD",
		"paidFromCash": false, "mediaId": receiptID,
	}})
	if len(created.Errors) > 0 {
		t.Fatal(created.Errors)
	}
	expenseID := created.Data.(map[string]any)["createExpense"].(map[string]any)["id"].(string)
	var attached string
	if err := admin.QueryRow(ctx, `SELECT media_id::text FROM expenses WHERE id = $1`, expenseID).Scan(&attached); err != nil || attached != receiptID {
		t.Fatalf("attached=%s err=%v", attached, err)
	}

	otherCompany, otherUser := uuid.New(), uuid.New()
	if _, err := admin.Exec(ctx, `INSERT INTO companies (id, name) VALUES ($1, 'Autre media')`, otherCompany); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO users (id, display_name) VALUES ($1, 'Autre media')`, otherUser); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
		INSERT INTO organization_memberships (company_id, user_id, org_role, status) VALUES ($1,$2,'owner','active')
	`, otherCompany, otherUser); err != nil {
		t.Fatal(err)
	}
	other := api.WithPrincipal(ctx, tenant.Principal{UserID: otherUser, CompanyID: otherCompany})
	hidden := api.Execute(other, schema, `query ($id: ID!) { media(id: $id) { id } }`, map[string]any{"id": mediaID})
	if len(hidden.Errors) == 0 {
		t.Fatal("another company read the file")
	}
}
