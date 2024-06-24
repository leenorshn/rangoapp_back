package middlewares

import (
	"context"
	"net/http"

	"rangoapp/utils"
)

var userCtxKey = &contextKey{"authUser"}

type contextKey struct {
	name string
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")

		if auth == "" {
			next.ServeHTTP(w, r)
			return
		}

		bearer := "Bearer "
		auth = auth[len(bearer):]

		validate, err := utils.JwtValidate(context.Background(), auth)
		if err != nil || !validate.Valid {
			http.Error(w, "Invalid token", http.StatusForbidden)
			return
		}

		customClaim, _ := validate.Claims.(*utils.JwtCustomClaim)

		ctx := context.WithValue(r.Context(), userCtxKey, customClaim)

		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

func CtxValue(ctx context.Context) *utils.JwtCustomClaim {
	raw, _ := ctx.Value(userCtxKey).(*utils.JwtCustomClaim)
	//fmt.Println(raw.ID)
	return raw
}
