package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JesusMaVe/api/internal/auth"
	"github.com/JesusMaVe/api/internal/items"
)

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func get(t *testing.T, d Deps, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	New(d).ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func TestHealthzOK(t *testing.T) {
	rec := get(t, Deps{DB: fakeDB{}, Log: discard()}, "/healthz")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestHealthzDBDown(t *testing.T) {
	rec := get(t, Deps{DB: fakeDB{err: errors.New("dial tcp: refused")}, Log: discard()}, "/healthz")
	if rec.Code != 503 || strings.Contains(rec.Body.String(), "refused") {
		t.Fatalf("se esperaba 503 genérico, got %d %s", rec.Code, rec.Body)
	}
}

func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	rec := get(t, Deps{DB: fakeDB{}, Log: discard(), MaxBodyBytes: 1024}, "/no-existe")
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("X-Request-Id") == "" {
		t.Fatalf("faltan headers: %v", rec.Header())
	}
}

type okVerifier struct{}

func (okVerifier) Verify(string) (auth.User, error) { return auth.User{Subject: "alice"}, nil }

// Una base que acepta la conexión pero no responde no debe colgar /api/items.
type hangingRepo struct{}

func (hangingRepo) List(ctx context.Context, _ int) ([]items.Item, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (hangingRepo) Create(ctx context.Context, _ items.NewItem) (items.Item, error) {
	<-ctx.Done()
	return items.Item{}, ctx.Err()
}

func (hangingRepo) Update(ctx context.Context, _ int64, _ string, _ items.Changes) (items.Item, error) {
	<-ctx.Done()
	return items.Item{}, ctx.Err()
}

func (hangingRepo) Delete(ctx context.Context, _ int64, _ string) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestItemsDoNotHangWhenDBHangs(t *testing.T) {
	h := New(Deps{DB: fakeDB{}, Log: discard(), MaxBodyBytes: 1024, Verifier: okVerifier{}, Items: hangingRepo{},
		Limits: items.Limits{TitleMax: 10, DescriptionMax: 10, PageLimit: 10}})
	req := httptest.NewRequest("GET", "/api/items", nil)
	req.Header.Set("Authorization", "Bearer x")
	rec := httptest.NewRecorder()
	start := time.Now()
	h.ServeHTTP(rec, req)
	if rec.Code != 500 || time.Since(start) > itemsTimeout+time.Second {
		t.Fatalf("got %d en %v; se esperaba 500 genérico dentro de %v", rec.Code, time.Since(start), itemsTimeout)
	}
}
