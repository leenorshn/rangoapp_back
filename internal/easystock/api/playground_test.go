package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlaygroundTargetsGraphQL(t *testing.T) {
	rec := httptest.NewRecorder()
	Playground("EasyStock", "/graphql").ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/playground", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("content type %q", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"endpoint":"/graphql"`) || !strings.Contains(body, `"Authorization":"Bearer "`) {
		t.Fatalf("playground config missing: %s", body)
	}
}
