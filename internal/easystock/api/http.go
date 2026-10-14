package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"rangoapp/internal/easystock/authn"
	"rangoapp/internal/easystock/tenant"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/parser"
)

const (
	refreshCookieName = "easystock_refresh"
	csrfCookieName    = "easystock_csrf"
	csrfHeader        = "X-EasyStock-CSRF"
)

// HTTP serves POST /graphql. Business calls need a verified access token.
// The refresh token for web clients stays in an HttpOnly cookie.
type HTTP struct {
	schema             graphql.Schema
	auth               *authn.Service
	allowIntrospection bool
	origins            map[string]struct{}
	cookieSecure       bool
}

// SessionConfig holds the checks required before a GraphQL operation.
type SessionConfig struct {
	Auth               *authn.Service
	AllowIntrospection bool
	CORSOrigins        []string
	CookieSecure       bool
}

type httpMetaKey struct{}

type httpMeta struct {
	IP            string
	UserAgent     string
	RefreshCookie string
	CSRFCookie    string
	CSRFHeader    string
}

type sinkKey struct{}

// ResponseSink collects cookies set by a GraphQL operation.
type ResponseSink struct {
	Cookies []*http.Cookie
}

func NewHTTP(schema graphql.Schema, session SessionConfig) *HTTP {
	origins := map[string]struct{}{}
	for _, origin := range session.CORSOrigins {
		if origin != "" {
			origins[origin] = struct{}{}
		}
	}
	return &HTTP{
		schema:             schema,
		auth:               session.Auth,
		allowIntrospection: session.AllowIntrospection,
		origins:            origins,
		cookieSecure:       session.CookieSecure,
	}
}

func (h *HTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.applyCORS(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeGraphQLError(w, http.StatusMethodNotAllowed, "method")
		return
	}
	var body struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeGraphQLError(w, http.StatusBadRequest, "invalid body")
		return
	}
	fields := rootFields(body.Query)
	public := len(fields) > 0 && only(fields, publicOperations)
	member, ok := h.session(r)
	if !ok && !public && !(h.allowIntrospection && introspectionOnly(body.Query)) {
		writeGraphQLError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if ok && member.Restricted() && !only(fields, passwordChangeOperations) {
		writeCodedError(w, http.StatusForbidden, "Changez le mot de passe temporaire avant de continuer.", "MUST_CHANGE_PASSWORD")
		return
	}
	ctx := r.Context()
	meta := httpMeta{
		IP: clientIP(r), UserAgent: r.UserAgent(), CSRFHeader: r.Header.Get(csrfHeader),
	}
	if cookie, err := r.Cookie(refreshCookieName); err == nil {
		meta.RefreshCookie = cookie.Value
	}
	if cookie, err := r.Cookie(csrfCookieName); err == nil {
		meta.CSRFCookie = cookie.Value
	}
	ctx = contextWith(ctx, meta)
	sink := &ResponseSink{}
	ctx = context.WithValue(ctx, sinkKey{}, sink)
	if ok {
		member.RequestID = r.Header.Get("X-Request-Id")
		ctx = WithPrincipal(ctx, member)
	}
	result := Execute(ctx, h.schema, body.Query, body.Variables)
	for _, cookie := range sink.Cookies {
		if cookie.MaxAge >= 0 {
			cookie.Secure = h.cookieSecure
			if h.cookieSecure {
				cookie.SameSite = http.SameSiteNoneMode
			}
		}
		http.SetCookie(w, cookie)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (h *HTTP) applyCORS(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin != "" {
		if _, ok := h.origins[origin]; ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-EasyStock-CSRF, X-Request-Id, X-EasyStock-Company")
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		}
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}

func (h *HTTP) session(r *http.Request) (tenant.Principal, bool) {
	if h.auth == nil {
		return tenant.Principal{}, false
	}
	raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || raw == "" {
		return tenant.Principal{}, false
	}
	member, err := h.auth.Principal(r.Context(), raw)
	if err != nil {
		return tenant.Principal{}, false
	}
	return member, true
}

func contextWith(ctx context.Context, meta httpMeta) context.Context {
	return context.WithValue(ctx, httpMetaKey{}, meta)
}

func rememberWebSession(ctx context.Context, result authn.SessionResult) {
	sink, _ := ctx.Value(sinkKey{}).(*ResponseSink)
	if sink == nil {
		return
	}
	secure := false
	maxAge := 90 * 24 * 3600
	sink.Cookies = append(sink.Cookies,
		&http.Cookie{
			Name: refreshCookieName, Value: result.RefreshToken, Path: "/", HttpOnly: true,
			Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: maxAge,
		},
		&http.Cookie{
			Name: csrfCookieName, Value: result.CSRFToken, Path: "/", HttpOnly: false,
			Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: maxAge,
		},
	)
}

func clearSessionCookies(ctx context.Context) {
	sink, _ := ctx.Value(sinkKey{}).(*ResponseSink)
	if sink == nil {
		return
	}
	expired := time.Unix(0, 0)
	sink.Cookies = append(sink.Cookies,
		&http.Cookie{Name: refreshCookieName, Value: "", Path: "/", HttpOnly: true, MaxAge: -1, Expires: expired},
		&http.Cookie{Name: csrfCookieName, Value: "", Path: "/", HttpOnly: false, MaxAge: -1, Expires: expired},
	)
}

func writeGraphQLError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"errors": []map[string]string{{"message": message}},
	})
}

func writeCodedError(w http.ResponseWriter, status int, message, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"errors": []map[string]any{{"message": message, "extensions": map[string]string{"code": code}}},
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

var publicOperations = map[string]struct{}{
	"requestOwnerSignupOtp": {}, "verifyOwnerSignupOtp": {}, "completeOwnerSignup": {},
	"login": {}, "refreshSession": {}, "logout": {}, "logoutAllSessions": {},
	"requestPasswordResetOtp": {}, "verifyPasswordResetOtp": {}, "resetPassword": {},
}

var passwordChangeOperations = map[string]struct{}{
	"changePassword": {}, "logout": {}, "logoutAllSessions": {}, "me": {},
}

func only(fields []string, allowed map[string]struct{}) bool {
	if len(fields) == 0 {
		return false
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return false
		}
	}
	return true
}

func rootFields(query string) []string {
	doc, err := parser.Parse(parser.ParseParams{Source: query})
	if err != nil || doc == nil {
		return nil
	}
	var names []string
	for _, def := range doc.Definitions {
		op, ok := def.(*ast.OperationDefinition)
		if !ok || op.SelectionSet == nil {
			continue
		}
		for _, sel := range op.SelectionSet.Selections {
			field, ok := sel.(*ast.Field)
			if ok && field.Name != nil {
				names = append(names, field.Name.Value)
			}
		}
	}
	return names
}

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
