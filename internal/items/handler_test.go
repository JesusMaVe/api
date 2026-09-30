package items

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JesusMaVe/api/internal/auth"
)

type fakeRepo struct {
	items     []Item
	err       error
	created   NewItem
	listLimit int
}

func (f *fakeRepo) List(_ context.Context, limit int) ([]Item, error) {
	f.listLimit = limit
	return f.items, f.err
}

func (f *fakeRepo) Create(_ context.Context, in NewItem) (Item, error) {
	f.created = in
	if f.err != nil {
		return Item{}, f.err
	}
	return Item{ID: 7, Title: in.Title, Description: in.Description, CreatedBy: in.CreatedBy, CreatedAt: time.Unix(0, 0).UTC()}, nil
}

var handlerLimits = Limits{TitleMax: 20, DescriptionMax: 50, PageLimit: 3}

func do(t *testing.T, repo *fakeRepo, logs *bytes.Buffer, method, body string, withUser bool) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(repo, handlerLimits, slog.New(slog.NewJSONHandler(logs, nil)))
	req := httptest.NewRequest(method, "/api/items", strings.NewReader(body))
	if withUser {
		req = req.WithContext(auth.WithUser(req.Context(), auth.User{Subject: "alice"}))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCreate(t *testing.T) {
	repo, logs := &fakeRepo{}, &bytes.Buffer{}
	rec := do(t, repo, logs, "POST", `{"title":"  Zelda  ","description":"BOTW"}`, true)
	if rec.Code != 201 || !strings.Contains(rec.Body.String(), `"created_by":"alice"`) || !strings.Contains(rec.Body.String(), `"id":7`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	if repo.created != (NewItem{Title: "Zelda", Description: "BOTW", CreatedBy: "alice"}) {
		t.Fatalf("el repo recibió %+v (título recortado y created_by del token)", repo.created)
	}
	if !strings.Contains(logs.String(), `"user":"alice"`) || strings.Contains(logs.String(), "Zelda") {
		t.Fatalf("el log de auditoría lleva usuario e id, no el contenido: %s", logs)
	}
}

func TestCreateRejects(t *testing.T) {
	cases := []struct {
		name, body string
		status     int
		want       string
	}{
		{"created_by en el body no se acepta", `{"title":"x","created_by":"bob"}`, 400, `"invalid request"`},
		{"JSON malformado", `{"title":`, 400, `"invalid request"`},
		{"título vacío", `{"title":"  "}`, 400, `"fields":{"title"`},
		{"título largo", `{"title":"` + strings.Repeat("a", 21) + `"}`, 400, `"fields":{"title"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{}
			rec := do(t, repo, &bytes.Buffer{}, "POST", tc.body, true)
			if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
			if repo.created != (NewItem{}) {
				t.Fatal("no debe llegar al repositorio")
			}
		})
	}
}

func TestCreateTooLarge(t *testing.T) {
	h := NewHandler(&fakeRepo{}, handlerLimits, slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)))
	req := httptest.NewRequest("POST", "/api/items", strings.NewReader(`{"title":"`+strings.Repeat("a", 100)+`"}`))
	req = req.WithContext(auth.WithUser(req.Context(), auth.User{Subject: "alice"}))
	rec := httptest.NewRecorder()
	req.Body = http.MaxBytesReader(rec, req.Body, 20)
	h.ServeHTTP(rec, req)
	if rec.Code != 413 || !strings.Contains(rec.Body.String(), `"request too large"`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestList(t *testing.T) {
	repo := &fakeRepo{items: []Item{{ID: 1, Title: "Zelda", CreatedBy: "alice"}}}
	rec := do(t, repo, &bytes.Buffer{}, "GET", "", true)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"items":[{"id":1`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	if repo.listLimit != 3 {
		t.Fatalf("debe pedir ITEMS_PAGE_LIMIT, pidió %d", repo.listLimit)
	}
}

func TestListEmptyIsArray(t *testing.T) {
	rec := do(t, &fakeRepo{items: []Item{}}, &bytes.Buffer{}, "GET", "", true)
	if rec.Body.String() != "{\"items\":[]}\n" {
		t.Fatalf("body = %q", rec.Body)
	}
}

func TestRepoErrorIsGeneric500(t *testing.T) {
	for _, method := range []string{"GET", "POST"} {
		t.Run(method, func(t *testing.T) {
			repo := &fakeRepo{err: errors.New("conn refused: detalle interno")}
			rec := do(t, repo, &bytes.Buffer{}, method, `{"title":"x"}`, true)
			if rec.Code != 500 || strings.Contains(rec.Body.String(), "detalle interno") {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestWithoutUserIs401(t *testing.T) {
	if rec := do(t, &fakeRepo{}, &bytes.Buffer{}, "GET", "", false); rec.Code != 401 {
		t.Fatalf("sin usuario en el contexto: %d", rec.Code)
	}
}
