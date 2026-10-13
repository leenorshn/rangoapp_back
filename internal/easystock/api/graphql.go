package api

import (
	"context"
	"time"

	"rangoapp/internal/easystock/catalog"
	"rangoapp/internal/easystock/commerce"
	"rangoapp/internal/easystock/contexttoken"
	commercev1 "rangoapp/internal/easystock/gen/commercev1"
	subscriptionv1 "rangoapp/internal/easystock/gen/subscriptionv1"
	"rangoapp/internal/easystock/identity"
	"rangoapp/internal/easystock/readmodel"
	"rangoapp/internal/easystock/reports"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/graphql-go/graphql"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type principalKey struct{}

// WithPrincipal attaches the member resolved from a verified Clerk session.
func WithPrincipal(ctx context.Context, p tenant.Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

func principal(ctx context.Context) (tenant.Principal, bool) {
	p, ok := sessionUser(ctx)
	return p, ok && p.CompanyID != uuid.Nil
}

func sessionUser(ctx context.Context) (tenant.Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(tenant.Principal)
	return p, ok && p.UserID != uuid.Nil
}

type clerkKey struct{}

// ClerkMeta carries the organization claim from the verified Clerk session.
type ClerkMeta struct {
	OrgID   string
	OrgRole string
}

// CompanyHeader selects a company that is not linked to a Clerk organization yet.
const CompanyHeader = "X-EasyStock-Company"

// WithClerkMeta attaches the verified Clerk organization claim.
func WithClerkMeta(ctx context.Context, meta ClerkMeta) context.Context {
	return context.WithValue(ctx, clerkKey{}, meta)
}

func clerkMeta(ctx context.Context) ClerkMeta {
	meta, _ := ctx.Value(clerkKey{}).(ClerkMeta)
	return meta
}

// SaleClient is the Commerce gRPC method used by confirmSale.
type SaleClient interface {
	ConfirmSale(ctx context.Context, in *commercev1.ConfirmSaleRequest, opts ...grpc.CallOption) (*commercev1.ConfirmSaleResponse, error)
}

// PlanClient is the subscription gRPC method used by the subscription query.
type PlanClient interface {
	GetSubscription(ctx context.Context, in *subscriptionv1.GetSubscriptionRequest, opts ...grpc.CallOption) (*subscriptionv1.GetSubscriptionResponse, error)
}

// CatalogFunc reads one page of catalog changes for the caller's company.
type CatalogFunc func(ctx context.Context, p tenant.Principal, after time.Time, afterID uuid.UUID, limit int) ([]catalog.Change, error)

// Deps wires GraphQL to the internal services. Reads stay in this process.
// Sale confirmation still goes through Commerce gRPC.
type Deps struct {
	Sales         SaleClient
	Plans         PlanClient
	Catalog       CatalogFunc
	Identity      *identity.Service
	Commerce      *commerce.Service
	Reads         *readmodel.Service
	Reports       *reports.Service
	ContextSecret string
}

// Schema builds the EasyStock GraphQL API. Amounts are decimal strings.
func Schema(deps Deps) (graphql.Schema, error) {
	saleResult := graphql.NewObject(graphql.ObjectConfig{
		Name: "SaleResult",
		Fields: graphql.Fields{
			"saleId":     &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"currency":   &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"netAmount":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"paidAmount": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"dueAmount":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"debtId":     &graphql.Field{Type: graphql.ID},
		},
	})
	change := graphql.NewObject(graphql.ObjectConfig{
		Name: "CatalogChange",
		Fields: graphql.Fields{
			"id":        &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"sku":       &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"name":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"baseUnit":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"status":    &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"updatedAt": &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
			"archived":  &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
		},
	})
	subscription := graphql.NewObject(graphql.ObjectConfig{
		Name: "Subscription",
		Fields: graphql.Fields{
			"planCode": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"status":   &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"source":   &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	lineInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "SaleLineInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"productId":         &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"unitName":          &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"quantity":          &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"expectedUnitPrice": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	paymentInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "PaymentInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"method":            &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"amount":            &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"externalReference": &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	})
	saleInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "ConfirmSaleInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"idempotencyKey": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"storeId":        &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"customerId":     &graphql.InputObjectFieldConfig{Type: graphql.ID},
			"currency":       &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"lines":          &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(lineInput)))},
			"payments":       &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(paymentInput)))},
		},
	})
	queryFields := graphql.Fields{
		"subscription": &graphql.Field{
			Type: graphql.NewNonNull(subscription),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				_, ctx, err := signed(p.Context, deps.ContextSecret)
				if err != nil {
					return nil, err
				}
				if deps.Plans == nil {
					return nil, errUnavailable("Abonnements indisponibles.")
				}
				view, err := deps.Plans.GetSubscription(ctx, &subscriptionv1.GetSubscriptionRequest{})
				if err != nil {
					return nil, err
				}
				return map[string]any{"planCode": view.GetPlanCode(), "status": view.GetStatus(), "source": view.GetSource()}, nil
			},
		},
		"catalogChanges": &graphql.Field{
			Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(change))),
			Args: graphql.FieldConfigArgument{
				"after":   &graphql.ArgumentConfig{Type: graphql.DateTime},
				"afterId": &graphql.ArgumentConfig{Type: graphql.ID},
				"limit":   &graphql.ArgumentConfig{Type: graphql.Int},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, ok := sessionUser(p.Context)
				if !ok {
					return nil, errUnavailable("Authentification requise.")
				}
				if member.CompanyID == uuid.Nil {
					return nil, errUnavailable("Créez ou sélectionnez une entreprise pour continuer.")
				}
				if deps.Catalog == nil {
					return nil, errUnavailable("Catalogue indisponible.")
				}
				after, _ := p.Args["after"].(time.Time)
				afterID := uuid.Nil
				if raw, _ := p.Args["afterId"].(string); raw != "" {
					parsed, err := uuid.Parse(raw)
					if err != nil {
						return nil, errUnavailable("Curseur invalide.")
					}
					afterID = parsed
				}
				limit := 100
				switch value := p.Args["limit"].(type) {
				case int:
					limit = value
				case int32:
					limit = int(value)
				case float64:
					limit = int(value)
				}
				rows, err := deps.Catalog(p.Context, member, after, afterID, limit)
				if err != nil {
					return nil, err
				}
				out := make([]map[string]any, 0, len(rows))
				for _, row := range rows {
					out = append(out, map[string]any{
						"id": row.ID.String(), "sku": row.SKU, "name": row.Name, "baseUnit": row.BaseUnit,
						"status": row.Status, "updatedAt": row.UpdatedAt, "archived": row.Archived,
					})
				}
				return out, nil
			},
		},
	}
	orgQueries, orgMutations := orgAPI(deps)
	for name, field := range orgQueries {
		queryFields[name] = field
	}
	fxQueries, fxMutations := fxAPI(deps)
	for name, field := range fxQueries {
		queryFields[name] = field
	}
	for name, field := range readFields(deps) {
		queryFields[name] = field
	}
	query := graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: queryFields})
	mutationFields := graphql.Fields{
		"confirmSale": &graphql.Field{
			Type: graphql.NewNonNull(saleResult),
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(saleInput)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				_, ctx, err := signed(p.Context, deps.ContextSecret)
				if err != nil {
					return nil, err
				}
				if deps.Sales == nil {
					return nil, errUnavailable("Commerce indisponible.")
				}
				req, err := saleRequest(p.Args["input"])
				if err != nil {
					return nil, err
				}
				out, err := deps.Sales.ConfirmSale(ctx, req)
				if err != nil {
					return nil, err
				}
				result := map[string]any{
					"saleId": out.GetSaleId(), "currency": out.GetCurrency(),
					"netAmount": out.GetNetAmount(), "paidAmount": out.GetPaidAmount(), "dueAmount": out.GetDueAmount(),
				}
				if out.GetDebtId() != "" {
					result["debtId"] = out.GetDebtId()
				}
				return result, nil
			},
		},
	}
	for name, field := range orgMutations {
		mutationFields[name] = field
	}
	for name, field := range fxMutations {
		mutationFields[name] = field
	}
	mutation := graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: mutationFields})
	return graphql.NewSchema(graphql.SchemaConfig{Query: query, Mutation: mutation})
}

