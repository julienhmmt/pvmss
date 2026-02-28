package apiv1

import (
	"context"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/julienschmidt/httprouter"

	"pvmss/state"
)

// JWTClaims are the custom claims embedded in every access token.
type JWTClaims struct {
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
	jwt.RegisteredClaims
}

// JWTMiddleware validates the access_token cookie and injects username + is_admin into context.
// It reads the JWT secret from sm.GetSettings().JWTSecret (settings.json).
func JWTMiddleware(sm state.StateManager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret := sm.GetSettings().JWTSecret
		if secret == "" {
			errNotConfigured(w)
			return
		}

		cookie, err := r.Cookie("access_token")
		if err != nil {
			errUnauthorized(w)
			return
		}

		claims := &JWTClaims{}
		token, err := jwt.ParseWithClaims(cookie.Value, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			errUnauthorized(w)
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyUsername, claims.Username)
		ctx = context.WithValue(ctx, contextKeyIsAdmin, claims.IsAdmin)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// JWTAdminMiddleware wraps JWTMiddleware and additionally requires is_admin=true.
func JWTAdminMiddleware(sm state.StateManager, next http.Handler) http.Handler {
	return JWTMiddleware(sm, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !r.Context().Value(contextKeyIsAdmin).(bool) {
			errForbidden(w)
			return
		}
		next.ServeHTTP(w, r)
	}))
}

// httprouterWrap adapts an http.Handler to httprouter.Handle, injecting router params into context.
func httprouterWrap(h http.Handler) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		ctx := context.WithValue(r.Context(), httprouter.ParamsKey, ps)
		h.ServeHTTP(w, r.WithContext(ctx))
	}
}

// usernameFromCtx extracts username from the request context (set by JWTMiddleware).
func usernameFromCtx(r *http.Request) string {
	v, _ := r.Context().Value(contextKeyUsername).(string)
	return v
}

// isAdminFromCtx extracts the is_admin flag from the request context.
func isAdminFromCtx(r *http.Request) bool {
	v, _ := r.Context().Value(contextKeyIsAdmin).(bool)
	return v
}
