package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	commercev1 "rangoapp/internal/easystock/gen/commercev1"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type fakeSales struct {
	req *commercev1.ConfirmSaleRequest
	md  metadata.MD
}

func (f *fakeSales) ConfirmSale(ctx context.Context, in *commercev1.ConfirmSaleRequest, _ ...grpc.CallOption) (*commercev1.ConfirmSaleResponse, error) {
	f.req = in
	f.md, _ = metadata.FromOutgoingContext(ctx)
	return &commercev1.ConfirmSaleResponse{
		SaleId: "sale-1", Currency: "USD", NetAmount: "20.0000", PaidAmount: "20.0000", DueAmount: "0.0000",
	}, nil
}

func TestConfirmSaleSendsDecimalStringsAndContext(t *testing.T) {
	sales := &fakeSales{}
	secret := "test-secret-test-secret-test-secret"
	schema, err := Schema(Deps{Sales: sales, ContextSecret: secret})
	if err != nil {
		t.Fatal(err)
	}
	member := tenant.Principal{UserID: uuid.New(), CompanyID: uuid.New(), RequestID: "req-1"}
	storeID := uuid.New()
	productID := uuid.New()
	query := `mutation ($input: ConfirmSaleInput!) {
		confirmSale(input: $input) { saleId netAmount paidAmount dueAmount }
	}`
	vars := map[string]any{
		"input": map[string]any{
			"idempotencyKey": "sale-key",
			"storeId":        storeID.String(),
			"currency":       "USD",
			"lines": []any{map[string]any{
				"productId": productID.String(), "unitName": "piece", "quantity": "2", "expectedUnitPrice": "10.00",
			}},
			"payments": []any{map[string]any{"method": "cash", "amount": "20.00"}},
		},
	}
	result := Execute(WithPrincipal(context.Background(), member), schema, query, vars)
	if len(result.Errors) > 0 {
		t.Fatal(result.Errors)
	}
	if sales.req.GetIdempotencyKey() != "sale-key" || sales.req.GetLines()[0].GetQuantity() != "2" || sales.req.GetPayments()[0].GetAmount() != "20.00" {
		t.Fatalf("%+v", sales.req)
	}
	if len(sales.md.Get("x-easystock-context")) != 1 {
		t.Fatal("missing internal context")
	}
	data := result.Data.(map[string]any)["confirmSale"].(map[string]any)
	if data["saleId"] != "sale-1" || data["netAmount"] != "20.0000" {
		t.Fatalf("%v", data)
	}
}

func TestGraphQLRejectsInternalToken(t *testing.T) {
	schema, err := Schema(Deps{ContextSecret: "test-secret-test-secret-test-secret"})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHTTP(schema, SessionConfig{})
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":"{ subscription { status } }"}`))
	req.Header.Set("Authorization", "Bearer not-a-clerk-session")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"message":"unauthorized"`) {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestPlaygroundIntrospectionSkipsSession(t *testing.T) {
	schema, err := Schema(Deps{})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHTTP(schema, SessionConfig{AllowIntrospection: true})
	payload, err := json.Marshal(map[string]string{
		"query": `query IntrospectionQuery { __schema { queryType { name } types { ...T } } } fragment T on __Type { name }`,
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(string(payload)))
	req.Header.Set("Authorization", "Bearer ")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"Query"`) {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}

	denied := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":"{ __schema { queryType { name } } subscription { status } }"}`))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, denied)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("mixed query status %d body %s", rec.Code, rec.Body.String())
	}
}
