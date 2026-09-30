package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JesusMaVe/api/internal/auth"
	"github.com/JesusMaVe/api/internal/auth/authtest"
	"github.com/JesusMaVe/api/internal/db"
	"github.com/JesusMaVe/api/internal/items"
	"github.com/JesusMaVe/api/internal/testdb"
)

// El requisito de la práctica: GET y POST de items solo funcionan con un Bearer válido.
func TestItemsFlowWithBearer(t *testing.T) {
	ctx := context.Background()
	pool, err := db.Connect(ctx, testdb.Start(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	keys := authtest.NewKeys(t)
	v, err := auth.NewVerifier(keys.PublicPEM, authtest.Issuer, authtest.Audience)
	if err != nil {
		t.Fatal(err)
	}
	h := New(Deps{
		DB: pool, Log: discard(), MaxBodyBytes: 1 << 16,
		Verifier: v, Items: items.NewPGRepository(pool),
		Limits: items.Limits{TitleMax: 120, DescriptionMax: 1000, PageLimit: 50},
	})
	bearer := "Bearer " + authtest.Sign(t, keys.Private, authtest.ValidClaims("alice"))
	do := func(method, body, authz string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/items", strings.NewReader(body))
		if authz != "" {
			req.Header.Set("Authorization", authz)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	if rec := do("GET", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET sin token = %d", rec.Code)
	}
	if rec := do("POST", `{"title":"Zelda"}`, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST sin token = %d", rec.Code)
	}
	rec := do("POST", `{"title":"Zelda","description":"BOTW"}`, bearer)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST con token = %d %s", rec.Code, rec.Body)
	}
	rec = do("GET", "", bearer)
	var body struct{ Items []items.Item }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("GET con token = %d %s", rec.Code, rec.Body)
	}
	if len(body.Items) != 1 || body.Items[0].Title != "Zelda" || body.Items[0].CreatedBy != "alice" {
		t.Fatalf("el item nuevo debe aparecer en el listado: %+v", body.Items)
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Error("las rutas de items también llevan los headers de seguridad")
	}
	big := `{"title":"` + strings.Repeat("a", 1<<16) + `"}`
	if rec := do("POST", big, bearer); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("body enorme = %d", rec.Code)
	}
}
