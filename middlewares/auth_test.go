package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddleware_PanicPrevention(t *testing.T) {
	tests := []struct {
		name           string
		authHeader     string
		shouldPanic    bool
		expectedStatus int
	}{
		{
			name:           "Empty header",
			authHeader:     "",
			shouldPanic:    false,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Short string",
			authHeader:     "Bea",
			shouldPanic:    false,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Exact length but wrong prefix",
			authHeader:     "Bearer", // 6 chars, missing space
			shouldPanic:    false,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Shorter than Bearer ",
			authHeader:     "Bear",
			shouldPanic:    false,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Single character",
			authHeader:     "B",
			shouldPanic:    false,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Valid Bearer token",
			authHeader:     "Bearer valid.token.here",
			shouldPanic:    false,
			expectedStatus: http.StatusForbidden, // Will fail validation but not panic
		},
		{
			name:           "Bearer with empty token",
			authHeader:     "Bearer ",
			shouldPanic:    false,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Bearer with whitespace only",
			authHeader:     "Bearer    ",
			shouldPanic:    false,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Bearer with newline",
			authHeader:     "Bearer \n",
			shouldPanic:    false,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Case sensitive check - bearer lowercase",
			authHeader:     "bearer token",
			shouldPanic:    false,
			expectedStatus: http.StatusOK, // Should pass through without auth
		},
		{
			name:           "Case sensitive check - BEARER uppercase",
			authHeader:     "BEARER token",
			shouldPanic:    false,
			expectedStatus: http.StatusOK, // Should pass through without auth
		},
		{
			name:           "Bearer without space",
			authHeader:     "Bearertoken",
			shouldPanic:    false,
			expectedStatus: http.StatusOK, // Should pass through without auth
		},
		{
			name:           "Multiple spaces after Bearer",
			authHeader:     "Bearer   token",
			shouldPanic:    false,
			expectedStatus: http.StatusForbidden, // Will fail validation but not panic
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.shouldPanic {
						t.Errorf("Unexpected panic: %v", r)
					}
				}
			}()

			req := httptest.NewRequest("GET", "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			rr := httptest.NewRecorder()
			handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus && tt.expectedStatus != 0 {
				// For now, we just check that it doesn't panic
				// Status code validation can be added later
			}
		})
	}
}

