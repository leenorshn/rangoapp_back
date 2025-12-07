package middlewares

import (
	"context"
	"net/http"
	"strings"

	"rangoapp/utils"
)

var userCtxKey = &contextKey{"authUser"}

type contextKey struct {
	name string
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication for OPTIONS requests (CORS preflight)
		if r.Method == "OPTIONS" {
			next.ServeHTTP(w, r)
			return
		}

		auth := r.Header.Get("Authorization")

		if auth == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Safely check for Bearer prefix using strings.HasPrefix (prevents panic)
		const bearerPrefix = "Bearer "
		
		// Use strings.HasPrefix for safe prefix checking (no panic risk)
		if !strings.HasPrefix(auth, bearerPrefix) {
			utils.Debug("Authorization header does not start with 'Bearer ' prefix")
			next.ServeHTTP(w, r)
			return
		}

		// Use strings.TrimPrefix to safely extract token (handles edge cases)
		token := strings.TrimPrefix(auth, bearerPrefix)
		
		// Trim any leading/trailing whitespace from token
		token = strings.TrimSpace(token)
		
		// Ensure token is not empty after trimming
		if token == "" {
			utils.Warning("Empty token after Bearer prefix")
			http.Error(w, "Invalid token: token is empty", http.StatusForbidden)
			return
		}

		validate, err := utils.JwtValidate(context.Background(), token)
		if err != nil || !validate.Valid {
			utils.LogError(err, "Invalid JWT token")
			http.Error(w, "Invalid token", http.StatusForbidden)
			return
		}

		customClaim, ok := validate.Claims.(*utils.JwtCustomClaim)
		if !ok || customClaim == nil {
			utils.Warning("Invalid token claims type assertion")
			http.Error(w, "Invalid token claims", http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), userCtxKey, customClaim)

		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

func CtxValue(ctx context.Context) *utils.JwtCustomClaim {
	raw, _ := ctx.Value(userCtxKey).(*utils.JwtCustomClaim)
	return raw
}
