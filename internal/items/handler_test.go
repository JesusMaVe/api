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
	updated   updateCall
	deleted   deleteCall
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

func (f *fakeRepo) Update(_ context.Context, id int64, owner string, c Changes) (Item, error) {
	f.updated = updateCall{id, owner, c}
	if f.err != nil {
		return Item{}, f.err
	}
	return Item{ID: id, Title: c.Title, Description: c.Description, CreatedBy: owner, CreatedAt: time.Unix(0, 0).UTC()}, nil
}

func (f *fakeRepo) Delete(_ context.Context, id int64, owner string) error {
	f.deleted = deleteCall{id, owner}
	return f.err
}

type updateCall struct {
	id    int64
	owner string
	c     Changes
}

type deleteCall struct {
	id    int64
	owner string
}

func doAt(t *testing.T, repo *fakeRepo, logs *bytes.Buffer, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(repo, handlerLimits, slog.New(slog.NewJSONHandler(logs, nil)))
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req = req.WithContext(auth.WithUser(req.Context(), auth.User{Subject: "alice"}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestUpdate(t *testing.T) {
	repo, logs := &fakeRepo{}, &bytes.Buffer{}
	rec := doAt(t, repo, logs, "PUT", "/api/items/7", `{"title":"  Zelda TOTK ","description":" secuela "}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"title":"Zelda TOTK"`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	want := updateCall{7, "alice", Changes{Title: "Zelda TOTK", Description: "secuela"}}
	if repo.updated != want {
		t.Fatalf("el repo recibió %+v, quería %+v (dueño = sub del token)", repo.updated, want)
	}
	if !strings.Contains(logs.String(), `"user":"alice"`) || strings.Contains(logs.String(), "Zelda") {
		t.Fatalf("el log de auditoría lleva usuario e id, no el contenido: %s", logs)
	}
}

func TestDelete(t *testing.T) {
	repo := &fakeRepo{}
	rec := doAt(t, repo, &bytes.Buffer{}, "DELETE", "/api/items/7", "")
	if rec.Code != 204 || rec.Body.Len() != 0 {
		t.Fatalf("got %d %q", rec.Code, rec.Body)
	}
	if repo.deleted != (deleteCall{7, "alice"}) {
		t.Fatalf("el repo recibió %+v", repo.deleted)
	}
}

func TestUpdateRejects(t *testing.T) {
	cases := []struct {
		name, path, body string
		status           int
		want             string
	}{
		{"id no numérico", "/api/items/abc", `{"title":"x"}`, 404, `"not found"`},
		{"id cero", "/api/items/0", `{"title":"x"}`, 404, `"not found"`},
		{"campo desconocido", "/api/items/7", `{"title":"x","created_by":"bob"}`, 400, `"invalid request"`},
		{"título vacío", "/api/items/7", `{"title":"  "}`, 400, `"fields":{"title"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{}
			rec := doAt(t, repo, &bytes.Buffer{}, "PUT", tc.path, tc.body)
			if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("got %d %s", rec.Code, rec.Body)
			}
			if repo.updated != (updateCall{}) {
				t.Fatal("no debe llegar al repositorio")
			}
		})
	}
}

func TestUpdateDeleteRepoErrors(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		want   string
	}{
		{"no existe", ErrNotFound, 404, `"not found"`},
		{"de otro usuario", ErrForbidden, 403, `"forbidden"`},
		{"falla la base", errors.New("conn refused: detalle interno"), 500, `"internal error"`},
	}
	for _, tc := range cases {
		for _, method := range []string{"PUT", "DELETE"} {
			t.Run(tc.name+"/"+method, func(t *testing.T) {
				rec := doAt(t, &fakeRepo{err: tc.err}, &bytes.Buffer{}, method, "/api/items/7", `{"title":"x"}`)
				if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.want) || strings.Contains(rec.Body.String(), "detalle") {
					t.Fatalf("got %d %s", rec.Code, rec.Body)
				}
			})
		}
	}
}

func TestUpdateDeleteWithoutUserIs401(t *testing.T) {
	h := NewHandler(&fakeRepo{}, handlerLimits, slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)))
	for _, method := range []string{"PUT", "DELETE"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/api/items/7", strings.NewReader(`{"title":"x"}`)))
		if rec.Code != 401 {
			t.Fatalf("%s sin usuario = %d", method, rec.Code)
		}
	}
}
