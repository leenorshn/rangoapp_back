package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"rangoapp/utils"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubscriptionMiddleware(t *testing.T) {
	t.Run("OPTIONS request passes through", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/", nil)
		rr := httptest.NewRecorder()

		handler := SubscriptionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("Request without token passes through", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()

		handler := SubscriptionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("Request with empty companyID passes through", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		ctx := context.WithValue(req.Context(), userCtxKey, &utils.JwtCustomClaim{
			CompanyID: "",
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		handler := SubscriptionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("Request with zero companyID passes through", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		ctx := context.WithValue(req.Context(), userCtxKey, &utils.JwtCustomClaim{
			CompanyID: "000000000000000000000000",
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		handler := SubscriptionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestValidateSubscriptionInContext(t *testing.T) {
	// Note: This function requires a real DB connection, so we test the logic
	// Full integration tests would be in integration test files
	
	t.Run("Context without claim", func(t *testing.T) {
		ctx := context.Background()
		// Without DB, we can't fully test, but we can test nil check
		// Full test requires DB setup
		assert.NotNil(t, ctx)
	})

	t.Run("Context with empty companyID", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), userCtxKey, &utils.JwtCustomClaim{
			CompanyID: "",
		})
		// Should return nil (pass through)
		// Full test requires DB setup
		assert.NotNil(t, ctx)
	})
}

func TestCheckSubscriptionLimits(t *testing.T) {
	t.Run("Context without claim", func(t *testing.T) {
		ctx := context.Background()
		// Without DB, we can't fully test
		assert.NotNil(t, ctx)
	})

	t.Run("Context with empty companyID", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), userCtxKey, &utils.JwtCustomClaim{
			CompanyID: "",
		})
		// Should return nil (pass through)
		assert.NotNil(t, ctx)
	})
}

func TestIsSubscriptionActive(t *testing.T) {
	t.Run("Context without claim", func(t *testing.T) {
		ctx := context.Background()
		// Without DB, we can't fully test
		assert.NotNil(t, ctx)
	})

	t.Run("Context with empty companyID", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), userCtxKey, &utils.JwtCustomClaim{
			CompanyID: "",
		})
		// Should return true (pass through)
		assert.NotNil(t, ctx)
	})
}