func signed(ctx context.Context, secret string) (tenant.Principal, context.Context, error) {
	member, ok := sessionUser(ctx)
	if !ok {
		return tenant.Principal{}, ctx, errUnavailable("Authentification requise.")
	}
	if member.CompanyID == uuid.Nil {
		return tenant.Principal{}, ctx, errUnavailable("Créez ou sélectionnez une entreprise pour continuer.")
	}
	if len(secret) < 32 {
		return tenant.Principal{}, ctx, errUnavailable("Commerce indisponible.")
	}
	token, err := contexttoken.Sign(secret, member, 30*time.Second)
	if err != nil {
		return tenant.Principal{}, ctx, err
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "x-easystock-context", token)
	return member, ctx, nil
}

func saleRequest(raw any) (*commercev1.ConfirmSaleRequest, error) {
	input, ok := raw.(map[string]any)
	if !ok {
		return nil, errUnavailable("Vente illisible.")
	}
	req := &commercev1.ConfirmSaleRequest{
		IdempotencyKey: str(input["idempotencyKey"]),
		StoreId:        str(input["storeId"]),
		CustomerId:     str(input["customerId"]),
		Currency:       str(input["currency"]),
	}
	for _, item := range list(input["lines"]) {
		line, ok := item.(map[string]any)
		if !ok {
			return nil, errUnavailable("Ligne de vente illisible.")
		}
		req.Lines = append(req.Lines, &commercev1.SaleLine{
			ProductId: str(line["productId"]), UnitName: str(line["unitName"]),
			Quantity: str(line["quantity"]), ExpectedUnitPrice: str(line["expectedUnitPrice"]),
		})
	}
	for _, item := range list(input["payments"]) {
		payment, ok := item.(map[string]any)
		if !ok {
			return nil, errUnavailable("Paiement illisible.")
		}
		req.Payments = append(req.Payments, &commercev1.Payment{
			Method: str(payment["method"]), Amount: str(payment["amount"]), ExternalReference: str(payment["externalReference"]),
		})
	}
	if req.IdempotencyKey == "" || len(req.Lines) == 0 {
		return nil, errUnavailable("La vente doit avoir une clé d'idempotence et au moins une ligne.")
	}
	return req, nil
}

