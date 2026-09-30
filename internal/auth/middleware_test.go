package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JesusMaVe/api/internal/auth/authtest"
)

func protected(t *testing.T) (http.Handler, authtest.Keys, *User) {
	t.Helper()
	keys := authtest.NewKeys(t)
	seen := &User{}
	h := RequireBearer(newVerifier(t, keys), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFrom(r.Context())
		if !ok {
			t.Error("el handler protegido no ve al usuario")
		}
		*seen = u
		w.WriteHeader(http.StatusNoContent)
	}))
	return h, keys, seen
}

func call(h http.Handler, authz string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/api/items", nil)
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRequireBearerValid(t *testing.T) {
	h, keys, seen := protected(t)
	tok := authtest.Sign(t, keys.Private, authtest.ValidClaims("alice"))
	for _, scheme := range []string{"Bearer", "bearer", "BEARER"} {
		t.Run(scheme, func(t *testing.T) {
			if rec := call(h, scheme+" "+tok); rec.Code != http.StatusNoContent {
				t.Fatalf("status = %d", rec.Code)
			}
			if seen.Subject != "alice" {
				t.Fatalf("usuario en el contexto: %+v", *seen)
			}
		})
	}
}

func TestRequireBearerRejects(t *testing.T) {
	h, keys, _ := protected(t)
	expired := authtest.ValidClaims("alice")
	expired["exp"] = time.Now().Add(-time.Minute).Unix()
	cases := map[string]string{
		"sin header":     "",
		"Basic":          "Basic YWxpY2U6eA==",
		"Bearer vacío":   "Bearer ",
		"solo Bearer":    "Bearer",
		"token inválido": "Bearer no.es.valido",
		"expirado":       "Bearer " + authtest.Sign(t, keys.Private, expired),
	}
	for name, authz := range cases {
		t.Run(name, func(t *testing.T) {
			rec := call(h, authz)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d", rec.Code)
			}
			if rec.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Errorf("WWW-Authenticate = %q", rec.Header().Get("WWW-Authenticate"))
			}
			if rec.Body.String() != "{\"error\":\"unauthorized\"}\n" {
				t.Errorf("body = %q", rec.Body)
			}
		})
	}
}

func TestUserFromEmptyContext(t *testing.T) {
	if _, ok := UserFrom(httptest.NewRequest("GET", "/", nil).Context()); ok {
		t.Fatal("sin RequireBearer no debe haber usuario")
	}
}
