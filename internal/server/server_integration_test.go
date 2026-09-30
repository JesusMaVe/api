package server

import (
	"context"
	"encoding/json"
	"fmt"
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
	doAt := func(method, path, body, authz string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if authz != "" {
			req.Header.Set("Authorization", authz)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	do := func(method, body, authz string) *httptest.ResponseRecorder {
		return doAt(method, "/api/items", body, authz)
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

	// PUT y DELETE: con Bearer y solo el dueño.
	path := fmt.Sprintf("/api/items/%d", body.Items[0].ID)
	bob := "Bearer " + authtest.Sign(t, keys.Private, authtest.ValidClaims("bob"))
	if rec := doAt("PUT", path, `{"title":"x"}`, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("PUT sin token = %d", rec.Code)
	}
	if rec := doAt("DELETE", path, "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("DELETE sin token = %d", rec.Code)
	}
	if rec := doAt("PUT", path, `{"title":"hackeado"}`, bob); rec.Code != http.StatusForbidden {
		t.Fatalf("PUT de otro usuario = %d", rec.Code)
	}
	if rec := doAt("DELETE", path, "", bob); rec.Code != http.StatusForbidden {
		t.Fatalf("DELETE de otro usuario = %d", rec.Code)
	}
	if rec := doAt("PUT", path, `{"title":"Zelda TOTK","description":"secuela"}`, bearer); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"title":"Zelda TOTK"`) {
		t.Fatalf("PUT del dueño = %d %s", rec.Code, rec.Body)
	}
	if rec := doAt("DELETE", path, "", bearer); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE del dueño = %d", rec.Code)
	}
	if rec := doAt("DELETE", path, "", bearer); rec.Code != http.StatusNotFound {
		t.Fatalf("DELETE repetido = %d", rec.Code)
	}
}
