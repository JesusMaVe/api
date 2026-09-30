package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/JesusMaVe/api/internal/httpx"
)

type TokenVerifier interface {
	Verify(token string) (User, error)
}

type userKey struct{}

func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, userKey{}, u)
}

func UserFrom(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userKey{}).(User)
	return u, ok
}

// RequireBearer exige Authorization: Bearer <jwt> válido (el esquema no distingue mayúsculas,
// RFC 6750) y pone el usuario en el contexto. Cualquier fallo da el mismo 401 genérico.
func RequireBearer(v TokenVerifier, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
		token = strings.TrimSpace(token)
		if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
			unauthorized(w)
			return
		}
		u, err := v.Verify(token)
		if err != nil {
			unauthorized(w)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), u)))
	})
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	httpx.Error(w, http.StatusUnauthorized, "unauthorized")
}