func str(value any) string {
	text, _ := value.(string)
	return text
}

func list(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []map[string]any:
		out := make([]any, len(typed))
		for i := range typed {
			out[i] = typed[i]
		}
		return out
	default:
		return nil
	}
}

type unavailable string

func (e unavailable) Error() string { return string(e) }

func errUnavailable(message string) error { return unavailable(message) }

func requireCompany(ctx context.Context) (tenant.Principal, error) {
	member, ok := sessionUser(ctx)
	if !ok {
		return tenant.Principal{}, errUnavailable("Authentification requise.")
	}
	if member.CompanyID == uuid.Nil {
		return tenant.Principal{}, errUnavailable("Créez ou sélectionnez une entreprise pour continuer.")
	}
	return member, nil
}

func argUUID(value any) (uuid.UUID, error) {
	text, _ := value.(string)
	if text == "" {
		return uuid.Nil, nil
	}
	id, err := uuid.Parse(text)
	if err != nil {
		return uuid.Nil, errUnavailable("Identifiant invalide.")
	}
	return id, nil
}

func argLimit(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func argTime(value any) (time.Time, bool) {
	switch typed := value.(type) {
	case time.Time:
		return typed, !typed.IsZero()
	case string:
		parsed, err := time.Parse(time.RFC3339, typed)
		return parsed, err == nil
	default:
		return time.Time{}, false
	}
}

func pageFrom(args map[string]any) (readmodel.Page, error) {
	after, _ := argTime(args["after"])
	afterID, err := argUUID(args["afterId"])
	if err != nil {
		return readmodel.Page{}, err
	}
	return readmodel.Page{After: after, AfterID: afterID, Limit: argLimit(args["limit"])}, nil
}

// Execute runs one GraphQL operation. Callers must already have authenticated the member.
func Execute(ctx context.Context, schema graphql.Schema, query string, vars map[string]any) *graphql.Result {
	return graphql.Do(graphql.Params{Schema: schema, RequestString: query, VariableValues: vars, Context: ctx})
}
