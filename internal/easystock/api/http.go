package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"rangoapp/internal/easystock/clerkauth"
	"rangoapp/internal/easystock/identity"
	"rangoapp/internal/easystock/tenant"

	"github.com/golang-jwt/jwt/v5"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/parser"
)

// HTTP serves POST /graphql. A missing or unverified Clerk session is rejected
// before the operation runs. The internal HMAC token is not accepted here.
// AllowIntrospection lets the playground load the schema without a session.
type HTTP struct {
	schema             graphql.Schema
	identity           *identity.Service
	keyfn              jwt.Keyfunc
	issuer             string
	parties            map[string]struct{}
	allowIntrospection bool
}

// SessionConfig holds the Clerk checks required before a GraphQL operation.
type SessionConfig struct {
	Identity *identity.Service
	Keyfunc  jwt.Keyfunc
	Issuer   string
	Parties  []string
	// AllowIntrospection accepts a query whose only root fields are __schema
	// and __type when no Clerk session is present. Set it with the playground.
	AllowIntrospection bool
}

func NewHTTP(schema graphql.Schema, session SessionConfig) *HTTP {
	parties := map[string]struct{}{}
	for _, party := range session.Parties {
		if party != "" {
			parties[party] = struct{}{}
		}
	}
	return &HTTP{
		schema:             schema,
		identity:           session.Identity,
		keyfn:              session.Keyfunc,
		issuer:             session.Issuer,
		parties:            parties,
		allowIntrospection: session.AllowIntrospection,
	}
}

func (h *HTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeGraphQLError(w, http.StatusMethodNotAllowed, "method")
		return
	}
	member, meta, ok := h.session(r)
	if !ok && !h.allowIntrospection {
		writeGraphQLError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		if !ok {
			writeGraphQLError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		writeGraphQLError(w, http.StatusBadRequest, "invalid body")
		return
	}
	ctx := r.Context()
	if ok {
		ctx = WithClerkMeta(WithPrincipal(ctx, member), meta)
	} else if !introspectionOnly(body.Query) {
		writeGraphQLError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	result := Execute(ctx, h.schema, body.Query, body.Variables)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func writeGraphQLError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"errors": []map[string]string{{"message": message}},
	})
}

func (h *HTTP) session(r *http.Request) (tenant.Principal, ClerkMeta, bool) {
	if h.identity == nil || h.keyfn == nil || h.issuer == "" {
		return tenant.Principal{}, ClerkMeta{}, false
	}
	header := r.Header.Get("Authorization")
	raw, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || raw == "" {
		return tenant.Principal{}, ClerkMeta{}, false
	}
	claims, err := clerkauth.Verify(raw, h.keyfn, h.issuer, h.parties, time.Now())
	if err != nil {
		return tenant.Principal{}, ClerkMeta{}, false
	}
	member, err := h.identity.ResolveSession(r.Context(), claims.Subject, claims.OrgID, r.Header.Get(CompanyHeader))
	if err != nil {
		return tenant.Principal{}, ClerkMeta{}, false
	}
	member.RequestID = r.Header.Get("X-Request-Id")
	return member, ClerkMeta{OrgID: claims.OrgID, OrgRole: claims.OrgRole}, true
}

// introspectionOnly reports whether query reads nothing but the GraphQL schema.
func introspectionOnly(query string) bool {
	doc, err := parser.Parse(parser.ParseParams{Source: query})
	if err != nil || doc == nil || len(doc.Definitions) == 0 {
		return false
	}
	sawQuery := false
	for _, def := range doc.Definitions {
		switch node := def.(type) {
		case *ast.FragmentDefinition:
		case *ast.OperationDefinition:
			if node.Operation != ast.OperationTypeQuery || node.SelectionSet == nil || len(node.SelectionSet.Selections) == 0 {
				return false
			}
			for _, sel := range node.SelectionSet.Selections {
				field, ok := sel.(*ast.Field)
				if !ok || field.Name == nil || (field.Name.Value != "__schema" && field.Name.Value != "__type") {
					return false
				}
			}
			sawQuery = true
		default:
			return false
		}
	}
	return sawQuery
}
